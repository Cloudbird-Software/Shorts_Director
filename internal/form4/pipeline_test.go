// pipeline_test.go —— 卡 #120（IR-0007 AC-7 / BEH-6，E5 切片-形态4）：
// fake 后端端到端——mock 商家（人像照 + 口播文案 + 三要素脚本）→
// gen_tts → gen_lipsync → transcribe 三要素 → 渲染（口播音轨保留）→
// 形态4 断言包 → run artifact。负例：三要素缺一即不可用（判定题会咬人）。
package form4_test

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
	"strconv"
	"testing"

	"github.com/Cloudbird-Software/Shorts_Director/internal/compiler"
	"github.com/Cloudbird-Software/Shorts_Director/internal/eval"
	"github.com/Cloudbird-Software/Shorts_Director/internal/form1"
	"github.com/Cloudbird-Software/Shorts_Director/internal/form4"
	"github.com/Cloudbird-Software/Shorts_Director/internal/operator"
	"github.com/Cloudbird-Software/Shorts_Director/internal/operators"
	"github.com/Cloudbird-Software/Shorts_Director/internal/qc"
)

const dejavuFont = "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"

func requirePipelineTools(t *testing.T) {
	t.Helper()
	for _, bin := range []string{"ffmpeg", "ffprobe", "python3"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("缺 %s: %v", bin, err)
		}
	}
	if _, err := os.Stat(dejavuFont); err != nil {
		t.Skipf("缺 DejaVu 字体: %v", err)
	}
}

// mixedRouter 把探针算子按 op 路由到进程内 Handler 或子进程 LocalRunner
// （lipsync 指标是 Python 算子，其余是 Go 探针）。
type mixedRouter map[string]operator.Runner

func (r mixedRouter) Run(ctx context.Context, req operator.Request) (operator.Response, error) {
	runner, ok := r[req.Op]
	if !ok {
		return operator.Response{}, fmt.Errorf("mixedRouter: 未注册探针 %q", req.Op)
	}
	return runner.Run(ctx, req)
}

func probeEngine(t *testing.T, dir string) *qc.Engine {
	t.Helper()
	syncnetBin := filepath.Join("..", "..", "operators", "syncnet_metric", "run.sh")
	router := mixedRouter{
		"ffprobe_field":         operators.HandlerRunner{Op: "ffprobe_field", H: (&operators.FFProbeFieldOp{}).Handle},
		"resolution":            operators.HandlerRunner{Op: "resolution", H: (&operators.ResolutionOp{}).Handle},
		"aigc_metadata_present": operators.HandlerRunner{Op: "aigc_metadata_present", H: (&operators.AIGCMetadataOp{}).Handle},
		"aigc_overlay_present":  operators.HandlerRunner{Op: "aigc_overlay_present", H: (&operators.AIGCOverlayOp{}).Handle},
		"lipsync_lse_c":         &operator.LocalRunner{Bin: syncnetBin},
		"lipsync_lse_d":         &operator.LocalRunner{Bin: syncnetBin},
	}
	tiers := map[string]qc.CostTier{
		"ffprobe_field":         qc.CostFree,
		"resolution":            qc.CostFree,
		"aigc_metadata_present": qc.CostFree,
		"aigc_overlay_present":  qc.CostLight,
		"lipsync_lse_c":         qc.CostHeavy,
		"lipsync_lse_d":         qc.CostHeavy,
	}
	e, err := qc.NewEngine(qc.Operators(router, filepath.Join(dir, "qc"), tiers)...)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// writeMerchant 落一个 ASCII mock 商家：信息表 + form4 口播脚本 + 合成人像。
func writeMerchant(t *testing.T, dir, prompt string, script form4.Script) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	m := form1.Merchant{
		SchemaVersion: 1, ID: script.MerchantID, Vertical: "beauty", Fictional: true,
		Info: form1.MerchantInfo{ShopName: "Yueyan Nail Studio",
			SignatureItem: "Gel Manicure", Price: "CNY 128",
			Address: "5 Bloom Street", Phone: "137-0000-9012"},
		AIGCDisclosure: "AI generated demo",
	}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "merchant.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	sraw, err := json.Marshal(script)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "form4.json"), sraw, 0o644); err != nil {
		t.Fatal(err)
	}
	// 合成人像：64×64 渐变 PNG（合成数据，无真实生物特征——INV-6）
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 4), G: uint8(y * 4), B: 160, A: 255})
		}
	}
	fh, err := os.Create(filepath.Join(dir, "seed_portrait.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	if err := png.Encode(fh, img); err != nil {
		t.Fatal(err)
	}
	_ = prompt
	return filepath.Join(dir, "seed_portrait.png")
}

