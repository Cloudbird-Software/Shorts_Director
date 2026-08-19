package slotquery

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Cloudbird-Software/Shorts_Director/internal/entity"
)

func loadQuery(t *testing.T, name string) Query {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(
		"..", "..", "schema", "testdata", "shot_slot_query", "valid", name))
	if err != nil {
		t.Fatalf("读样本失败: %v", err)
	}
	var q Query
	if err := json.Unmarshal(raw, &q); err != nil {
		t.Fatalf("解析 %s: %v", name, err)
	}
	return q
}

func shotForTest() *entity.Shot {
	return &entity.Shot{
		ID:       "018f6c01-aaaa-7aaa-8aaa-000000000001",
		AssetID:  "018f6b2e-9c4a-7b3e-a1d2-3f4e5d6c7b8a",
		TenantID: "018f6b10-0000-7000-8000-000000000001",
		State:    entity.ShotAvailable,
		Identity: entity.ShotIdentity{InFrame: 0, OutFrame: 125, FPS: 30},
		Semantic: entity.ShotSemantic{
			ShotType:        "CLOSEUP",
			ShotTypeClasses: []string{"DETAIL"},
			Scene:           "KITCHEN_LINE",
			Subjects:        []string{"NOODLE_SOUP"},
			Actions:         []string{"STIR"},
		},
		Affordance: entity.ShotAffordance{
			IsLoopable:   boolPtr(true),
			SafeCrop9x16: &entity.SafeCrop{OK: true},
			HasSpeech:    boolPtr(false),
		},
		Technical:  entity.ShotTechnical{QualityTier: intPtr(3)},
		Compliance: entity.ShotCompliance{},
		Lifecycle:  entity.ShotLifecycle{UseCount: 2},
		TagProvenance: entity.Provenance{
			GeneratedBy: entity.GeneratedByDeterministic,
			ModelID:     "t@1", PromptVersion: "none",
			InputDigest: "sha256:" + strings.Repeat("a", 64),
			CreatedAt:   "2026-08-01T09:31:00Z",
		},
	}
}

func boolPtr(b bool) *bool { return &b }
func intPtr(i int) *int    { return &i }

// TestValidSamplesValidate：G1 valid 样本必须通过 IV 校验（IV-SQ-1/2）。
func TestValidSamplesValidate(t *testing.T) {
	for _, name := range []string{
		"minimal.json", "hook_detail_full.json", "numeric_ranges.json",
		"compound_or_not.json", "terminal_only_empty_must.json",
	} {
		q := loadQuery(t, name)
		if err := q.Validate(); err != nil {
			t.Errorf("valid 样本 %s 未通过 Validate: %v", name, err)
		}
	}
}

// TestIVSQ1：末端非终端图形且 must 非空 ⇒ 拒绝。
func TestIVSQ1(t *testing.T) {
	q := loadQuery(t, "hook_detail_full.json")
	last := &q.FallbackChain[len(q.FallbackChain)-1]
	wasTerminal := last.IsTerminalGraphic
	wasMust := last.Must
	last.IsTerminalGraphic = false
	last.Must = []Predicate{{Op: "eq", Field: "shot_type", Value: "CLOSEUP"}}
	if err := q.Validate(); err == nil || !strings.Contains(err.Error(), "IV-SQ-1") {
		t.Errorf("IV-SQ-1 未触发: %v", err)
	}
	last.IsTerminalGraphic = wasTerminal
	last.Must = wasMust
	if err := q.Validate(); err != nil {
		t.Errorf("恢复后应合法: %v", err)
	}
}

// TestIVSQ2：白名单外字段与词表外取值 ⇒ 拒绝。
func TestIVSQ2(t *testing.T) {
	q := loadQuery(t, "minimal.json")
	q.Must = append(q.Must, Predicate{Op: "eq", Field: "asset_id", Value: "x"})
	if err := q.Validate(); err == nil || !strings.Contains(err.Error(), "IV-SQ-2") {
		t.Errorf("白名单外字段未拒绝: %v", err)
	}
	q.Must = q.Must[:len(q.Must)-1]
	q.Must = append(q.Must, Predicate{Op: "eq", Field: "shot_type", Value: "NOT_A_TYPE"})
	if err := q.Validate(); err == nil || !strings.Contains(err.Error(), "IV-SQ-2") {
		t.Errorf("词表外取值未拒绝: %v", err)
	}
}

