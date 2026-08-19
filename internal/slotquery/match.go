package slotquery

import (
	"fmt"

	"github.com/Cloudbird-Software/Shorts_Director/internal/entity"
)

// Match 报告 shot 是否满足硬条件：must 全真 ∧ forbid 全假。
func Match(q Query, s *entity.Shot) (bool, error) {
	for i, p := range q.Must {
		ok, err := Evaluate(p, s)
		if err != nil {
			return false, fmt.Errorf("slotquery: must[%d]: %w", i, err)
		}
		if !ok {
			return false, nil
		}
	}
	for i, p := range q.Forbid {
		ok, err := Evaluate(p, s)
		if err != nil {
			return false, fmt.Errorf("slotquery: forbid[%d]: %w", i, err)
		}
		if ok {
			return false, nil
		}
	}
	return true, nil
}

// Score 计算软条件总分：命中 should 谓词累加权重，semantic 无注入
// 排序器时按 0 分跳过（不报错——软条件允许降级，可行性不受影响）。
func Score(q Query, s *entity.Shot) float64 {
	total := 0.0
	for _, w := range q.Should {
		ok, err := Evaluate(w.Predicate, s)
		if err != nil || !ok {
			continue
		}
		total += w.Weight
	}
	return total
}
