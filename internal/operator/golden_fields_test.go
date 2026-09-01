// golden_fields_test.go —— Freeze Gate G7：consumer 只访问 golden 已知字段。
//
// 机制（三层）：
//  1. 清单生成：testdata/golden/<op>/*.json → 各 op 的 outputs 字段清单；
//     同一 op 的多个 fixture 字段集合必须一致（fixture 删字段即失败）
//  2. 静态检查：AST 扫描 internal/ 非测试源码中 <x>.Outputs["字面量"] 访问，
//     每个访问包须以 ConsumedGoldenOps 声明消费关系，访问字段 ⊆ 清单
//  3. 防回归负例：删 fixture 字段 / 越界访问，检查必须失败
//
// 本文件跑在 make go-check（进 CI 依赖 #42 的人工接线）。
package operator_test

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Cloudbird-Software/Shorts_Director/internal/eval"
	"github.com/Cloudbird-Software/Shorts_Director/internal/form1"
	"github.com/Cloudbird-Software/Shorts_Director/internal/operator"
	"github.com/Cloudbird-Software/Shorts_Director/internal/qc"
)

const committedGoldenRoot = "../../testdata/golden"

// fixtureManifest 从 root 生成 op → 字段集合清单；不一致即 t.Error。
func fixtureManifest(t *testing.T, root string) map[string]map[string]bool {
	t.Helper()
	ops, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("golden 根目录不可读: %v", err)
	}
	manifest := map[string]map[string]bool{}
	for _, od := range ops {
		if !od.IsDir() {
			continue
		}
		files, _ := os.ReadDir(filepath.Join(root, od.Name()))
		var base []string
		for _, fd := range files {
			if !strings.HasSuffix(fd.Name(), ".json") {
				continue
			}
			raw, err := os.ReadFile(filepath.Join(root, od.Name(), fd.Name()))
			if err != nil {
				t.Fatal(err)
			}
			var resp struct {
				Outputs map[string]any `json:"outputs"`
			}
			if err := json.Unmarshal(raw, &resp); err != nil {
				t.Fatalf("%s/%s: %v", od.Name(), fd.Name(), err)
			}
			if len(resp.Outputs) == 0 {
				t.Errorf("%s/%s 缺 outputs——fixture 必须携带输出契约", od.Name(), fd.Name())
				continue
			}
			keys := make([]string, 0, len(resp.Outputs))
			for k := range resp.Outputs {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			if base == nil {
				base = keys
				continue
			}
			if strings.Join(keys, ",") != strings.Join(base, ",") {
				t.Errorf("op %s 的 fixture 字段集合不一致（%v ≠ %v）——上游输出契约漂移",
					od.Name(), keys, base)
			}
		}
		if base == nil {
			t.Errorf("op %s 目录无 fixture", od.Name())
			continue
		}
		set := map[string]bool{}
		for _, k := range base {
			set[k] = true
		}
		manifest[od.Name()] = set
	}
	return manifest
}

// outputsAccesses AST 扫描 root 下非测试源码，收集 文件 → Outputs 字面访问集合。
func outputsAccesses(t *testing.T, root string) map[string]map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	out := map[string]map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") ||
			strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			idx, ok := n.(*ast.IndexExpr)
			if !ok {
				return true
			}
			sel, ok := idx.X.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Outputs" {
				return true
			}
			lit, ok := idx.Index.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			field, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return true
			}
			rel = filepath.ToSlash(rel) // Windows 反斜杠归一
			if out[rel] == nil {
				out[rel] = map[string]bool{}
			}
			out[rel][field] = true
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// consumerDecl 是访问包 → 消费算子清单的注册表（G7 检查的声明面）。
// 键是相对 internal/ 的包目录（slash 形式）。新增 consumer 包时在此登记，
// 并在该包暴露 ConsumedGoldenOps 变量。
func consumerDecl() map[string][]string {
	return map[string][]string{
		"qc":    qc.ConsumedGoldenOps,
		"eval":  eval.ConsumedGoldenOps,
		"form1": form1.ConsumedGoldenOps,
	}
}

// validateAccesses 判定访问集合 ⊆ 清单（通用适配器按被消费各 op 字段集合的
// 交集判界——适配器对任意 op 走同一访问路径）。返回违规描述，空即通过。
func validateAccesses(manifest map[string]map[string]bool,
	accesses map[string]map[string]bool, decl map[string][]string) []string {
	var violations []string
	declDirs := map[string]bool{}
	for dir := range decl {
		declDirs[dir] = true
	}
	for file, fields := range accesses {
		dir := path.Dir(file)
		if dir == "operator" {
			continue // FakeRunner/DecodeResponse 契约本体，非消费边界
		}
		ops, ok := decl[dir]
		if !ok {
			violations = append(violations,
				file+": 存在 Outputs 字面访问但包未在 G7 consumerDecl 登记")
			continue
		}
		inter := map[string]bool{}
		for i, op := range ops {
			set, ok := manifest[op]
			if !ok {
				violations = append(violations,
					file+": 声明消费的 op "+op+" 在 testdata/golden 无 fixture")
				continue
			}
			if i == 0 {
				for k := range set {
					inter[k] = true
				}
			} else {
				for k := range inter {
					if !set[k] {
						delete(inter, k)
					}
				}
			}
		}
		for f := range fields {
			if !inter[f] {
				violations = append(violations,
					file+": 访问字段 "+f+" 不在所消费 op 的 golden 清单内（上游删改了输出？）")
			}
		}
	}
	sort.Strings(violations)
	return violations
}