func testFont(t *testing.T) compiler.Font {
	t.Helper()
	bs, err := os.ReadFile(dejavuFont)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(bs)
	return compiler.Font{
		Family: "DejaVu_Sans", Path: dejavuFont,
		Hash: "sha256:" + hex.EncodeToString(sum[:]),
	}
}

// runPipeline 装配并执行一次形态4 管线（fake 后端 + 真实渲染）。
func runPipeline(t *testing.T, suite *eval.Suite, merchants map[string]*form1.Merchant,
	scripts map[string]*form4.Script, workdir string) *form4.Artifact {
	t.Helper()
	ttsBin := filepath.Join("..", "..", "operators", "gen_tts", "run.sh")
	lipBin := filepath.Join("..", "..", "operators", "gen_lipsync", "run.sh")
	trBin := filepath.Join("..", "..", "operators", "transcribe", "run.sh")
	art, err := form4.Run(context.Background(), form4.Options{
		Suite: suite, Merchants: merchants, Scripts: scripts,
		TTS:        &operator.LocalRunner{Bin: ttsBin},
		Lipsync:    &operator.LocalRunner{Bin: lipBin},
		Transcribe: &operator.LocalRunner{Bin: trBin},
		Engine:     probeEngine(t, workdir),
		Font:       testFont(t), RunnerMode: "local",
		WorkdirRoot: workdir, Root: filepath.Dir(workdir),
		Date:          "2026-09-01",
		RendererExpect: compiler.RendererExpect{FFmpeg: "test", Remotion: "4.0.230", Node: "22.11.0"},
	})
	if err != nil {
		t.Fatalf("form4.Run 失败: %v", err)
	}
	return art
}

func testSuite(prompt string) *eval.Suite {
	return &eval.Suite{
		SchemaVersion: 1, SuiteID: "form4_test", GenForm: "DIGITAL_HUMAN",
		Op: "gen_lipsync", Model: "fake", Seeds: []int64{7}, FPS: 25,
		Params: map[string]any{"width": 540, "height": 960},
		Entries: []eval.Entry{{
			ID: "nail_test", ImagePath: "nail_test/seed_portrait.png",
			Prompt: prompt, DurationSec: 6,
		}},
		Budget: eval.Budget{WallSeconds: 600, GpuSeconds: 0},
	}
}

// TestForm4EndToEnd：fake 后端全链——口播三要素齐全 → OK/usable，
// 成品画幅 1080×1920、时长 ≤6s、携带口播音轨（VO 保留）与 AIGC 双轨。
func TestForm4EndToEnd(t *testing.T) {
	requirePipelineTools(t)
	root := t.TempDir()
	workdir := filepath.Join(root, "work")
	prompt := "Yueyan nails 128 call" // 21 runes → fake TTS 5.04s ≤ 6s
	writeMerchant(t, filepath.Join(root, "nail_test"), prompt, form4.Script{
		SchemaVersion: 1, MerchantID: "nail_test",
		Brand: "Yueyan", SellingPoint: "nails 128", CTA: "call",
	})
	merchants, err := form1.LoadMerchantsDir(root)
	if err != nil {
		t.Fatal(err)
	}
	scripts, err := form4.LoadScriptsDir(root)
	if err != nil {
		t.Fatal(err)
	}
	art := runPipeline(t, testSuite(prompt), merchants, scripts, workdir)

	if len(art.Items) != 1 {
		t.Fatalf("期望 1 条，得到 %d", len(art.Items))
	}
	it := art.Items[0]
	if it.Status != eval.ItemOK || !it.Usable {
		t.Fatalf("期望 OK/usable，得到 %s: %s\nassertions: %+v", it.Status, it.Error, it.Assertions)
	}
	if it.AudioHash == "" || it.TranscribedText != prompt {
		t.Fatalf("VO 锚/转写文本异常: audio=%q text=%q", it.AudioHash, it.TranscribedText)
	}
	for _, a := range it.Assertions {
		if !a.Pass {
			t.Errorf("断言 %s 未过: measured=%v expected=%v", a.AssertionID, a.Measured, a.Expected)
		}
	}
	if art.Yield.YieldRatio != 1.0 || art.Yield.ItemsUsable != 1 {
		t.Fatalf("出片率异常: %+v", art.Yield)
	}

	// 成品硬校验：画幅/时长/口播音轨存在（VO 真的进了容器）
	w, h, dur, hasAudio := probeMedia(t, it.VideoPath)
	if w != 1080 || h != 1920 {
		t.Fatalf("画幅异常: %d×%d", w, h)
	}
	if dur > 6.0 || dur < 4.0 {
		t.Fatalf("时长 %v 超出 [4,6]", dur)
	}
	if !hasAudio {
		t.Fatal("成品缺音频轨——口播未保留（RenderVO 失效）")
	}
}

