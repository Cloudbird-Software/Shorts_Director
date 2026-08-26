package slotquery

import (
	"fmt"

	"github.com/Cloudbird-Software/Shorts_Director/internal/entity"
	"github.com/Cloudbird-Software/Shorts_Director/internal/jsoncmp"
)

// Evaluate 对 shot 求谓词值。字段无值（打标未覆盖）时 eq/in 类谓词按
// "不匹配"处理；类型不匹配（数值谓词用于字符串字段）按错误处理——
// 这是契约级 bug，不允许静默。
func Evaluate(p Predicate, s *entity.Shot) (bool, error) {
	return evalWith(p, func(field string) (any, bool) {
		return FieldValue(s, field)
	})
}

// EvaluateFields 对属性字典求谓词值（QC applies_when 的求值入口）。
// 字段域仍是 IV-SQ-2 白名单；语义与 Evaluate 完全一致：无值不匹配、
// 类型不匹配报错。字典适合承载不在 Shot 上的关联字段（如 Asset 的
// source_kind）——由编排层展平注入。
func EvaluateFields(p Predicate, fields map[string]any) (bool, error) {
	return evalWith(p, func(field string) (any, bool) {
		v, ok := fields[field]
		return v, ok
	})
}

// FieldValue 返回 shot 白名单字段的值（标量或数值统一为 any；
// 打标未覆盖时 ok=false）。QC/编排层用它把 Shot 展平进属性字典。
func FieldValue(s *entity.Shot, field string) (any, bool) {
	if v, ok := scalarField(s, field); ok {
		return v, true
	}
	f, ok := numberField(s, field)
	return f, ok
}

// FlattenShot 把 shot 的全部白名单字段展平为属性字典（未打标字段缺省）。
// QC applies_when 等跨实体求值用它把 Shot 并入属性集。
func FlattenShot(s *entity.Shot) map[string]any {
	out := map[string]any{}
	for f := range fieldWhitelist {
		if v, ok := FieldValue(s, f); ok {
			out[f] = v
		}
	}
	return out
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
	case "semantic":
		return false, ErrSemanticNotRankable
	}

	if !fieldWhitelist[p.Field] {
		return false, fmt.Errorf("slotquery: 字段 %q 不在白名单", p.Field)
	}

	got, ok := get(p.Field)
	if !ok {
		return false, nil // 字段无值：不匹配
	}

	if numericOps[p.Op] || p.Op == "between" {
		f, isNum := jsoncmp.Float(got)
		if !isNum {
			return false, fmt.Errorf("slotquery: op %s 用于非数值字段 %q", p.Op, p.Field)
		}
		return compareNumber(p, f)
	}
	return compareScalar(p, got)
}

// compareNumber 求数值比较（gte/lte/gt/lt/between）。
func compareNumber(p Predicate, got float64) (bool, error) {
	if p.Op == "between" {
		if len(p.Range) != 2 {
			return false, fmt.Errorf("slotquery: between 需要 range=[min,max]")
		}
		return got >= p.Range[0] && got <= p.Range[1], nil
	}
	want, ok := jsoncmp.Float(p.Value)
	if !ok {
		return false, fmt.Errorf("slotquery: op %s 的 value 必须是数值", p.Op)
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
	return false, fmt.Errorf("slotquery: 非数值操作符 %q", p.Op)
}

// compareScalar 求标量比较（eq/neq/in/nin）。
// 多值字段（subjects/actions/mood 等 []any）语义为成员关系：
// eq ⇔ 包含；neq ⇔ 不包含；in ⇔ 有交集；nin ⇔ 无交集。
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
			return false, fmt.Errorf("slotquery: in 的 value 必须是数组")
		}
		return intersects(want), nil
	case "nin":
		want, ok := p.Value.([]any)
		if !ok {
			return false, fmt.Errorf("slotquery: nin 的 value 必须是数组")
		}
		return !intersects(want), nil
	}
	return false, fmt.Errorf("slotquery: 非标量操作符 %q", p.Op)
}