// TestG7ManifestContent：提交的 golden 清单内容钉死（新增 op/字段须同步本测试）。
func TestG7ManifestContent(t *testing.T) {
	m := fixtureManifest(t, committedGoldenRoot)
	want := map[string][]string{
		"probe":             {"fps", "width"},
		"blackdetect_ratio": {"evidence_uri", "value"},
		"gen_i2v":           {"video_path", "content_hash"},
		"gen_tts":           {"audio_path", "content_hash", "duration_sec"},
		"gen_lipsync":       {"video_path", "content_hash"},
		"lipsync_lse_c":     {"evidence_uri", "value"},
		"lipsync_lse_d":     {"evidence_uri", "value"},
	}
	if len(m) != len(want) {
		t.Errorf("golden 清单规模 %d ≠ %d（新 op 落 fixture 须同步本测试）", len(m), len(want))
	}
	for op, fields := range want {
		set, ok := m[op]
		if !ok {
			t.Errorf("op %s 缺 fixture", op)
			continue
		}
		for _, f := range fields {
			if !set[f] {
				t.Errorf("op %s 清单缺字段 %s", op, f)
			}
		}
	}
}

// TestG7ConsumerAccessWithinGolden：静态检查主断言——越界访问 = 失败。
func TestG7ConsumerAccessWithinGolden(t *testing.T) {
	manifest := fixtureManifest(t, committedGoldenRoot)
	accesses := outputsAccesses(t, filepath.Join("..", "..", "internal"))
	if len(accesses) == 0 {
		t.Fatal("AST 扫描未发现任何 Outputs 访问——检查本身失效")
	}
	if v := validateAccesses(manifest, accesses, consumerDecl()); len(v) > 0 {
		t.Errorf("G7 越界/未声明访问：\n  %s", strings.Join(v, "\n  "))
	}
}

// TestG7NegativeFieldDeleted（负例）：fixture 删一个字段，检查必须失败。
func TestG7NegativeFieldDeleted(t *testing.T) {
	tmp := t.TempDir()
	src, err := filepath.Glob(filepath.Join(committedGoldenRoot, "blackdetect_ratio", "*.json"))
	if err != nil || len(src) != 1 {
		t.Fatalf("blackdetect_ratio fixture 异常: %v", err)
	}
	raw, _ := os.ReadFile(src[0])
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	outputs := doc["outputs"].(map[string]any)
	delete(outputs, "value") // 模拟上游算子删字段
	b, _ := json.Marshal(doc)
	dst := filepath.Join(tmp, "blackdetect_ratio", filepath.Base(src[0]))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, b, 0o644); err != nil {
		t.Fatal(err)
	}
	// 复制 probe 保持清单完整
	pSrc, _ := filepath.Glob(filepath.Join(committedGoldenRoot, "probe", "*.json"))
	if err := os.MkdirAll(filepath.Join(tmp, "probe"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, p := range pSrc {
		b, _ := os.ReadFile(p)
		if err := os.WriteFile(filepath.Join(tmp, "probe", filepath.Base(p)), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	manifest := fixtureManifest(t, tmp) // tmp 清单此时已缺 value
	accesses := map[string]map[string]bool{
		"qc/bridge.go": {"value": true, "evidence_uri": true},
	}
	v := validateAccesses(manifest, accesses, consumerDecl())
	if len(v) == 0 {
		t.Fatal("fixture 删除 value 字段后检查未失败（防回归失效）")
	}
	if !strings.Contains(strings.Join(v, "\n"), "value") {
		t.Errorf("违规应指向缺失字段 value，得到：\n  %s", strings.Join(v, "\n  "))
	}
}

// TestG7NegativeAccessOutsideManifest（负例）：越界字段访问必须被拒。
func TestG7NegativeAccessOutsideManifest(t *testing.T) {
	manifest := fixtureManifest(t, committedGoldenRoot)
	accesses := map[string]map[string]bool{
		"qc/bridge.go": {"renamed_field": true},
	}
	v := validateAccesses(manifest, accesses, consumerDecl())
	if len(v) == 0 || !strings.Contains(strings.Join(v, "\n"), "renamed_field") {
		t.Errorf("越界访问未被检查拒绝或违规未指向字段：%v", v)
	}
}

// TestCommittedFixturesLoadBearing：提交的 fixture 必须能被 FakeRunner 与
// 真实消费路径（qc 桥接器）命中——防 fixture 与代码脱节腐烂。
func TestCommittedFixturesLoadBearing(t *testing.T) {
	seed := int64(42)
	resp, err := (&operator.FakeRunner{Dir: committedGoldenRoot}).Run(
		context.Background(), operator.Request{
			ContractVersion: 1, Op: "probe",
			Inputs:      map[string]any{"media_path": "/mnt/work/abc.mp4"},
			Workdir:     "/mnt/work/job-1",
			Determinism: operator.Determinism{Seed: &seed},
		})
	if err != nil || resp.Outputs["width"] != float64(1080) {
		t.Fatalf("probe fixture 未命中: resp=%+v err=%v", resp, err)
	}

	adapter := &qc.RunnerProbeAdapter{
		Op: "blackdetect_ratio", Tier: qc.CostLight,
		Runner: &operator.FakeRunner{Dir: committedGoldenRoot},
	}
	m, err := adapter.Measure(context.Background(), &qc.Subject{
		MediaURI:  "s3://media/render.mp4",
		MediaHash: "sha256:" + strings.Repeat("a", 64),
	}, map[string]any{"threshold": 0.98})
	if err != nil || m.Value != float64(0.01) {
		t.Fatalf("blackdetect_ratio fixture 经真实消费路径未命中: m=%+v err=%v", m, err)
	}
}
