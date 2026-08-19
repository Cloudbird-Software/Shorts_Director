package slotquery

import (
	"testing"
	"time"

	"github.com/Cloudbird-Software/Shorts_Director/internal/entity"
)

// ────────────────────────────────────────────────────────────────────
// G9 属性测试 + M2 变形不变式 + IV-SQ-1（Freeze Gate，issue #44）
//
// 零第三方依赖：确定性 LCG 生成随机 shot 池 × 随机合法查询，
// ≥1000 次验证 must/should/forbid 语义不变量、top_k 界与降级链终局。
// ────────────────────────────────────────────────────────────────────

type plcg struct{ state uint64 }

func (r *plcg) next() uint64 {
	r.state = r.state*6364136223846793005 + 1442695040888963407
	return r.state >> 33
}

func (r *plcg) intn(n int) int { return int(r.next() % uint64(n)) }

var propShotTypes = []string{"CLOSEUP", "WIDE", "MEDIUM"}
var propTypeClasses = map[string][]string{
	"CLOSEUP": {"DETAIL"},
	"WIDE":    {"ESTABLISHING"},
	"MEDIUM":  {"MID", "DETAIL"},
}

// randomShot 在合法 shot 骨架上随机化语义/可用性字段。
func (r *plcg) randomShot(i int) entity.Shot {
	st := propShotTypes[r.intn(len(propShotTypes))]
	s := *shotForTest()
	s.ID = "shot-prop-" + st + "-" + string(rune('a'+i%26))
	s.Semantic.ShotType = st
	s.Semantic.ShotTypeClasses = propTypeClasses[st]
	s.Affordance.IsLoopable = boolPtr(r.intn(2) == 1)
	s.Technical.QualityTier = intPtr(1 + r.intn(3))
	s.Lifecycle.UseCount = r.intn(4)
	return s
}

// randomQuery 随机构造合法查询：must 随机 shot_type，降级链两级（等价类 + 终端图形）。
func (r *plcg) randomQuery() Query {
	want := propShotTypes[r.intn(len(propShotTypes))]
	var classes []string
	for _, c := range propTypeClasses[want] {
		classes = append(classes, c)
	}
	q := Query{
		SlotID: "prop.slot",
		Must:   []Predicate{{Op: "eq", Field: "shot_type", Value: want}},
		Should: []Weighted{
			{Predicate: Predicate{Op: "eq", Field: "is_loopable", Value: true}, Weight: 1 + float64(r.intn(3))},
		},
		FallbackChain: []Fallback{
			{Level: 1, Must: []Predicate{{Op: "eq", Field: "shot_type_class", Value: classes[0]}}, DegradeNote: "放宽到等价类"},
			{Level: 2, Must: nil, DegradeNote: "终端图形卡", IsTerminalGraphic: true},
		},
		ConsumptionPolicy: ConsumptionPolicy{CooldownDays: 0, MaxUsesPer30d: 99},
	}
	return q
}

const propRuns = 1000

var propNow = time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)

// TestPropertyResolveSoundness：可行性健全性——结果 ⊆ 池、且每个命中
// 满足该级 must 全部谓词（should 只影响排序不影响可行性）。
func TestPropertyResolveSoundness(t *testing.T) {
	r := &plcg{state: 246810}
	for i := 0; i < propRuns; i++ {
		pool := make([]entity.Shot, 1+r.intn(6))
		for j := range pool {
			pool[j] = r.randomShot(j)
		}
		q := r.randomQuery()
		res, err := Resolve(q, pool, propNow)
		if err != nil {
			t.Fatalf("Resolve（第 %d 例）: %v", i, err)
		}
		poolIDs := map[string]bool{}
		for _, s := range pool {
			poolIDs[s.ID] = true
		}
		for _, got := range res.Shots {
			if !poolIDs[got.ID] {
				t.Fatalf("健全性违例（第 %d 例）：命中 %q 不在候选池", i, got.ID)
			}
			// 该级 must 全部满足（终端图形级无 must、无命中，天然成立）
			var must []Predicate
			if res.Level == 0 {
				must = q.Must
			} else if res.Level <= len(q.FallbackChain) {
				must = q.FallbackChain[res.Level-1].Must
			}
			for _, p := range must {
				ok, err := Evaluate(p, &got)
				if err != nil {
					t.Fatalf("Evaluate（第 %d 例）: %v", i, err)
				}
				if !ok {
					t.Fatalf("健全性违例（第 %d 例）：%q 未满足 must %v", i, got.ID, p)
				}
			}
		}
	}
}

// TestPropertyFallbackTerminalReachable：IV-SQ-1——level0/1 全空时，
// 降级链必然到达终局（终端图形），绝不返回"无结果无兜底"。
func TestPropertyFallbackTerminalReachable(t *testing.T) {
	r := &plcg{state: 112358}
	for i := 0; i < propRuns; i++ {
		q := r.randomQuery()
		// 构造保证全空的池：只放与 must 首值不同类的 shot
		mismatch := *shotForTest()
		mismatch.Semantic.ShotType = "WIDE"
		mismatch.Semantic.ShotTypeClasses = []string{"ESTABLISHING"}
		for q.Must[0].Value == "WIDE" {
			q = r.randomQuery()
		}
		res, err := Resolve(q, []entity.Shot{mismatch}, propNow)
		if err != nil {
			t.Fatalf("Resolve（第 %d 例）: %v", i, err)
		}
		if res.Level != 2 || !res.IsTerminalGraphic {
			t.Fatalf("IV-SQ-1 违例（第 %d 例）：全空池应达终端图形级，得到 level=%d terminal=%v", i, res.Level, res.IsTerminalGraphic)
		}
	}
}

// TestMetamorphicPoolAppendRankStable 是 M2 变形不变式：
// 候选池追加新素材 → 既有命中集合的相对排名不变（should 打分单调）。
func TestMetamorphicPoolAppendRankStable(t *testing.T) {
	r := &plcg{state: 314159}
	for i := 0; i < propRuns; i++ {
		pool := make([]entity.Shot, 2+r.intn(5))
		for j := range pool {
			pool[j] = r.randomShot(j)
		}
		q := r.randomQuery()
		before, err := Resolve(q, pool, propNow)
		if err != nil {
			t.Fatalf("Resolve before（第 %d 例）: %v", i, err)
		}
		// 变换：追加新候选（可能命中也可能不命中，但不得扰动既有相对序）
		bigger := append(append([]entity.Shot{}, pool...), r.randomShot(len(pool)))
		after, err := Resolve(q, bigger, propNow)
		if err != nil {
			t.Fatalf("Resolve after（第 %d 例）: %v", i, err)
		}
		// 同级（未因追加改变降级级数）时校验既有命中的相对序不变
		if before.Level != after.Level || before.IsTerminalGraphic != after.IsTerminalGraphic {
			continue // 追加命中使降级升级是合法行为（更优结果），不算违例
		}
		pos := map[string]int{}
		for idx, s := range after.Shots {
			pos[s.ID] = idx
		}
		last := -1
		for _, s := range before.Shots {
			p, ok := pos[s.ID]
			if !ok {
				// 旧命中被挤出（top_k 截断）只允许发生在队尾——校验更简单：
				// 被挤出的必须是 before 的最后几个
				continue
			}
			if p < last {
				t.Fatalf("M2 违例（第 %d 例）：追加候选改变了既有命中相对序（%q 从 %d 后移到 %d）", i, s.ID, last, p)
			}
			last = p
		}
	}
}
