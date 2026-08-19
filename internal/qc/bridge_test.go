package qc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Cloudbird-Software/Shorts_Director/internal/operator"
)

// goldenFake 落一条 golden fixture 并返回 FakeRunner。
// golden 请求 = 断言引擎经桥接器发出的确切请求形态。
func goldenFake(t *testing.T, op string, args map[string]any, respJSON string) *operator.FakeRunner {
	t.Helper()
	req := operator.Request{
		ContractVersion: 1, Op: op,
		Inputs: map[string]any{
			"media_path": "s3://media/render.mp4",
			"media_hash": "sha256:" + strings.Repeat("a", 64), // 与 subjWith 一致
		},
		Workdir:     "/tmp/qc/" + op,
		Determinism: operator.Determinism{Seed: nil},
	}
	for k, v := range args {
		req.Inputs[k] = v
	}
	key, err := operator.GoldenKey(req)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	opDir := filepath.Join(dir, op)
	if err := os.MkdirAll(opDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(opDir, key+".json"), []byte(respJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	return &operator.FakeRunner{Dir: dir}
}

func TestRunnerProbeAdapterMeasure(t *testing.T) {
	resp := `{"contract_version":1,"op":"blackdetect_ratio","status":"OK",
	 "outputs":{"value":0.01,"evidence_uri":"s3://qc/evidence/1.json"},
	 "metrics":{"wall_ms":5},"operator_version":"blackdetect@1.0"}`
	fake := goldenFake(t, "blackdetect_ratio", map[string]any{"threshold": 0.98}, resp)
	adapter := &RunnerProbeAdapter{
		Op: "blackdetect_ratio", Tier: CostLight, Runner: fake,
	}
	m, err := adapter.Measure(context.Background(), subjWith(nil), map[string]any{"threshold": 0.98})
	if err != nil {
		t.Fatal(err)
	}
	if m.Value != float64(0.01) {
		t.Fatalf("value=%#v", m.Value)
	}
	if m.EvidenceURI != "s3://qc/evidence/1.json" {
		t.Fatalf("evidence=%q", m.EvidenceURI)
	}
}

func TestRunnerProbeAdapterInputError(t *testing.T) {
	resp := `{"contract_version":1,"op":"blackdetect_ratio","status":"INPUT_ERROR",
	 "outputs":{},"metrics":{"wall_ms":1},"operator_version":"blackdetect@1.0",
	 "error":{"code":"BAD_MEDIA","message":"文件损坏"}}`
	fake := goldenFake(t, "blackdetect_ratio", map[string]any{"threshold": 0.98}, resp)
	adapter := &RunnerProbeAdapter{Op: "blackdetect_ratio", Tier: CostLight, Runner: fake}
	_, err := adapter.Measure(context.Background(), subjWith(nil), map[string]any{"threshold": 0.98})
	if err == nil {
		t.Fatal("INPUT_ERROR 应作为引擎错误（上游数据问题由编排层决策）")
	}
}

// Operators 批量构造 → NewEngine 全链路：断言引擎经 C2 Runner 拿到测量值。
func TestEngineWithRunnerBridge(t *testing.T) {
	resp := `{"contract_version":1,"op":"blackdetect_ratio","status":"OK",
	 "outputs":{"value":0.01},"metrics":{"wall_ms":5},"operator_version":"blackdetect@1.0"}`
	fake := goldenFake(t, "blackdetect_ratio", map[string]any{"threshold": 0.98}, resp)
	probes := Operators(fake, "/tmp/qc", map[string]CostTier{
		"blackdetect_ratio": CostLight,
	})
	e, err := NewEngine(probes...)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := e.Run(context.Background(), subjWith(nil), []Assertion{{
		AssertionID: "L0.BLACK_FRAMES.default",
		Level:       L0, Severity: SeverityBlocker,
		Probe:  Probe{Op: "blackdetect_ratio", Args: map[string]any{"threshold": 0.98}},
		Expect: Expect{Op: "lte", Value: 0.02},
		Remedy: Remedy{Action: "REJECT_SOURCE", InstructionTemplate: "黑帧 {{measured}}"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Pass() {
		t.Fatalf("rep=%+v", rep)
	}
	if len(probes) != 1 || probes[0].ID() != "blackdetect_ratio" {
		t.Fatalf("Operators 构造: %v", probes)
	}
}
