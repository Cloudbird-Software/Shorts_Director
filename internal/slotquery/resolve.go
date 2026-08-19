package slotquery

import (
	"fmt"
	"sort"
	"time"

	"github.com/Cloudbird-Software/Shorts_Director/internal/entity"
)

// Resolution 是一次 slot 取材的结果：命中的降级级 + 排序后的候选。
type Resolution struct {
	SlotID            string
	Level             int    // 0 = 原始查询；>0 = 降级
	DegradeNote       string // 降级说明（落盘 constraints_report.fallbacks_used）
	IsTerminalGraphic bool   // true = 渲染 Remotion 图形卡（不依赖 shot 池）
	Shots             []entity.Shot
}

// Resolve 走降级链逐级取材。规则：
//  1. 候选池先过实体层硬门（IsConsumable：状态/合规/ttl；EligibleForVertical：IV-SH-1）
//  2. level 0 用 must/forbid；之后逐级用 fallback.must（继承顶层 forbid 与 should）
//  3. 每级叠加消耗策略（cooldown 内或超 max_uses 的 shot 出池）
//  4. 首个非空级胜出；排序 = Score↓ → use_count↑（prefer_least_used，默认开）→ ID↑
//  5. 终端图形级永远可解（IV-SQ-1）——池空也返回 IsTerminalGraphic，永不无解
func Resolve(q Query, pool []entity.Shot, now time.Time) (*Resolution, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}
	usable := make([]entity.Shot, 0, len(pool))
	for _, s := range pool {
		if s.IsConsumable(now) && s.EligibleForVertical() {
			usable = append(usable, s)
		}
	}
	consumable := applyConsumption(q.ConsumptionPolicy, usable, now)

	if res := matchLevel(q, 0, "", q.Must, consumable); res != nil {
		return res, nil
	}
	for _, f := range q.FallbackChain {
		if f.IsTerminalGraphic {
			// 终端兜底渲染 Remotion 图形卡，不依赖 shot 池——Shots 留空。
			return &Resolution{
				SlotID: q.SlotID, Level: f.Level, DegradeNote: f.DegradeNote,
				IsTerminalGraphic: true,
			}, nil
		}
		if res := matchLevel(q, f.Level, f.DegradeNote, f.Must, consumable); res != nil {
			return res, nil
		}
	}
	// Validate 已保证末端可解；防御性兜底（不改变契约语义）。
	return nil, fmt.Errorf("slotquery: slot=%s 无解——fallback_chain 未以终端兜底收尾", q.SlotID)
}

// matchLevel 在一级 must 上求候选并按 q 排序；空结果返回 nil。
func matchLevel(q Query, level int, note string, must []Predicate, pool []entity.Shot) *Resolution {
	matched := matchAll(must, q.Forbid, pool)
	if len(matched) == 0 {
		return nil
	}
	return &Resolution{SlotID: q.SlotID, Level: level, DegradeNote: note, Shots: order(matched, q)}
}

func matchAll(must, forbid []Predicate, pool []entity.Shot) []entity.Shot {
	out := make([]entity.Shot, 0, len(pool))
	for _, s := range pool {
		if ok, err := Match(Query{Must: must, Forbid: forbid}, &s); err == nil && ok {
			out = append(out, s)
		}
	}
	return out
}

// order 确定性全序：Score↓ → use_count↑（prefer_least_used，默认开）→ ID↑。
// 同一池内 ID 唯一 ⇒ 序稳定可复现（A2：确定性是硬约束）。
func order(shots []entity.Shot, q Query) []entity.Shot {
	preferLeast := true
	if q.ConsumptionPolicy.PreferLeastUsed != nil {
		preferLeast = *q.ConsumptionPolicy.PreferLeastUsed
	}
	sort.SliceStable(shots, func(i, j int) bool {
		si, sj := &shots[i], &shots[j]
		qi, qj := Score(q, si), Score(q, sj)
		if qi != qj {
			return qi > qj
		}
		if preferLeast && si.Lifecycle.UseCount != sj.Lifecycle.UseCount {
			return si.Lifecycle.UseCount < sj.Lifecycle.UseCount
		}
		return si.ID < sj.ID
	})
	return shots
}

// applyConsumption 过滤消耗策略：cooldown_days 内未冷却或 30 天窗口超限的 shot 出池。
// use_count 是 30 天窗口计数的物化近似（权威值在 AssetQueryEngine 查询层）。
func applyConsumption(p ConsumptionPolicy, pool []entity.Shot, now time.Time) []entity.Shot {
	out := make([]entity.Shot, 0, len(pool))
	for _, s := range pool {
		if s.Lifecycle.UseCount >= p.MaxUsesPer30d {
			continue
		}
		if p.CooldownDays > 0 && s.Lifecycle.LastUsedAt != nil {
			if last, err := time.Parse("2006-01-02", *s.Lifecycle.LastUsedAt); err == nil {
				if now.Sub(last) < time.Duration(p.CooldownDays)*24*time.Hour {
					continue
				}
			}
		}
		out = append(out, s)
	}
	return out
}
