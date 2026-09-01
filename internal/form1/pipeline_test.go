// pipeline_test.go —— 卡 #118（IR-0007 AC-6 / BEH-5，E5）：fake 后端
// 3 mock 商家端到端——生成（lavfi 合成 mp4 的 stub 算子）→ plan 构造 →
// 渲染（真实解码 + 信息层 + AIGC 双轨）→ 形态1 断言包 → run artifact。
package form1_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Cloudbird-Software/Shorts_Director/internal/compiler"
	"github.com/Cloudbird-Software/Shorts_Director/internal/contracts"
	"github.com/Cloudbird-Software/Shorts_Director/internal/eval"
	"github.com/Cloudbird-Software/Shorts_Director/internal/form1"
	"github.com/Cloudbird-Software/Shorts_Director/internal/operator"
	"github.com/Cloudbird-Software/Shorts_Director/internal/operators"
	"github.com/Cloudbird-Software/Shorts_Director/internal/qc"
)

const dejavuFont = "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"

func requirePipelineTools(t *testing.T) {
	t.Helper()
	for _, bin := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("缺 %s: %v", bin, err)
		}
	}
	if _, err := os.Stat(dejavuFont); err != nil {
		t.Skipf("缺 DejaVu 字体: %v", err)
	}
}

// stubGen 是 fake 生成后端：ffmpeg lavfi 合成确定性 mp4（按 seed 变色），
// 返回真实 content_hash——与 FakeRunner 不同，产物要被真实渲染消费。
type stubGen struct{ t *testing.T }

func (g stubGen) Run(ctx context.Context, req operator.Request) (operator.Response, error) {
	seed := int64(0)
	if req.Determinism.Seed != nil {
		seed = *req.Determinism.Seed
	}
	if err := os.MkdirAll(req.Workdir, 0o755); err != nil {
		return operator.Response{}, err
	}
	path := filepath.Join(req.Workdir, fmt.Sprintf("gen_seed%d.mp4", seed))
	src := fmt.Sprintf("color=c=0x%06X:s=540x960:d=6:r=24", 0x224400+uint32(seed)&0xFFFF)
	if out, err := exec.Command("ffmpeg", "-y", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", src, "-c:v", "libx264", "-preset", "veryfast",
		"-pix_fmt", "yuv420p", path).CombinedOutput(); err != nil {
		return operator.Response{}, fmt.Errorf("stub 生成失败: %v: %s", err, out)
	}
	bs, err := os.ReadFile(path)
	if err != nil {
		return operator.Response{}, err
	}
	sum := sha256.Sum256(bs)
	return operator.Response{
		ContractVersion: contracts.ContractOperator, Op: req.Op, Status: operator.StatusOK,
		Outputs: map[string]any{
			"video_path": path, "content_hash": "sha256:" + hex.EncodeToString(sum[:]),
		},
		Metrics:         operator.Metrics{WallMs: 50, GpuSecond: 0, PeakMemMB: 0},
		OperatorVersion: "gen_i2v@stub", ModelVersions: map[string]string{"model": "fake"},
	}, nil
}

// probeRouter 把探针算子按 op 路由到进程内 Handler（测试无子进程依赖，
// 与 CLI 的 shorts-operator 子进程路径共用同一 Handler 实现）。
type probeRouter map[string]operators.Handler

func (r probeRouter) Run(ctx context.Context, req operator.Request) (operator.Response, error) {
	h, ok := r[req.Op]
	if !ok {
		return operator.Response{}, fmt.Errorf("probeRouter: 未注册探针 %q", req.Op)
	}
	return operators.HandlerRunner{Op: req.Op, H: h}.Run(ctx, req)
}

