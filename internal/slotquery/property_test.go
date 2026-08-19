package slotquery

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/Cloudbird-Software/Shorts_Director/internal/entity"
)

// Freeze Gate G9（属性 ×1000）+ G10 变形不变式 M2。
// 生成器固定种子；字段域取 Shot 直供子集（quality_tier/is_loopable/scene/
// duration），避开编排层注入字段（source_kind 等）。

var genScenes = []string{"KITCHEN", "COUNTER", "DINING_AREA"}

func genShot(r *rand.Rand, i int) entity.Shot {
	qt := 1 + r.Intn(4)
	loop := r.Intn(2) == 1
	in := r.Intn(200)
	out := in + 1 + r.Intn(500)
	s := entity.Shot{
		ID:       fmt.Sprintf("018f6c01-aaaa-7aaa-8aaa-%012d", i),
		AssetID:  "018f6b2e-9c4a-7b3e-a1d2-3f4e5d6c7b8a",
		TenantID: "018f6b10-0000-7000-8000-000000000001",
		State:    entity.ShotAvailable,
		Identity: entity.ShotIdentity{InFrame: in, OutFrame: out, FPS: 25},
		Semantic: entity.ShotSemantic{Scene: genScenes[r.Intn(len(genScenes))]},
		Affordance: entity.ShotAffordance{
			IsLoopable: &loop,
		},
		Technical: entity.ShotTechnical{QualityTier: &qt},
		TagProvenance: entity.Provenance{
			GeneratedBy:   entity.GeneratedByDeterministic,
			ModelID:       "gen@1",
			PromptVersion: "none",
			InputDigest:   "sha256:" + fmt.Sprintf("%064d", i),
			CreatedAt:     "2026-08-01T00:00:00Z",
		},
	}
	return s
}

func genPredicate(r *rand.Rand) Predicate {
	switch r.Intn(4) {
	case 0:
		return Predicate{Op: "gte", Field: "quality_tier", Value: float64(1 + r.Intn(4))}
	case 1:
		return Predicate{Op: "eq", Field: "is_loopable", Value: r.Intn(2) == 1}
	case 2:
		return Predicate{Op: "eq", Field: "scene", Value: genScenes[r.Intn(len(genScenes))]}
	default:
		return Predicate{Op: "lte", Field: "duration", Value: float64(100 + r.Intn(900))}
	}
}

func genQuery(r *rand.Rand) Query {
	must := make([]Predicate, r.Intn(3))
	for i := range must {
		must[i] = genPredicate(r)
	}
	forbid := []Predicate{}
	if r.Intn(3) == 0 {
		forbid = append(forbid, Predicate{
			Op: "eq", Field: "scene", Value: genScenes[r.Intn(len(genScenes))]})
	}
	return Query{
		SlotID: "prop.slot",
		Must:   must,
		Forbid: forbid,
		FallbackChain: []Fallback{{
			Level: 1, Must: []Predicate{},
			DegradeNote: "终端图形兜底", IsTerminalGraphic: true,
		}},
		ConsumptionPolicy: ConsumptionPolicy{CooldownDays: 0, MaxUsesPer30d: 30},
	}
}

// TestG9ResolveInvariants（属性 ×1000）：随机 query × 随机 shot 池：
//   - 合法 query 永不报错（终端兜底保证可解，IV-SQ-1）
//   - 非终端解的每个 shot 满足全部 must 且不触 forbid
//   - 同输入两次 Resolve 结果完全一致（确定性）
func TestG9ResolveInvariants(t *testing.T) {
	r := rand.New(rand.NewSource(44)) //nolint:gosec
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	for run := 0; run < 1000; run++ {
		q := genQuery(r)
		pool := make([]entity.Shot, r.Intn(12))
		for i := range pool {
			pool[i] = genShot(r, i)
		}
		res, err := Resolve(q, pool, now)
		if err != nil {
			t.Fatalf("run %d: 合法 query 报错 %v", run, err)
		}
		if !res.IsTerminalGraphic {
			if len(res.Shots) == 0 {
				t.Fatalf("run %d: 非终端解为空", run)
			}
			for _, s := range res.Shots {
				for _, p := range q.Must {
					ok, err := Evaluate(p, &s)
					if err != nil || !ok {
						t.Fatalf("run %d: 返回 shot 违反 must %v (err=%v)", run, p, err)
					}
				}
				for _, p := range q.Forbid {
					ok, err := Evaluate(p, &s)
					if err != nil || ok {
						t.Fatalf("run %d: 返回 shot 触发 forbid %v", run, p)
					}
				}
			}
		}
		again, err := Resolve(q, pool, now)
		if err != nil || !reflect.DeepEqual(res, again) {
			t.Fatalf("run %d: Resolve 非确定性", run)
		}
	}
}

// TestM2PoolAppendRankingStable（变形不变式 2，×1000）：
// shot 池追加新候选 → 既有命中集合的相对排名不变（should 打分单调）。
// order() 的全序：score ↓ → use_count ↑ → ID ↑——追加不改变既有元素间序。
func TestM2PoolAppendRankingStable(t *testing.T) {
	r := rand.New(rand.NewSource(45)) //nolint:gosec
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	for run := 0; run < 1000; run++ {
		q := genQuery(r)
		pool := make([]entity.Shot, 1+r.Intn(10))
		for i := range pool {
			pool[i] = genShot(r, i)
		}
		res1, err := Resolve(q, pool, now)
		if err != nil {
			t.Fatal(err)
		}
		if res1.IsTerminalGraphic {
			continue // 无既有命中可比
		}
		// 追加新候选（新 ID 空间，不与既有冲突）
		extra := make([]entity.Shot, 1+r.Intn(6))
		for i := range extra {
			extra[i] = genShot(r, 1000+i)
		}
		res2, err := Resolve(q, append(append([]entity.Shot{}, pool...), extra...), now)
		if err != nil {
			t.Fatal(err)
		}
		if res2.IsTerminalGraphic {
			t.Fatalf("run %d: 追加候选后反而落终端兜底", run)
		}
		// res2 的命中序列限制到 res1 的成员，必须与 res1 完全同序（子序列不变）
		old := res1.Shots
		seen := map[string]bool{}
		for _, s := range old {
			seen[s.ID] = true
		}
		restricted := make([]entity.Shot, 0, len(old))
		for _, s := range res2.Shots {
			if seen[s.ID] {
				restricted = append(restricted, s)
			}
		}
		if len(restricted) != len(old) {
			t.Fatalf("run %d: 既有命中丢失 %d → %d", run, len(old), len(restricted))
		}
		for i := range old {
			if old[i].ID != restricted[i].ID {
				t.Fatalf("run %d: 既有命中相对排名改变 pos %d: %s ≠ %s",
					run, i, old[i].ID, restricted[i].ID)
			}
		}
	}
}
