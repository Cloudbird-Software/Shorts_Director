package qc

import (
	"context"
	"strings"
	"testing"
)

// mockProbe 是测试用算子桩：返回固定测量值并计数调用次数。
type mockProbe struct {
	id    string
	cost  CostTier
	value any
	calls int
}

func (m *mockProbe) ID() string     { return m.id }
func (m *mockProbe) Cost() CostTier { return m.cost }
func (m *mockProbe) Measure(ctx context.Context, subj *Subject, args map[string]any) (Measurement, error) {
	m.calls++
	return Measurement{Value: m.value, EvidenceURI: "s3://qc/evidence/" + m.id + ".json"}, nil
}

func boolPtr(b bool) *bool { return &b }

// subjWith 最小被检对象。
func subjWith(fields map[string]any) *Subject {
	return &Subject{
		MediaURI:  "s3://media/render.mp4",
		MediaHash: "sha256:" + strings.Repeat("a", 64),
		Fields:    fields,
	}
}

func TestRunPass(t *testing.T) {
	black := &mockProbe{id: "blackdetect_ratio", cost: CostLight, value: 0.01}
	e, err := NewEngine(black)
	if err != nil {
		t.Fatal(err)
	}
	assertions := []Assertion{{
		AssertionID: "L0.BLACK_FRAMES.default",
		Level:       L0, Severity: SeverityBlocker,
		Probe:  Probe{Op: "blackdetect_ratio", Args: map[string]any{"threshold": 0.98}},
		Expect: Expect{Op: "lte", Value: 0.02},
		Remedy: Remedy{Action: "REJECT_SOURCE", InstructionTemplate: "黑帧占比 {{measured}} 超限 {{expected}}，素材报废"},
	}}
	rep, err := e.Run(context.Background(), subjWith(nil), assertions)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Pass() {
		t.Fatalf("期望通过，得到 remedy_sheet=%v", rep.RemedySheet)
	}
	if rep.Results[0].EvidenceURI == "" {
		t.Fatal("证据 URI 缺失（A2：非确定性显式落盘）")
	}
	if black.calls != 1 {
		t.Fatalf("probe 调用 %d 次，期望 1", black.calls)
	}
}

func TestRunFailRendersInstruction(t *testing.T) {
	black := &mockProbe{id: "blackdetect_ratio", cost: CostLight, value: 0.05}
	e, _ := NewEngine(black)
	assertions := []Assertion{{
		AssertionID: "L0.BLACK_FRAMES.default",
		Level:       L0, Severity: SeverityBlocker,
		Probe:  Probe{Op: "blackdetect_ratio", Args: map[string]any{}},
		Expect: Expect{Op: "lte", Value: 0.02},
		Remedy: Remedy{Action: "REJECT_SOURCE", InstructionTemplate: "黑帧占比 {{measured}} 超限 {{expected}}，素材报废"},
	}}
	rep, err := e.Run(context.Background(), subjWith(nil), assertions)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Pass() {
		t.Fatal("期望失败")
	}
	got := rep.RemedySheet[0].Instruction
	want := "黑帧占比 0.05 超限 0.02，素材报废"
	if got != want {
		t.Fatalf("指令渲染 %q，期望 %q", got, want)
	}
	if rep.RemedySheet[0].Action != "REJECT_SOURCE" {
		t.Fatalf("action %q", rep.RemedySheet[0].Action)
	}
}