func probeEngine(t *testing.T, dir string) *qc.Engine {
	t.Helper()
	router := probeRouter{
		"ffprobe_field":        (&operators.FFProbeFieldOp{}).Handle,
		"resolution":           (&operators.ResolutionOp{}).Handle,
		"blackdetect_ratio":    (&operators.BlackdetectRatioOp{}).Handle,
		"aigc_metadata_present": (&operators.AIGCMetadataOp{}).Handle,
		"aigc_overlay_present": (&operators.AIGCOverlayOp{}).Handle,
	}
	tiers := map[string]qc.CostTier{
		"ffprobe_field":         qc.CostFree,
		"resolution":            qc.CostFree,
		"aigc_metadata_present": qc.CostFree,
		"blackdetect_ratio":     qc.CostLight,
		"aigc_overlay_present":  qc.CostLight,
	}
	e, err := qc.NewEngine(qc.Operators(router, filepath.Join(dir, "qc"), tiers)...)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// writeMerchants 落 3 个 ASCII mock 商家（INV-6 fictional）+ 种子图。
func writeMerchants(t *testing.T, dir string) map[string]*form1.Merchant {
	t.Helper()
	merchants := []form1.Merchant{
		{SchemaVersion: 1, ID: "noodle_test", Vertical: "food", Fictional: true,
			Info: form1.MerchantInfo{ShopName: "Lan Jie Noodle House",
				SignatureItem: "Beef Noodle Double Top", Price: "CNY 28",
				Address: "88 Xingfu Road, Floor 1", Phone: "138-0000-1234"},
			AIGCDisclosure: "AI generated demo"},
		{SchemaVersion: 1, ID: "coffee_test", Vertical: "cafe", Fictional: true,
			Info: form1.MerchantInfo{ShopName: "Valley Coffee",
				SignatureItem: "Pour Over V60", Price: "CNY 32",
				Address: "12 Peak Lane", Phone: "139-0000-5678"},
			AIGCDisclosure: "AI generated demo"},
		{SchemaVersion: 1, ID: "nail_test", Vertical: "beauty", Fictional: true,
			Info: form1.MerchantInfo{ShopName: "Yueyan Nail Studio",
				SignatureItem: "Gel Manicure", Price: "CNY 99",
				Address: "5 Bloom Street, Suite 2", Phone: "137-0000-9012"},
			AIGCDisclosure: "AI generated demo"},
	}
	out := map[string]*form1.Merchant{}
	for i := range merchants {
		d := filepath.Join(dir, merchants[i].ID)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(merchants[i])
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "merchant.json"), raw, 0o644); err != nil {
			t.Fatal(err)
		}
		m, err := form1.LoadMerchant(filepath.Join(d, "merchant.json"))
		if err != nil {
			t.Fatal(err)
		}
		out[m.ID] = m
		// 种子图：64×64 纯色 PNG（合成数据，无第三方版权）
		img := image.NewRGBA(image.Rect(0, 0, 64, 64))
		for y := 0; y < 64; y++ {
			for x := 0; x < 64; x++ {
				img.Set(x, y, color.RGBA{R: uint8(40 * i), G: 120, B: 180, A: 255})
			}
		}
		fh, err := os.Create(filepath.Join(d, "seed_hero.png"))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(fh, img); err != nil {
			t.Fatal(err)
		}
		fh.Close()
	}
	return out
}

func pipelineSuite(dir string) *eval.Suite {
	entries := []eval.Entry{}
	for _, id := range []string{"noodle_test", "coffee_test", "nail_test"} {
		entries = append(entries, eval.Entry{
			ID: id, ImagePath: filepath.Join(dir, id, "seed_hero.png"),
			Prompt: "ambience b-roll, slow push in", DurationSec: 6,
		})
	}
	return &eval.Suite{
		SchemaVersion: 1, SuiteID: "form1_e2e_test", GenForm: "I2V_AMBIENCE",
		Op: "gen_i2v", Model: "fake", Seeds: []int64{7}, FPS: 24,
		Entries: entries, Budget: eval.Budget{WallSeconds: 600, GpuSeconds: 100},
	}
}

func pipelineOptions(t *testing.T, dir string) form1.Options {
	t.Helper()
	fbs, err := os.ReadFile(dejavuFont)
	if err != nil {
		t.Fatal(err)
	}
	fsum := sha256.Sum256(fbs)
	return form1.Options{
		Suite: pipelineSuite(dir), Merchants: writeMerchants(t, dir),
		Gen: stubGen{t: t}, Engine: probeEngine(t, dir),
		Font: compiler.Font{Family: "DejaVu_Sans", Path: dejavuFont,
			Hash: "sha256:" + hex.EncodeToString(fsum[:])},
		RunnerMode: "fake", ProfileRef: "sha256:test-profile",
		WorkdirRoot: filepath.Join(dir, "work"), Root: dir, Date: "2026-09-01",
		RendererExpect: compiler.RendererExpect{
			FFmpeg: "7.1", Remotion: "4.0.230", Node: "22.11.0"},
	}
}

