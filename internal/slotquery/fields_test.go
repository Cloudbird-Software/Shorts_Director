package slotquery

import (
	"testing"

	"github.com/Cloudbird-Software/Shorts_Director/internal/entity"
)

// EvaluateFields 是 QC applies_when 的求值入口：属性字典可承载
// 不在 Shot 上的关联字段（如 Asset 的 source_kind）。
func TestEvaluateFields(t *testing.T) {
	fields := map[string]any{
		"source_kind": "GEN_VIDEO",
		"has_lipsync": true,
		"duration":    float64(90),
	}
	// L2 断言的典型条件：source_kind in [GEN_*]
	gen := Predicate{Op: "in", Field: "source_kind", Value: []any{"GEN_VIDEO", "GEN_IMAGE"}}
	ok, err := EvaluateFields(gen, fields)
	if err != nil || !ok {
		t.Fatalf("GEN_VIDEO 应命中: ok=%v err=%v", ok, err)
	}
	shot := Predicate{Op: "eq", Field: "source_kind", Value: "SHOOT"}
	ok, err = EvaluateFields(shot, fields)
	if err != nil || ok {
		t.Fatalf("SHOOT 不应命中: ok=%v err=%v", ok, err)
	}
	// 数值谓词 + 无值字段：不匹配而非错误
	missing := Predicate{Op: "gte", Field: "motion_energy", Value: 0.3}
	ok, err = EvaluateFields(missing, fields)
	if err != nil || ok {
		t.Fatalf("无值字段应不匹配: ok=%v err=%v", ok, err)
	}
	// 数值谓词用于字符串字段：契约错误（不静默）
	bad := Predicate{Op: "gte", Field: "source_kind", Value: 1}
	if _, err = EvaluateFields(bad, fields); err == nil {
		t.Fatal("数值谓词用于字符串字段应报错")
	}
}

func TestFlattenShot(t *testing.T) {
	s := &entity.Shot{
		ID: "0192b4d0-8f00-7000-8000-000000000001", AssetID: "a", TenantID: "t",
		State:    entity.ShotAvailable,
		Identity: entity.ShotIdentity{InFrame: 0, OutFrame: 75, FPS: 25},
		Semantic: entity.ShotSemantic{Scene: "COUNTER", Subjects: []string{"DISH_FINISHED"}},
	}
	m := FlattenShot(s)
	if m["scene"] != "COUNTER" {
		t.Fatalf("scene=%v", m["scene"])
	}
	if d, ok := m["duration"].(float64); !ok || d != 75 {
		t.Fatalf("duration=%#v", m["duration"])
	}
	if _, has := m["mood"]; has {
		t.Fatal("未打标字段不应出现")
	}
	// 展平结果可直接被 EvaluateFields 消费（QC 编排路径闭环）
	p := Predicate{Op: "eq", Field: "scene", Value: "COUNTER"}
	ok, err := EvaluateFields(p, m)
	if err != nil || !ok {
		t.Fatalf("展平属性应可求值: ok=%v err=%v", ok, err)
	}
}