// TestForm4TranscribeGateBites：三要素缺一（品牌名不在口播文案）→
// 转写断言失败、条目不可用——判定题必须会咬人，不是恒真装饰。
func TestForm4TranscribeGateBites(t *testing.T) {
	requirePipelineTools(t)
	root := t.TempDir()
	workdir := filepath.Join(root, "work")
	prompt := "Hi" // 1.0s——最短链路
	writeMerchant(t, filepath.Join(root, "nail_test"), prompt, form4.Script{
		SchemaVersion: 1, MerchantID: "nail_test",
		Brand: "Ghost Brand", SellingPoint: "nails 128", CTA: "call",
	})
	merchants, err := form1.LoadMerchantsDir(root)
	if err != nil {
		t.Fatal(err)
	}
	scripts, err := form4.LoadScriptsDir(root)
	if err != nil {
		t.Fatal(err)
	}
	art := runPipeline(t, testSuite(prompt), merchants, scripts, workdir)

	it := art.Items[0]
	if it.Status != eval.ItemAssertFail || it.Usable {
		t.Fatalf("三要素缺失应 ASSERT_FAIL/不可用，得到 %s usable=%v", it.Status, it.Usable)
	}
	byID := map[string]qc.Result{}
	for _, a := range it.Assertions {
		byID[a.AssertionID] = a
	}
	if r, ok := byID["L1.transcribe.brand"]; !ok || r.Pass {
		t.Fatalf("期望 L1.transcribe.brand 失败: %+v", r)
	}
	if art.Yield.ItemsUsable != 0 || art.Yield.YieldRatio != 0 {
		t.Fatalf("出片率应为 0: %+v", art.Yield)
	}
}

// probeMedia 用 ffprobe 提取画幅/时长/音频轨存在性（值断言的证据源）。
func probeMedia(t *testing.T, path string) (w, h int, dur float64, hasAudio bool) {
	t.Helper()
	out, err := exec.Command("ffprobe", "-v", "error",
		"-show_entries", "stream=codec_type,width,height",
		"-show_entries", "format=duration",
		"-of", "json", path).Output()
	if err != nil {
		t.Fatalf("ffprobe %s: %v", path, err)
	}
	var doc struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	dur, err = strconv.ParseFloat(doc.Format.Duration, 64)
	if err != nil {
		t.Fatalf("时长解析失败 %q: %v", doc.Format.Duration, err)
	}
	for _, s := range doc.Streams {
		switch s.CodecType {
		case "video":
			w, h = s.Width, s.Height
		case "audio":
			hasAudio = true
		}
	}
	return w, h, dur, hasAudio
}

// TestForm4LoadScriptValidation：三要素不齐拒收；目录名与 merchant_id 不符拒收。
func TestForm4LoadScriptValidation(t *testing.T) {
	dir := t.TempDir()
	full := `{"schema_version":1,"merchant_id":"m1","brand":"B","selling_point":"S","cta":"C"}`
	path := filepath.Join(dir, "form4.json")
	if err := os.WriteFile(path, []byte(full), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := form4.LoadScript(path); err != nil {
		t.Fatalf("合法脚本被拒: %v", err)
	}
	for name, doc := range map[string]string{
		"缺 cta": `{"schema_version":1,"merchant_id":"m1","brand":"B","selling_point":"S"}`,
		"缺品牌":  `{"schema_version":1,"merchant_id":"m1","selling_point":"S","cta":"C"}`,
		"版本不符": `{"schema_version":2,"merchant_id":"m1","brand":"B","selling_point":"S","cta":"C"}`,
	} {
		if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := form4.LoadScript(path); err == nil {
			t.Errorf("%s 应被拒收", name)
		}
	}
	// 目录批量加载：目录名与 merchant_id 不符拒收
	mdir := filepath.Join(dir, "m1")
	if err := os.MkdirAll(mdir, 0o755); err != nil {
		t.Fatal(err)
	}
	bad := `{"schema_version":1,"merchant_id":"other","brand":"B","selling_point":"S","cta":"C"}`
	if err := os.WriteFile(filepath.Join(mdir, "form4.json"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := form4.LoadScriptsDir(dir); err == nil {
		t.Error("目录名与 merchant_id 不符应被 LoadScriptsDir 拒收")
	}
}