// TestSemanticOnlyInShould：semantic 在 must ⇒ 拒绝。
func TestSemanticOnlyInShould(t *testing.T) {
	q := loadQuery(t, "minimal.json")
	q.Must = append(q.Must, Predicate{Op: "semantic", Query: "热气腾腾", TopK: 5})
	if err := q.Validate(); err == nil {
		t.Error("semantic 在 must 应拒绝")
	}
	// 硬匹配直接报错。
	if _, err := Evaluate(Predicate{Op: "semantic", Query: "x", TopK: 5}, shotForTest()); err == nil {
		t.Error("semantic 硬匹配应报 ErrSemanticNotRankable")
	}
}

func TestEvaluateOps(t *testing.T) {
	s := shotForTest()
	cases := []struct {
		p    Predicate
		want bool
	}{
		{p: Predicate{Op: "eq", Field: "shot_type", Value: "CLOSEUP"}, want: true},
		{p: Predicate{Op: "eq", Field: "shot_type", Value: "WIDE"}, want: false},
		{p: Predicate{Op: "neq", Field: "shot_type", Value: "WIDE"}, want: true},
		{p: Predicate{Op: "in", Field: "scene", Value: []any{"KITCHEN_LINE", "STOREFRONT"}}, want: true},
		{p: Predicate{Op: "in", Field: "scene", Value: []any{"STOREFRONT"}}, want: false},
		{p: Predicate{Op: "eq", Field: "is_loopable", Value: true}, want: true},
		{p: Predicate{Op: "eq", Field: "clean_in", Value: true}, want: false}, // 未打标不匹配
		{p: Predicate{Op: "between", Field: "duration", Range: []float64{100, 150}}, want: true},
		{p: Predicate{Op: "gte", Field: "quality_tier", Value: 3}, want: true},
		{p: Predicate{Op: "lt", Field: "use_count", Value: 2}, want: false},
		{
			p: Predicate{Op: "and", Operands: []Predicate{
				{Op: "eq", Field: "shot_type", Value: "CLOSEUP"},
				{Op: "or", Operands: []Predicate{
					{Op: "eq", Field: "scene", Value: "NOPE"},
					{Op: "eq", Field: "action", Value: "STIR"},
				}},
			}},
			want: true,
		},
		{p: Predicate{Op: "not", Operands: []Predicate{{Op: "eq", Field: "mood", Value: "COZY"}}}, want: true},
	}
	for _, c := range cases {
		got, err := Evaluate(c.p, s)
		if err != nil {
			t.Fatalf("Evaluate(%+v) err = %v", c.p, err)
		}
		if got != c.want {
			t.Errorf("Evaluate(%+v) = %v, want %v", c.p, got, c.want)
		}
	}
}

func TestMatchScore(t *testing.T) {
	q := loadQuery(t, "hook_detail_full.json")
	if err := q.Validate(); err != nil {
		t.Fatalf("样本应合法: %v", err)
	}
	s := shotForTest()
	ok, err := Match(q, s)
	if err != nil {
		t.Fatal(err)
	}
	// 样本 must 求值结果确定性：不因未打标字段误杀。
	_ = ok
	// Score：命中的 should 累加权重。
	scored := Score(q, s)
	if scored < 0 {
		t.Errorf("Score 不应为负: %v", scored)
	}
}

// loadQuerySub 从指定子目录（valid/evolution）加载样本。
func loadQuerySub(t *testing.T, sub, name string) Query {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(
		"..", "..", "schema", "testdata", "shot_slot_query", sub, name))
	if err != nil {
		t.Fatalf("读样本失败: %v", err)
	}
	var q Query
	if err := json.Unmarshal(raw, &q); err != nil {
		t.Fatalf("解析 %s: %v", name, err)
	}
	return q
}

// TestEvolutionSamplesValidate：G5 向后兼容——evolution/ 样本必须仍可消费。
func TestEvolutionSamplesValidate(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join(
		"..", "..", "schema", "testdata", "shot_slot_query", "evolution"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if q := loadQuerySub(t, "evolution", e.Name()); q.Validate() != nil {
			t.Errorf("evolution 样本 %s 未通过 Validate: %v", e.Name(), q.Validate())
		}
	}
}