func TestAppliesWhenFilter(t *testing.T) {
	lip := &mockProbe{id: "lipsync_lse_c", cost: CostHeavy, value: 5.0}
	e, _ := NewEngine(lip)
	assertions := []Assertion{{
		AssertionID: "L2.LIPSYNC.default",
		Level:       L2, Severity: SeverityMajor,
		// applies_when: has_lipsync=true，但被检对象无口型片段
		AppliesWhen: predicateHasLipsync(true),
		Probe:       Probe{Op: "lipsync_lse_c", Args: map[string]any{}},
		Expect:      Expect{Op: "gte", Value: 6.0},
		Remedy:      Remedy{Action: "REGENERATE", InstructionTemplate: "口型同步 LSE-C {{measured}}，请重新生成"},
	}}
	rep, err := e.Run(context.Background(), subjWith(map[string]any{"has_lipsync": false}), assertions)
	if err != nil {
		t.Fatal(err)
	}
	if lip.calls != 0 {
		t.Fatalf("条件不符仍执行 probe %d 次", lip.calls)
	}
	if rep.Results[0].Skipped == "" {
		t.Fatal("期望标记 Skipped")
	}
	if !rep.Pass() {
		t.Fatal("条件不符的断言不应判失败")
	}
}

func TestProbeDedup(t *testing.T) {
	sharp := &mockProbe{id: "laplacian_var", cost: CostLight, value: 150.0}
	e, _ := NewEngine(sharp)
	args := map[string]any{"sample": "N_UNIFORM", "n": 8}
	assertions := []Assertion{
		{
			AssertionID: "L0.SHARPNESS.min", Level: L0, Severity: SeverityBlocker,
			Probe: Probe{Op: "laplacian_var", Args: args}, Expect: Expect{Op: "gte", Value: 120},
			Remedy: Remedy{Action: "RESHOOT", InstructionTemplate: "画面模糊 {{measured}}"},
		},
		{
			AssertionID: "L0.SHARPNESS.tier", Level: L0, Severity: SeverityMinor,
			Probe: Probe{Op: "laplacian_var", Args: args}, Expect: Expect{Op: "gte", Value: 100},
			Remedy: Remedy{Action: "RECOLOR", InstructionTemplate: "清晰度 {{measured}}"},
		},
	}
	rep, err := e.Run(context.Background(), subjWith(nil), assertions)
	if err != nil {
		t.Fatal(err)
	}
	if sharp.calls != 1 {
		t.Fatalf("同 op+args 应去重，实际调用 %d 次", sharp.calls)
	}
	if !rep.Pass() {
		t.Fatal("两条断言都应通过")
	}
	if len(rep.Results) != 2 {
		t.Fatalf("结果数 %d", len(rep.Results))
	}
}

func TestBlockerShortCircuit(t *testing.T) {
	// 免费探针先跑并 BLOCKER 失败 → 重量探针绝不执行（成本控制核心）
	free := &mockProbe{id: "ffprobe_field", cost: CostFree, value: 719.0}
	heavy := &mockProbe{id: "lipsync_lse_c", cost: CostHeavy, value: 9.0}
	e, _ := NewEngine(free, heavy)
	assertions := []Assertion{
		{
			AssertionID: "L0.RESOLUTION.min", Level: L0, Severity: SeverityBlocker,
			Probe: Probe{Op: "ffprobe_field", Args: map[string]any{"field": "width"}}, Expect: Expect{Op: "gte", Value: 1080},
			Remedy: Remedy{Action: "REJECT_SOURCE", InstructionTemplate: "分辨率 {{measured}} 不足 {{expected}}"},
		},
		{
			AssertionID: "L2.LIPSYNC.default", Level: L2, Severity: SeverityMajor,
			Probe: Probe{Op: "lipsync_lse_c", Args: map[string]any{}}, Expect: Expect{Op: "gte", Value: 6.0},
			Remedy: Remedy{Action: "REGENERATE", InstructionTemplate: "口型 {{measured}}"},
		},
	}
	rep, err := e.Run(context.Background(), subjWith(nil), assertions)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.ShortCircuited {
		t.Fatal("期望 BLOCKER 短路")
	}
	if heavy.calls != 0 {
		t.Fatalf("BLOCKER 失败后重量 probe 仍执行 %d 次", heavy.calls)
	}
	if rep.Results[1].Skipped != "BLOCKER 短路" {
		t.Fatalf("L2 断言应标记短路跳过，得到 %+v", rep.Results[1])
	}
	if rep.Pass() {
		t.Fatal("BLOCKER 失败应判不通过")
	}
}

