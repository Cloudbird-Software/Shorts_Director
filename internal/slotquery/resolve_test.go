package slotquery

import (
	"testing"
	"time"

	"github.com/Cloudbird-Software/Shorts_Director/internal/entity"
)

// baseQuery 构造合法最小查询：level0 要 CLOSEUP，两级降级，末端终端图形兜底。
func baseQuery() Query {
	return Query{
		SlotID: "hook.detail",
		Must:   []Predicate{{Op: "eq", Field: "shot_type", Value: "CLOSEUP"}},
		Should: []Weighted{{Predicate: Predicate{Op: "eq", Field: "is_loopable", Value: true}, Weight: 2}},
		FallbackChain: []Fallback{
			{Level: 1, Must: []Predicate{{Op: "eq", Field: "shot_type_class", Value: "DETAIL"}}, DegradeNote: "放宽到 DETAIL 等价类"},
			{Level: 2, Must: nil, DegradeNote: "终端图形卡", IsTerminalGraphic: true},
		},
		ConsumptionPolicy: ConsumptionPolicy{CooldownDays: 0, MaxUsesPer30d: 5},
	}
}

// clone 复制 shot 并改 ID/use_count，构造候选池。
func cloneShot(base *entity.Shot, id string, useCount int) entity.Shot {
	s := *base
	s.ID = id
	s.Lifecycle.UseCount = useCount
	return s
}

func TestResolveLevel0DirectHit(t *testing.T) {
	q := baseQuery()
	now := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	pool := []entity.Shot{cloneShot(shotForTest(), "shot-1", 0)}
	res, err := Resolve(q, pool, now)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Level != 0 || res.IsTerminalGraphic || len(res.Shots) != 1 || res.Shots[0].ID != "shot-1" {
		t.Errorf("level0 应直接命中: %+v", res)
	}
	if res.DegradeNote != "" {
		t.Errorf("level0 命中不应有降级说明，得到 %q", res.DegradeNote)
	}
}

func TestResolveDegradesToFallback(t *testing.T) {
	q := baseQuery()
	// 池里只有 WIDE（非 CLOSEUP、非 DETAIL 类）——level0/1 都空，应落到终端图形。
	wide := shotForTest()
	wide.Semantic.ShotType = "WIDE"
	wide.Semantic.ShotTypeClasses = []string{"ESTABLISHING"}
	now := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	res, err := Resolve(q, []entity.Shot{*wide}, now)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !res.IsTerminalGraphic || res.Level != 2 || len(res.Shots) != 0 {
		t.Errorf("应落到终端图形级: %+v", res)
	}
	if res.DegradeNote != "终端图形卡" {
		t.Errorf("DegradeNote = %q", res.DegradeNote)
	}
}

func TestResolveLevel1Fallback(t *testing.T) {
	q := baseQuery()
	// CLOSEUP⇒DETAIL 等价类的 shot：level0 的 shot_type=CLOSEUP 失败（值被改），
	// 但 shot_type_class=DETAIL 在 level1 命中。
	detail := shotForTest()
	detail.Semantic.ShotType = "INSERT"
	now := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	res, err := Resolve(q, []entity.Shot{*detail}, now)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Level != 1 || res.IsTerminalGraphic {
		t.Errorf("应在 level1 命中: level=%d terminal=%v", res.Level, res.IsTerminalGraphic)
	}
	if res.DegradeNote != "放宽到 DETAIL 等价类" {
		t.Errorf("DegradeNote = %q", res.DegradeNote)
	}
}

func TestResolveEmptyPoolTerminal(t *testing.T) {
	q := baseQuery()
	now := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	res, err := Resolve(q, nil, now)
	if err != nil {
		t.Fatalf("空池也应可解（IV-SQ-1）: %v", err)
	}
	if !res.IsTerminalGraphic {
		t.Error("空池必须落到终端图形级")
	}
}