// TestForm1EndToEnd：3 商家 × 1 seed——全部 OK 可用、出片率 100%、
// 断言包全过（L0 探针 + L3 AIGC 双轨 + L1 信息层逐字一致）、
// 成品存在且 digest 落 artifact。
func TestForm1EndToEnd(t *testing.T) {
	requirePipelineTools(t)
	dir := t.TempDir()

	art, err := form1.Run(context.Background(), pipelineOptions(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	if len(art.Items) != 3 {
		t.Fatalf("应产 3 条成品，得到 %d", len(art.Items))
	}
	if art.Yield.YieldRatio != 1.0 || art.Yield.ItemsUsable != 3 || art.Yield.EntriesWithUsable != 3 {
		for _, it := range art.Items {
			t.Logf("条目 %s: status=%s err=%s", it.EntryID, it.Status, it.Error)
		}
		t.Fatalf("fake 后端应 100%% 出片: %+v", art.Yield)
	}
	assertIDs := map[string]bool{}
	for _, it := range art.Items {
		if it.Status != eval.ItemOK || !it.Usable {
			t.Fatalf("条目 %s seed %d 应 OK 可用: status=%s err=%s assertions=%+v",
				it.EntryID, it.Seed, it.Status, it.Error, it.Assertions)
		}
		if _, err := os.Stat(it.VideoPath); err != nil {
			t.Fatalf("成品缺失: %v", err)
		}
		if len(it.ContentHash) != len("sha256:")+64 || it.PlanDigest == "" {
			t.Fatalf("条目 %s 缺内容寻址字段: %+v", it.EntryID, it.ItemResult)
		}
		if it.MerchantRef == "" {
			t.Fatalf("条目 %s 缺信息表证据引用", it.EntryID)
		}
		if it.Timing.GenMs <= 0 || it.Timing.RenderMs <= 0 || it.Timing.AssertMs <= 0 {
			t.Fatalf("条目 %s 耗时分解缺失: %+v", it.EntryID, it.Timing)
		}
		for _, a := range it.Assertions {
			assertIDs[a.AssertionID] = true
			if !a.Pass {
				t.Errorf("条目 %s 断言 %s 失败: measured=%v expected=%v",
					it.EntryID, a.AssertionID, a.Measured, a.Expected)
			}
		}
	}
	// 断言包覆盖：L0 形态 + L3 AIGC 双轨 + L1 信息层六项
	for _, id := range []string{
		"L0.duration_in_form_range", "L0.canvas_width", "L0.canvas_height",
		"L0.black_ratio", "L3.aigc_disclosure_overlay", "L3.aigc_implicit_metadata",
		"L1.info_layer.shop_name", "L1.info_layer.signature_item", "L1.info_layer.price",
		"L1.info_layer.address", "L1.info_layer.phone", "L1.info_layer.aigc_disclosure",
	} {
		if !assertIDs[id] {
			t.Errorf("断言包缺 %s", id)
		}
	}
	if art.Digest == "" {
		t.Fatal("artifact 缺 digest")
	}
	if d, err := art.ComputeDigest(); err != nil || d != art.Digest {
		t.Fatalf("artifact digest 不可复算: %q vs %q (%v)", d, art.Digest, err)
	}
	// 落盘与回读
	path, err := art.Save(filepath.Join(dir, "out"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var back form1.Artifact
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.Yield.YieldRatio != 1.0 || len(back.Items) != 3 || back.Date != "2026-09-01" {
		t.Fatalf("artifact 往返语义漂移: %+v", back.Yield)
	}
}

// TestForm1Deterministic：同输入（同商家/同 seed/同日期/stub 确定性生成）
// 双跑 → plan digest 与成品 digest 全等（plan 构造确定性 + R-1 渲染确定性）。
func TestForm1Deterministic(t *testing.T) {
	requirePipelineTools(t)
	dir := t.TempDir()
	run := func() *form1.Artifact {
		art, err := form1.Run(context.Background(), pipelineOptions(t, dir))
		if err != nil {
			t.Fatal(err)
		}
		return art
	}
	a1, a2 := run(), run()
	for i := range a1.Items {
		if a1.Items[i].Status != eval.ItemOK || a2.Items[i].Status != eval.ItemOK {
			t.Fatalf("确定性双跑必须建立在成功条目上: %s %s / %s（否则断言空转）",
				a1.Items[i].EntryID, a1.Items[i].Status, a2.Items[i].Status)
		}
		if a1.Items[i].PlanDigest != a2.Items[i].PlanDigest {
			t.Errorf("条目 %s plan digest 漂移: %s vs %s",
				a1.Items[i].EntryID, a1.Items[i].PlanDigest, a2.Items[i].PlanDigest)
		}
		if a1.Items[i].ContentHash != a2.Items[i].ContentHash {
			t.Errorf("条目 %s 成品 digest 漂移: %s vs %s",
				a1.Items[i].EntryID, a1.Items[i].ContentHash, a2.Items[i].ContentHash)
		}
	}
}

// TestForm1RunValidation：Options 必填项缺失即拒绝（Date/Font/RendererExpect）。
func TestForm1RunValidation(t *testing.T) {
	requirePipelineTools(t)
	dir := t.TempDir()
	opts := pipelineOptions(t, dir)
	opts.Date = ""
	if _, err := form1.Run(context.Background(), opts); err == nil {
		t.Error("缺 Date 应被拒绝")
	}
	opts = pipelineOptions(t, dir)
	opts.Font = compiler.Font{}
	if _, err := form1.Run(context.Background(), opts); err == nil {
		t.Error("缺 Font 三元组应被拒绝")
	}
	opts = pipelineOptions(t, dir)
	opts.RendererExpect = compiler.RendererExpect{}
	if _, err := form1.Run(context.Background(), opts); err == nil {
		t.Error("缺 RendererExpect 应被拒绝")
	}
}