func TestCostOrderingCheapFirst(t *testing.T) {
	var order []string
	mk := func(id string, c CostTier) *trackingProbe {
		return &trackingProbe{id: id, cost: c, order: &order}
	}
	e, _ := NewEngine(mk("lipsync_lse_c", CostHeavy), mk("blackdetect_ratio", CostLight), mk("ffprobe_field", CostFree))
	assertions := []Assertion{
		{
			AssertionID: "L2.LIPSYNC.default", Level: L2, Severity: SeverityMajor,
			Probe: Probe{Op: "lipsync_lse_c", Args: map[string]any{}}, Expect: Expect{Op: "gte", Value: 6.0},
			Remedy: Remedy{Action: "REGENERATE", InstructionTemplate: "x"},
		},
		{
			AssertionID: "L0.BLACK_FRAMES.default", Level: L0, Severity: SeverityMajor,
			Probe: Probe{Op: "blackdetect_ratio", Args: map[string]any{}}, Expect: Expect{Op: "lte", Value: 0.02},
			Remedy: Remedy{Action: "REJECT_SOURCE", InstructionTemplate: "x"},
		},
		{
			AssertionID: "L0.FPS.min", Level: L0, Severity: SeverityMajor,
			Probe: Probe{Op: "ffprobe_field", Args: map[string]any{"field": "fps"}}, Expect: Expect{Op: "eq", Value: 25.0},
			Remedy: Remedy{Action: "REJECT_SOURCE", InstructionTemplate: "x"},
		},
	}
	if _, err := e.Run(context.Background(), subjWith(nil), assertions); err != nil {
		t.Fatal(err)
	}
	if strings.Join(order, ",") != "ffprobe_field,blackdetect_ratio,lipsync_lse_c" {
		t.Fatalf("执行顺序 %v，期望按成本免费→轻量→重量", order)
	}
}

// trackingProbe 记录跨 probe 的全局执行顺序。
type trackingProbe struct {
	id    string
	cost  CostTier
	order *[]string
}

func (p *trackingProbe) ID() string     { return p.id }
func (p *trackingProbe) Cost() CostTier { return p.cost }
func (p *trackingProbe) Measure(ctx context.Context, subj *Subject, args map[string]any) (Measurement, error) {
	*p.order = append(*p.order, p.id)
	return Measurement{Value: 25.0}, nil
}

func TestProbeArgsTemplate(t *testing.T) {
	det := &recordingProbe{id: "object_present"}
	e, _ := NewEngine(det)
	assertions := []Assertion{{
		AssertionID: "L1.SUBJECT_PRESENT.default",
		Level:       L1, Severity: SeverityMajor,
		Probe: Probe{Op: "object_present", Args: map[string]any{
			"object": "{{spec.subject}}", "conf": "{{spec.conf}}",
		}},
		Expect: Expect{Op: "is_true", Value: true},
		Remedy: Remedy{Action: "RESHOOT", InstructionTemplate: "未检出主体 {{spec.subject}}"},
	}}
	subj := subjWith(nil)
	subj.Spec = map[string]any{
		"spec": map[string]any{"subject": "DISH_FINISHED", "conf": 0.35},
	}
	if _, err := e.Run(context.Background(), subj, assertions); err != nil {
		t.Fatal(err)
	}
	if got := det.lastArgs["object"]; got != "DISH_FINISHED" {
		t.Fatalf("object 渲染为 %#v", got)
	}
	// 纯占位模板保留原类型：conf 是数值而非 "0.35" 字符串
	if conf, ok := det.lastArgs["conf"].(float64); !ok || conf != 0.35 {
		t.Fatalf("conf 应保持数值 0.35，得到 %#v", det.lastArgs["conf"])
	}
}

// recordingProbe 记录最近一次收到的 args。
type recordingProbe struct {
	id       string
	lastArgs map[string]any
}

