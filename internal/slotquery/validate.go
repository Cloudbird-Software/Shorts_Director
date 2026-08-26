package slotquery

import (
	"errors"
	"fmt"
	"strings"

	vocabgen "github.com/Cloudbird-Software/Shorts_Director/codegen/go/vocab"
	"github.com/Cloudbird-Software/Shorts_Director/internal/vocab"
)

// ErrSemanticNotRankable 表示 semantic 谓词需要注入向量排序器（仅 should）。
var ErrSemanticNotRankable = errors.New("slotquery: semantic 谓词需要 SemanticRanker，不能用于硬匹配")

// vocabContains 校验 id 是否落在词表（或带点前缀的任意分表）内。
func vocabContains(nameOrPrefix, id string) error {
	if !strings.HasSuffix(nameOrPrefix, ".") {
		if !vocab.IsVocabID(nameOrPrefix, id) {
			return fmt.Errorf("不在词表 %s", nameOrPrefix)
		}
		return nil
	}
	for _, table := range vocabgen.VocabFiles {
		if strings.HasPrefix(table, nameOrPrefix) && vocab.IsVocabID(table, id) {
			return nil
		}
	}
	return fmt.Errorf("不在任何 %s* 分表", nameOrPrefix)
}

// ValidateQuery 校验完整查询（must/forbid 禁 semantic + IV-SQ-1 降级链可解性）。
func (q Query) Validate() error {
	if q.SlotID == "" {
		return errors.New("slotquery: slot_id 必填")
	}
	hard := append(append([]Predicate{}, q.Must...), q.Forbid...)
	for i, p := range hard {
		if p.Op == "semantic" {
			return fmt.Errorf("slotquery: semantic 仅允许出现在 should（must[%d]/forbid）", i)
		}
		if err := p.Validate(); err != nil {
			return fmt.Errorf("slotquery: must/forbid[%d]: %w", i, err)
		}
	}
	for i, w := range q.Should {
		if err := w.Predicate.Validate(); err != nil {
			return fmt.Errorf("slotquery: should[%d]: %w", i, err)
		}
	}
	if len(q.FallbackChain) < 1 {
		return errors.New("slotquery: fallback_chain 至少 1 级（IV-SQ-1）")
	}
	// IV-SQ-1：链末端必须命中永远可用的兜底——is_terminal_graphic 或无条件（空 must）。
	last := q.FallbackChain[len(q.FallbackChain)-1]
	if !last.IsTerminalGraphic && len(last.Must) > 0 {
		return fmt.Errorf(
			"slotquery: IV-SQ-1 违反——fallback_chain 末端必须 is_terminal_graphic 或空 must（slot=%s）",
			q.SlotID)
	}
	for i, f := range q.FallbackChain {
		for j, p := range f.Must {
			if p.Op == "semantic" {
				return fmt.Errorf("slotquery: semantic 不得出现在 fallback[%d].must[%d]", i, j)
			}
			if err := p.Validate(); err != nil {
				return fmt.Errorf("slotquery: fallback[%d].must[%d]: %w", i, j, err)
			}
		}
	}
	if q.ConsumptionPolicy.CooldownDays < 0 || q.ConsumptionPolicy.MaxUsesPer30d < 1 {
		return errors.New("slotquery: consumption_policy 边界非法（cooldown≥0, max_uses_per_30d≥1）")
	}
	return nil
}
