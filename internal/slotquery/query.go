// Package slotquery 实现 ShotSlotQuery 谓词 AST 的内存求值器：
// L3 范式层引用 L2 物料层的唯一形式（只允许等价类谓词，禁止具体实例）。
// SQL/pgvector 编译路径属 AssetQueryEngine（S3），本包是语义真源——
// 编译器输出必须与本包求值结果一致。
package slotquery

import (
	"fmt"
)

// Predicate 是受限谓词 AST（schema/entities/shot_slot_query.json $defs/Predicate）。
// op ∈ eq/in/neq/nin/gte/lte/gt/lt/between/semantic/and/or/not；
// semantic 仅允许出现在 should（IV，由 Validate 强制）。
type Predicate struct {
	Op       string      `json:"op"`
	Field    string      `json:"field,omitempty"`
	Value    any         `json:"value,omitempty"`
	Range    []float64   `json:"range,omitempty"`
	Query    string      `json:"query,omitempty"`
	TopK     int         `json:"top_k,omitempty"`
	Operands []Predicate `json:"operands,omitempty"`
}

// Weighted 是带权软条件（仅排序打分，不影响可行性）。
type Weighted struct {
	Predicate Predicate `json:"predicate"`
	Weight    float64   `json:"weight"` // [-5,5]
}

// ConsumptionPolicy 是素材消耗策略（多样性约束的运行期形态）。
type ConsumptionPolicy struct {
	CooldownDays    int   `json:"cooldown_days"`               // ≥0
	MaxUsesPer30d   int   `json:"max_uses_per_30d"`            // ≥1
	PreferLeastUsed *bool `json:"prefer_least_used,omitempty"` // 默认 true
}

// Fallback 是降级链一级：level 0 为原始查询，逐级放宽。
type Fallback struct {
	Level             int         `json:"level"`
	Must              []Predicate `json:"must"`
	DegradeNote       string      `json:"degrade_note"`
	IsTerminalGraphic bool        `json:"is_terminal_graphic,omitempty"`
}

// Query 是一个 slot 的完整取材查询。
type Query struct {
	SlotID            string            `json:"slot_id"`
	Must              []Predicate       `json:"must"`
	Should            []Weighted        `json:"should,omitempty"`
	Forbid            []Predicate       `json:"forbid,omitempty"`
	FallbackChain     []Fallback        `json:"fallback_chain"`
	ConsumptionPolicy ConsumptionPolicy `json:"consumption_policy"`
}

// fieldWhitelist 是 IV-SQ-2 的字段白名单（与 schema $defs/Field 一致）。
var fieldWhitelist = map[string]bool{
	"shot_type": true, "shot_type_class": true,
	"camera_motion.type": true, "camera_motion.dir": true,
	"scene": true, "subject": true, "action": true, "mood": true,
	"is_loopable": true, "clean_in": true, "clean_out": true,
	"has_speech": true, "has_lipsync": true,
	"negative_space": true, "safe_crop_9x16.ok": true,
	"motion_energy": true, "duration": true, "sharpness_tier": true,
	"quality_tier": true, "season": true, "ttl_at": true,
	"linked_sku": true, "use_count": true, "last_used_at": true,
	"source_kind": true,
}

// vocabFields 是取值必须落在受控词表内的字段 → 词表名（分表用前缀匹配）。
var vocabFields = map[string]string{
	"shot_type":          "shot_type",
	"camera_motion.type": "camera_motion",
	"scene":              "scene.",
	"subject":            "subject.",
	"action":             "action",
	"season":             "season",
}

// scalarOps / numericOps / logicalOps 是受控操作符集。
var (
	scalarOps  = map[string]bool{"eq": true, "neq": true, "in": true, "nin": true}
	numericOps = map[string]bool{"gte": true, "lte": true, "gt": true, "lt": true}
)

// Validate 校验谓词自身：字段白名单、op 与 value 形态、词表受控（IV-SQ-2）。
func (p Predicate) Validate() error {
	switch p.Op {
	case "and", "or", "not":
		if len(p.Operands) < 1 {
			return fmt.Errorf("slotquery: 逻辑算子 %s 需要 ≥1 operands", p.Op)
		}
		for i := range p.Operands {
			if err := p.Operands[i].Validate(); err != nil {
				return fmt.Errorf("slotquery: operands[%d]: %w", i, err)
			}
		}
		return nil
	case "semantic":
		if p.Query == "" || p.TopK < 1 || p.TopK > 50 {
			return fmt.Errorf("slotquery: semantic 需要 query 与 top_k∈[1,50]")
		}
		return nil
	case "between":
		if !fieldWhitelist[p.Field] {
			return fmt.Errorf("slotquery: IV-SQ-2 字段 %q 不在白名单", p.Field)
		}
		if len(p.Range) != 2 || p.Range[0] > p.Range[1] {
			return fmt.Errorf("slotquery: between 需要 range=[min,max] 且 min≤max")
		}
		return p.validateVocab()
	case "eq", "neq", "in", "nin", "gte", "lte", "gt", "lt":
		if !fieldWhitelist[p.Field] {
			return fmt.Errorf("slotquery: IV-SQ-2 字段 %q 不在白名单", p.Field)
		}
		if p.Value == nil {
			return fmt.Errorf("slotquery: op %s 需要 value", p.Op)
		}
		return p.validateVocab()
	default:
		return fmt.Errorf("slotquery: 非受控操作符 %q", p.Op)
	}
}

// validateVocab 校验词表绑定字段的取值受控（分表字段前缀匹配任意分表）。
func (p Predicate) validateVocab() error {
	prefix, bound := vocabFields[p.Field]
	if !bound {
		return nil
	}
	values := p.scalarValues()
	for _, v := range values {
		id, ok := v.(string)
		if !ok {
			continue
		}
		if err := vocabContains(prefix, id); err != nil {
			return fmt.Errorf("slotquery: IV-SQ-2 %s=%q: %w", p.Field, id, err)
		}
	}
	return nil
}

func (p Predicate) scalarValues() []any {
	if arr, ok := p.Value.([]any); ok {
		return arr
	}
	return []any{p.Value}
}