// scalarField 从 shot 取白名单字段的标量/数组值；ok=false 表示未打标。
func scalarField(s *entity.Shot, field string) (any, bool) {
	switch field {
	case "shot_type":
		return s.Semantic.ShotType, s.Semantic.ShotType != ""
	case "shot_type_class":
		return toAny(s.Semantic.ShotTypeClasses), len(s.Semantic.ShotTypeClasses) > 0
	case "camera_motion.type":
		if s.Affordance.CameraMotion == nil {
			return nil, false
		}
		t := s.Affordance.CameraMotion.Type
		if t == nil {
			return nil, false
		}
		return *t, true
	case "camera_motion.dir":
		if s.Affordance.CameraMotion == nil || s.Affordance.CameraMotion.Dir == nil {
			return nil, false
		}
		return *s.Affordance.CameraMotion.Dir, true
	case "scene":
		return s.Semantic.Scene, s.Semantic.Scene != ""
	case "subject":
		return toAny(s.Semantic.Subjects), len(s.Semantic.Subjects) > 0
	case "action":
		return toAny(s.Semantic.Actions), len(s.Semantic.Actions) > 0
	case "mood":
		return toAny(s.Semantic.Mood), len(s.Semantic.Mood) > 0
	case "negative_space":
		return toAny(s.Affordance.NegativeSpace), len(s.Affordance.NegativeSpace) > 0
	case "is_loopable":
		return boolOr(s.Affordance.IsLoopable)
	case "clean_in":
		return boolOr(s.Affordance.CleanIn)
	case "clean_out":
		return boolOr(s.Affordance.CleanOut)
	case "has_speech":
		return boolOr(s.Affordance.HasSpeech)
	case "has_lipsync":
		return boolOr(s.Affordance.HasLipsync)
	case "safe_crop_9x16.ok":
		if s.Affordance.SafeCrop9x16 == nil {
			return nil, false
		}
		return s.Affordance.SafeCrop9x16.OK, true
	case "season":
		return toAny(s.Lifecycle.Seasons), len(s.Lifecycle.Seasons) > 0
	case "ttl_at":
		return strOr(s.Lifecycle.TTLAt)
	case "linked_sku":
		return toAny(s.Lifecycle.LinkedSKUs), len(s.Lifecycle.LinkedSKUs) > 0
	case "last_used_at":
		return strOr(s.Lifecycle.LastUsedAt)
	}
	return nil, false
}

// numberField 从 shot 取数值字段（缺失 ⇒ ok=false 不匹配）。
func numberField(s *entity.Shot, field string) (float64, bool) {
	switch field {
	case "motion_energy":
		return floatOr(s.Affordance.MotionEnergy)
	case "duration":
		return float64(s.Identity.OutFrame - s.Identity.InFrame), true
	case "quality_tier":
		return floatOrInt(s.Technical.QualityTier)
	case "use_count":
		return float64(s.Lifecycle.UseCount), true
	case "sharpness_tier":
		// sharpness_tier 是 technical.sharpness 的分桶（桶界与 QC L1 对齐）。
		if s.Technical.Sharpness == nil {
			return 0, false
		}
		return sharpnessTier(*s.Technical.Sharpness), true
	}
	return 0, false
}

// sharpnessTier 将 laplacian_var 分桶为 1–4（tier 越高越清晰）。
func sharpnessTier(laplacianVar float64) float64 {
	switch {
	case laplacianVar >= 500:
		return 4
	case laplacianVar >= 200:
		return 3
	case laplacianVar >= 80:
		return 2
	default:
		return 1
	}
}

func floatOr(p *float64) (float64, bool) {
	if p == nil {
		return 0, false
	}
	return *p, true
}

func floatOrInt(p *int) (float64, bool) {
	if p == nil {
		return 0, false
	}
	return float64(*p), true
}

func boolOr(p *bool) (any, bool) {
	if p == nil {
		return nil, false
	}
	return *p, true
}

func strOr(p *string) (any, bool) {
	if p == nil {
		return nil, false
	}
	return *p, true
}

func toAny(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}