func (p *recordingProbe) ID() string     { return p.id }
func (p *recordingProbe) Cost() CostTier { return CostLight }
func (p *recordingProbe) Measure(ctx context.Context, subj *Subject, args map[string]any) (Measurement, error) {
	m := make(map[string]any, len(args))
	for k, v := range args {
		m[k] = v
	}
	p.lastArgs = m
	return Measurement{Value: true}, nil
}

func TestRemedyUnknownVarFails(t *testing.T) {
	black := &mockProbe{id: "blackdetect_ratio", cost: CostLight, value: 0.05}
	e, _ := NewEngine(black)
	assertions := []Assertion{{
		AssertionID: "L0.BLACK_FRAMES.default",
		Level:       L0, Severity: SeverityBlocker,
		Probe:  Probe{Op: "blackdetect_ratio", Args: map[string]any{}},
		Expect: Expect{Op: "lte", Value: 0.02},
		Remedy: Remedy{Action: "REJECT_SOURCE", InstructionTemplate: "{{no_such_var}} 超限"},
	}}
	if _, err := e.Run(context.Background(), subjWith(nil), assertions); err == nil {
		t.Fatal("未知模板变量应报错（宁失败不静默）")
	}
}

func TestCompareExpect(t *testing.T) {
	cases := []struct {
		name     string
		op       string
		value    any
		measured any
		pass     bool
		wantErr  bool
	}{
		{"gte 通过", "gte", 120.0, 150.0, true, false},
		{"gte 失败", "gte", 120.0, 80.0, false, false},
		{"between 内", "between", []any{80.0, 200.0}, 150.0, true, false},
		{"between 外", "between", []any{80.0, 200.0}, 300.0, false, false},
		{"between 非数值", "between", []any{80.0, 200.0}, "hi", false, true},
		{"is_true 真", "is_true", true, true, true, false},
		{"is_true 假测量", "is_true", true, false, false, false},
		{"is_false", "is_false", true, false, true, false},
		{"contains_none 无命中", "contains_none", []any{"国家级", "第一"}, []any{"家常", "实惠"}, true, false},
		{"contains_none 有命中", "contains_none", []any{"国家级", "第一"}, []any{"国家级"}, false, false},
		{"contains_all 命中", "contains_all", []any{"效果因人而异"}, []any{"效果因人而异"}, true, false},
		{"neq 标量", "neq", "GEN_VIDEO", "SHOOT", true, false},
		{"eq 字符串", "eq", "25", "25", true, false},
		{"gte 非数值", "gte", 120.0, "blurry", false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pass, err := compareExpect(Expect{Op: c.op, Value: c.value}, c.measured)
			if c.wantErr {
				if err == nil {
					t.Fatal("期望报错")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if pass != c.pass {
				t.Fatalf("pass=%v，期望 %v", pass, c.pass)
			}
		})
	}
}

func TestValidateRejectSourceInVocab(t *testing.T) {
	a := Assertion{
		AssertionID: "L0.BLACK_FRAMES.default",
		Level:       L0, Severity: SeverityBlocker,
		Probe:  Probe{Op: "blackdetect_ratio", Args: map[string]any{}},
		Expect: Expect{Op: "lte", Value: 0.02},
		Remedy: Remedy{Action: "REJECT_SOURCE", InstructionTemplate: "素材报废"},
	}
	if err := a.Validate(); err != nil {
		t.Fatalf("REJECT_SOURCE 应在词表内: %v", err)
	}
}

func TestNewEngineDuplicateProbe(t *testing.T) {
	if _, err := NewEngine(&mockProbe{id: "fps"}, &mockProbe{id: "fps"}); err == nil {
		t.Fatal("重复注册应报错")
	}
}

// predicateHasLipsync 构造 applies_when 谓词。
func predicateHasLipsync(v bool) *Predicate {
	return &Predicate{Op: "eq", Field: "has_lipsync", Value: v}
}
