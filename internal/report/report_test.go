// report_test.go —— 卡 #122（IR-0007 AC-5/AC-9/AC-10）：出片率报告聚合
// 与产能外推口径单测：复算一致性执法、profile 引用一致、估算标注、
// 无电价变量、A100 迁移口径、fake 警示、digest 确定性。
package report_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Cloudbird-Software/Shorts_Director/internal/eval"
	"github.com/Cloudbird-Software/Shorts_Director/internal/report"
)

const testProfileDigest = "sha256:cafebabe"
const testDate = "2026-09-02"

// writeProfile 写最小 capability profile。
func writeProfile(t *testing.T, dir string) string {
	t.Helper()
	p := filepath.Join(dir, "profile.json")
	if err := os.WriteFile(p, []byte(
		`{"schema_version":"1","digest":"`+testProfileDigest+`","gpu":{"present":false}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// artifactFixture 构造一个 form1/form4 形态的 run artifact JSON 并以
// <digesthex>.json 落盘（复算 digest 由调用方注入，保证文件名一致）。
type itemFixture struct {
	EntryID string
	Seed    int64
	Status  string
	Usable  bool
	WallMs  int64
	TotalMs int64 // timing.total_ms（0 = 纯生成套件回落 WallMs）
}

func writeArtifact(t *testing.T, dir, suiteID, model, runnerMode string,
	items []itemFixture) string {
	t.Helper()
	var docItems []map[string]any
	var evalItems []eval.ItemResult
	for _, f := range items {
		d := map[string]any{
			"entry_id": f.EntryID, "seed": f.Seed, "status": f.Status,
			"usable": f.Usable, "wall_ms": f.WallMs,
		}
		if f.TotalMs > 0 {
			d["timing"] = map[string]any{"total_ms": f.TotalMs}
		}
		docItems = append(docItems, d)
		evalItems = append(evalItems, eval.ItemResult{
			EntryID: f.EntryID, Seed: f.Seed, Status: f.Status,
			Usable: f.Usable, WallMs: f.WallMs,
		})
	}
	y := eval.ComputeYield(evalItems)
	doc := map[string]any{
		"schema_version": 1,
		"suite": map[string]any{
			"schema_version": 1, "suite_id": suiteID, "gen_form": "I2V_AMBIENCE",
			"op": "gen_i2v", "model": model, "seeds": []int{7}, "fps": 24,
			"entries": []map[string]any{{"id": "m1", "image_path": "x.png",
				"prompt": "p", "duration_sec": 6}},
			"assertion_pack": []any{},
			"budget":         map[string]any{"wall_seconds": 600, "gpu_seconds": 100},
		},
		"runner_mode":            runnerMode,
		"capability_profile_ref": testProfileDigest,
		"items":                  docItems,
		"yield":                  y,
	}
	// digest 与文件名一致性由 Build 校验——这里直接以内容摘要命名。
	raw, _ := json.Marshal(doc)
	name := digestOf(t, raw) + ".json"
	p := filepath.Join(dir, name)
	// 落盘带 digest 字段的完整 artifact（重新序列化保证 digest 字段在）。
	doc["digest"] = digestOf(t, mustMarshal(t, doc))
	raw2 := mustMarshal(t, doc)
	if err := os.WriteFile(p, append(raw2, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// digestOf 对 raw 计算与 report 包一致口径的稳定命名（测试只要求
// 文件名 == 内容中 digest 字段；这里用简单 sha256 hex）。
func digestOf(t *testing.T, raw []byte) string {
	t.Helper()
	sum := sha256Sum(raw)
	return sum
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// TestBuildAggregatesAndExtrapolates：正常聚合——出片率复算一致、
// 平均耗时、产能区间（单卡串行：86400s ÷ 单条耗时）、估算标注。
func TestBuildAggregatesAndExtrapolates(t *testing.T) {
	dir := t.TempDir()
	profile := writeProfile(t, dir)
	// form1 形态：timing.total_ms（gen+render+assert）
	a1 := writeArtifact(t, dir, "form1_fake_v100", "fake", "local", []itemFixture{
		{EntryID: "m1", Seed: 7, Status: "OK", Usable: true, WallMs: 3000, TotalMs: 10000},
		{EntryID: "m1", Seed: 42, Status: "ASSERT_FAIL", Usable: false, WallMs: 3000, TotalMs: 11000},
		{EntryID: "m2", Seed: 7, Status: "OK", Usable: true, WallMs: 3000, TotalMs: 20000},
	})
	rep, err := report.Build(report.Options{
		ArtifactPaths: []string{a1}, ProfilePath: profile, Date: testDate,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Suites) != 1 || rep.Suites[0].SuiteID != "form1_fake_v100" {
		t.Fatalf("套件视图错误: %+v", rep.Suites)
	}
	if got := rep.Suites[0].Yield.YieldRatio; got != 1.0 {
		t.Fatalf("复算出片率应 1.0（两个条目各至少 1 条可用）: %v", got)
	}
	if got := rep.Suites[0].AvgItemMs; got != 15000 { // (10000+20000)/2
		t.Fatalf("可用条目平均全链耗时应 15000ms: %v", got)
	}
	c := rep.Capacity
	if !c.Estimated {
		t.Fatal("产能外推必须显式标注估算（AC-10）")
	}
	if c.DailyLow != int(86400.0/20.0) || c.DailyHigh != int(86400.0/10.0) {
		t.Fatalf("日产能区间应 [4320, 8640]: %+v", c)
	}
	if c.A100Note == "" || c.CostNote == "" {
		t.Fatal("必须含 A100 迁移口径与成本口径说明（AC-10/DECISION-5/6）")
	}
	if c.Warning == "" {
		t.Fatal("fake 后端基样必须带警示（不代表真实产能）")
	}
	if rep.Digest == "" {
		t.Fatal("报告缺 digest")
	}
}

// TestBuildYieldMismatchFails：内嵌聚合与明细复算不一致 → 硬错误（AC-5）。
func TestBuildYieldMismatchFails(t *testing.T) {
	dir := t.TempDir()
	profile := writeProfile(t, dir)
	a1 := writeArtifact(t, dir, "s1", "fake", "local", []itemFixture{
		{EntryID: "m1", Seed: 7, Status: "OK", Usable: true, WallMs: 1000},
	})
	// 篡改内嵌 yield：明细 1/1 可用，内嵌声明 0。
	raw, _ := os.ReadFile(a1)
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	doc["yield"] = map[string]any{
		"entries_total": 1, "entries_with_usable": 0, "yield_ratio": 0.0,
		"items_total": 1, "items_usable": 0,
	}
	if err := os.WriteFile(a1, mustMarshal(t, doc), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := report.Build(report.Options{
		ArtifactPaths: []string{a1}, ProfilePath: profile, Date: testDate,
	}); err == nil {
		t.Fatal("出片率复算不一致必须失败")
	}
}

// TestBuildProfileRefMismatchFails：artifact 缺/异 profile 引用 → 失败。
func TestBuildProfileRefMismatchFails(t *testing.T) {
	dir := t.TempDir()
	profile := writeProfile(t, dir)
	a1 := writeArtifact(t, dir, "s1", "fake", "local", []itemFixture{
		{EntryID: "m1", Seed: 7, Status: "OK", Usable: true, WallMs: 1000},
	})
	// 改 profile 引用但保持 yield/digest 结构（digest 校验只对文件名）。
	raw, _ := os.ReadFile(a1)
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	doc["capability_profile_ref"] = "sha256:other"
	if err := os.WriteFile(a1, mustMarshal(t, doc), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := report.Build(report.Options{
		ArtifactPaths: []string{a1}, ProfilePath: profile, Date: testDate,
	}); err == nil {
		t.Fatal("profile 引用不一致必须失败")
	}
}

// TestBuildDeterministic：同输入双跑 digest 全等（报告仪器确定性）。
func TestBuildDeterministic(t *testing.T) {
	dir := t.TempDir()
	profile := writeProfile(t, dir)
	a1 := writeArtifact(t, dir, "s1", "fake", "local", []itemFixture{
		{EntryID: "m1", Seed: 7, Status: "OK", Usable: true, WallMs: 1000},
	})
	var digests []string
	for i := 0; i < 2; i++ {
		rep, err := report.Build(report.Options{
			ArtifactPaths: []string{a1}, ProfilePath: profile, Date: testDate,
		})
		if err != nil {
			t.Fatal(err)
		}
		digests = append(digests, rep.Digest)
	}
	if digests[0] != digests[1] {
		t.Fatalf("报告不 deterministic: %s ≠ %s", digests[0], digests[1])
	}
}

// TestBuildNoUsableItems：无可用条目 → 区间 0 + 警示，不 panic。
func TestBuildNoUsableItems(t *testing.T) {
	dir := t.TempDir()
	profile := writeProfile(t, dir)
	a1 := writeArtifact(t, dir, "s1", "fake", "local", []itemFixture{
		{EntryID: "m1", Seed: 7, Status: "GEN_FAIL", Usable: false, WallMs: 0},
	})
	rep, err := report.Build(report.Options{
		ArtifactPaths: []string{a1}, ProfilePath: profile, Date: testDate,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Capacity.DailyLow != 0 || rep.Capacity.DailyHigh != 0 {
		t.Fatalf("无可用条目区间应 0: %+v", rep.Capacity)
	}
	if rep.Capacity.Warning == "" {
		t.Fatal("无可用条目必须警示")
	}
}

// TestCapacityExcludesElectricityPrice：产能结构不出现电价/货币变量
// （DECISION-6——口径用序列化扫描执法；cost_note 是口径说明文字，不是变量）。
func TestCapacityExcludesElectricityPrice(t *testing.T) {
	raw, err := json.Marshal(report.Capacity{})
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, banned := range []string{"price", "yuan", "kwh", "电价", "electricity", "currency"} {
		if strings.Contains(s, banned) {
			t.Fatalf("Capacity 不得引入电价/货币变量（发现 %q）: %s", banned, s)
		}
	}
}

func sha256Sum(bs []byte) string {
	sum := sha256.Sum256(bs)
	return hex.EncodeToString(sum[:])
}