func TestResolveHardGates(t *testing.T) {
	now := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	mk := func(mutate func(*entity.Shot)) entity.Shot {
		s := shotForTest()
		mutate(s)
		return *s
	}
	expired := mk(func(s *entity.Shot) { d := "2026-08-01"; s.Lifecycle.TTLAt = &d })
	risky := mk(func(s *entity.Shot) { s.Compliance.RiskFlags = []string{"THIRD_PARTY_FACE"} })
	notAvail := mk(func(s *entity.Shot) { s.State = entity.ShotTagged })
	noCrop := mk(func(s *entity.Shot) { s.Affordance.SafeCrop9x16 = &entity.SafeCrop{OK: false, Method: strPtr("NONE")} })

	for name, s := range map[string]entity.Shot{
		"ttl 过期": expired, "risk_flags 非空": risky, "非 AVAILABLE": notAvail, "裁切不可行": noCrop,
	} {
		res, err := Resolve(baseQuery(), []entity.Shot{s}, now)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !res.IsTerminalGraphic {
			t.Errorf("%s：硬门未拦截，误入 shot 级（level=%d）", name, res.Level)
		}
	}
}

func TestResolveConsumptionPolicy(t *testing.T) {
	now := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	q := baseQuery()
	q.ConsumptionPolicy.MaxUsesPer30d = 1

	overused := cloneShot(shotForTest(), "shot-over", 1) // use_count ≥ max ⇒ 出池
	res, err := Resolve(q, []entity.Shot{overused}, now)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !res.IsTerminalGraphic {
		t.Error("超 max_uses 的 shot 应出池并降级到终端图形")
	}

	// cooldown：last_used_at 3 天前，cooldown 7 天 ⇒ 出池。
	q.ConsumptionPolicy.MaxUsesPer30d = 5
	q.ConsumptionPolicy.CooldownDays = 7
	cooling := cloneShot(shotForTest(), "shot-cool", 0)
	last := "2026-08-17"
	cooling.Lifecycle.LastUsedAt = &last
	res, err = Resolve(q, []entity.Shot{cooling}, now)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !res.IsTerminalGraphic {
		t.Error("cooldown 内的 shot 应出池")
	}
	// last_used_at 8 天前 ⇒ 冷却完成，可入选。
	ok := cloneShot(shotForTest(), "shot-ok", 0)
	lastOK := "2026-08-12"
	ok.Lifecycle.LastUsedAt = &lastOK
	res, err = Resolve(q, []entity.Shot{ok}, now)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Level != 0 || res.Shots[0].ID != "shot-ok" {
		t.Errorf("冷却完成的 shot 应在 level0 命中: %+v", res)
	}
}

func TestResolveDeterministicOrder(t *testing.T) {
	now := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	q := baseQuery()

	// 同分（都 loopable）⇒ use_count 升序；再同 ⇒ ID 升序。
	lo := cloneShot(shotForTest(), "shot-b", 0)
	hi := cloneShot(shotForTest(), "shot-a", 3)
	same := cloneShot(shotForTest(), "shot-c", 0)

	for round := 0; round < 3; round++ { // 多轮洗入验证稳定
		res, err := Resolve(q, []entity.Shot{hi, same, lo}, now)
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		got := []string{res.Shots[0].ID, res.Shots[1].ID, res.Shots[2].ID}
		want := []string{"shot-b", "shot-c", "shot-a"}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("round %d 排序 = %v, want %v", round, got, want)
			}
		}
	}

	// should 加分优先：clean_in 加分 5 > loopable 加分 2 ⇒ 排序压过 use_count 劣势。
	q.Should = append(q.Should, Weighted{
		Predicate: Predicate{Op: "eq", Field: "clean_in", Value: true}, Weight: 5,
	})
	scored := cloneShot(shotForTest(), "shot-scored", 4) // use_count 劣势但加分 5
	scored.Affordance.CleanIn = boolPtr(true)
	scored.Affordance.IsLoopable = boolPtr(false)
	res, err := Resolve(q, []entity.Shot{scored, lo}, now)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Shots[0].ID != "shot-scored" {
		t.Errorf("Score 应优先于 use_count: %v", res.Shots[0].ID)
	}
}

func TestResolveInvalidQueryRejected(t *testing.T) {
	q := baseQuery()
	q.ConsumptionPolicy.MaxUsesPer30d = 0
	if _, err := Resolve(q, nil, time.Now()); err == nil {
		t.Error("非法 consumption_policy 应被拒绝")
	}
	q = baseQuery()
	q.FallbackChain = q.FallbackChain[:1] // 去掉终端兜底
	q.FallbackChain[0].Must = []Predicate{{Op: "eq", Field: "shot_type", Value: "CLOSEUP"}}
	if _, err := Resolve(q, nil, time.Now()); err == nil {
		t.Error("IV-SQ-1 违反应在 Resolve 入口被拒绝")
	}
}

func strPtr(s string) *string { return &s }
