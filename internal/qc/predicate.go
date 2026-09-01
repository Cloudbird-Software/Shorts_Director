// predicate.go 是 QC applies_when 的受限谓词 AST（自 slotquery 迁移精简——
// IR-0007 退役 ShotSlotQuery 取材查询后，QC 保留谓词求值的最小子集）。
//
// 与原实现的差异：去掉 semantic（pgvector 检索语义，仅服务于已退役的
// 取材 should 排序）与 Shot 实体展平路径——QC 被检对象以 Fields 字典
// 注入求值属性（eval 域字段由套件定义提供）。
//
// 语义契约不变：字段无值（未覆盖）按"不匹配"处理；类型不匹配
// （数值谓词用于字符串字段）按错误处理——契约级 bug 不允许静默。
package qc

import (
	"fmt"
	"strings"

	vocabgen "github.com/Cloudbird-Software/Shorts_Director/codegen/go/vocab"
	"github.com/Cloudbird-Software/Shorts_Director/internal/jsoncmp"
	"github.com/Cloudbird-Software/Shorts_Director/internal/vocab"
)

// Predicate 是受限谓词 AST（qc_assertion.schema.json $defs/Predicate）。
// op ∈ eq/in/neq/nin/gte/lte/gt/lt/between/and/or/not。
type Predicate struct {
	Op       string      `json:"op"`
	Field    string      `json:"field,omitempty"`
	Value    any         `json:"value,omitempty"`
	Range    []float64   `json:"range,omitempty"`
	Operands []Predicate `json:"operands,omitempty"`
}

// fieldWhitelist 是 applies_when 可引用的字段白名单（迁移自 IV-SQ-2；
// eval 域新增字段在此追加并配 property test）。
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
	"source_kind": true, "gen_form": true, "model": true,
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

// 受控操作符集。
var (
	scalarOps  = map[string]bool{"eq": true, "neq": true, "in": true, "nin": true}
	numericOps = map[string]bool{"gte": true, "lte": true, "gt": true, "lt": true}
)

// Validate 校验谓词自身：字段白名单、op 与 value 形态、词表受控。
func (p Predicate) Validate() error {
	switch p.Op {
	case "and", "or", "not":
		if len(p.Operands) < 1 {
			return fmt.Errorf("qc: 逻辑算子 %s 需要 ≥1 operands", p.Op)
		}
		for i := range p.Operands {
			if err := p.Operands[i].Validate(); err != nil {
				return fmt.Errorf("qc: operands[%d]: %w", i, err)
			}
		}
		return nil
	case "between":
		if !fieldWhitelist[p.Field] {
			return fmt.Errorf("qc: applies_when 字段 %q 不在白名单", p.Field)
		}
		if len(p.Range) != 2 || p.Range[0] > p.Range[1] {
			return fmt.Errorf("qc: between 需要 range=[min,max] 且 min≤max")
		}
		return p.validateVocab()
	case "eq", "neq", "in", "nin", "gte", "lte", "gt", "lt":
		if !fieldWhitelist[p.Field] {
			return fmt.Errorf("qc: applies_when 字段 %q 不在白名单", p.Field)
		}
		if p.Value == nil {
			return fmt.Errorf("qc: op %s 需要 value", p.Op)
		}
		return p.validateVocab()
	default:
		return fmt.Errorf("qc: 非受控操作符 %q", p.Op)
	}
}

// validateVocab 校验词表绑定字段的取值受控（分表字段前缀匹配任意分表）。
func (p Predicate) validateVocab() error {
	prefix, bound := vocabFields[p.Field]
	if !bound {
		return nil
	}
	for _, v := range p.scalarValues() {
		id, ok := v.(string)
		if !ok {
			continue
		}
		if err := vocabContains(prefix, id); err != nil {
			return fmt.Errorf("qc: applies_when %s=%q: %w", p.Field, id, err)
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

// EvaluateFields 对属性字典求谓词值。字段无值不匹配、类型不匹配报错。
func EvaluateFields(p Predicate, fields map[string]any) (bool, error) {
	return evalWith(p, func(field string) (any, bool) {
		v, ok := fields[field]
		return v, ok
	})
}

// evalWith 是谓词求值的统一骨架：逻辑算子递归，叶子按字段取值器求比较。
func evalWith(p Predicate, get func(string) (any, bool)) (bool, error) {
	switch p.Op {
	case "and":
		for i, sub := range p.Operands {
			ok, err := evalWith(sub, get)
			if err != nil {
				return false, fmt.Errorf("and.operands[%d]: %w", i, err)
			}
			if !ok {
				return false, nil
			}
		}
		return true, nil
	case "or":
		for i, sub := range p.Operands {
			ok, err := evalWith(sub, get)
			if err != nil {
				return false, fmt.Errorf("or.operands[%d]: %w", i, err)
			}
			if ok {
				return true, nil
			}
		}
		return false, nil
	case "not":
		ok, err := evalWith(p.Operands[0], get)
		return !ok, err
	}

	if !fieldWhitelist[p.Field] {
		return false, fmt.Errorf("qc: applies_when 字段 %q 不在白名单", p.Field)
	}

	got, ok := get(p.Field)
	if !ok {
		return false, nil // 字段无值：不匹配
	}

	if numericOps[p.Op] || p.Op == "between" {
		f, isNum := jsoncmp.Float(got)
		if !isNum {
			return false, fmt.Errorf("qc: op %s 用于非数值字段 %q", p.Op, p.Field)
		}
		return compareNumber(p, f)
	}
	return compareScalar(p, got)
}

// compareNumber 求数值比较（gte/lte/gt/lt/between）。
func compareNumber(p Predicate, got float64) (bool, error) {
	if p.Op == "between" {
		if len(p.Range) != 2 {
			return false, fmt.Errorf("qc: between 需要 range=[min,max]")
		}
		return got >= p.Range[0] && got <= p.Range[1], nil
	}
	want, ok := jsoncmp.Float(p.Value)
	if !ok {
		return false, fmt.Errorf("qc: op %s 的 value 必须是数值", p.Op)
	}
	switch p.Op {
	case "gte":
		return got >= want, nil
	case "lte":
		return got <= want, nil
	case "gt":
		return got > want, nil
	case "lt":
		return got < want, nil
	}
	return false, fmt.Errorf("qc: 非数值操作符 %q", p.Op)
}

// compareScalar 求标量比较（eq/neq/in/nin）。
// 多值字段语义为成员关系：eq ⇔ 包含；neq ⇔ 不包含；in ⇔ 有交集；nin ⇔ 无交集。
func compareScalar(p Predicate, got any) (bool, error) {
	multi, isMulti := got.([]any)
	single := func(w any) bool { return jsoncmp.Equal(got, w) }
	intersects := func(want []any) bool {
		for _, w := range want {
			if isMulti {
				for _, e := range multi {
					if jsoncmp.Equal(e, w) {
						return true
					}
				}
			} else if jsoncmp.Equal(got, w) {
				return true
			}
		}
		return false
	}
	switch p.Op {
	case "eq":
		if isMulti {
			for _, e := range multi {
				if jsoncmp.Equal(e, p.Value) {
					return true, nil
				}
			}
			return false, nil
		}
		return single(p.Value), nil
	case "neq":
		if isMulti {
			for _, e := range multi {
				if jsoncmp.Equal(e, p.Value) {
					return false, nil
				}
			}
			return true, nil
		}
		return !single(p.Value), nil
	case "in":
		want, ok := p.Value.([]any)
		if !ok {
			return false, fmt.Errorf("qc: in 的 value 必须是数组")
		}
		return intersects(want), nil
	case "nin":
		want, ok := p.Value.([]any)
		if !ok {
			return false, fmt.Errorf("qc: nin 的 value 必须是数组")
		}
		return !intersects(want), nil
	}
	return false, fmt.Errorf("qc: 非标量操作符 %q", p.Op)
}
