// render_e4_test.go —— 卡 #118（IR-0007 E4/E5 渲染侧）：真实媒体解码、
// 信息层 drawtext（R-3 安全区 / R-4 字体哈希）、AIGC 元数据双轨、bitexact。
package renderer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Cloudbird-Software/Shorts_Director/internal/compiler"
	"github.com/Cloudbird-Software/Shorts_Director/internal/compliance"
	"github.com/Cloudbird-Software/Shorts_Director/internal/videoplan"
)

// realMediaPlan 基于 minimal plan 构造 GENERATED 主轨 + 信息层 overlays：
// 媒体用 ffmpeg lavfi 现场生成（确定性纯色 2s），hash 与 plan 钉死值一致。
func realMediaPlan(t *testing.T, dir string, withOverlays bool) (videoplan.Plan, compiler.MediaIndex, []compiler.Font) {
	t.Helper()
	requireFFmpeg(t)
	p := loadMinimalPlan(t)
	p.PlanID = "018f6c01-bbbb-7bbb-8bbb-0000000000e5"
	p.Canvas = videoplan.Canvas{W: 1080, H: 1920, FPS: 25,
		SafeArea: videoplan.SafeArea{Top: 120, Bottom: 120, Left: 60, Right: 60}}
	mediaPath := filepath.Join(dir, "gen.mp4")
	out, err := exec.Command("ffmpeg", "-y", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "color=c=0x3A7D44:s=1080x1920:d=2:r=25",
		"-c:v", "libx264", "-preset", "veryfast", "-pix_fmt", "yuv420p", mediaPath).CombinedOutput()
	if err != nil {
		t.Fatalf("生成测试媒体: %v\n%s", err, out)
	}
	bs, _ := os.ReadFile(mediaPath)
	sum := sha256.Sum256(bs)
	hash := "sha256:" + hex.EncodeToString(sum[:])

	// 主轨：单 GENERATED clip，50 帧（2s × 25fps）
	main := videoplan.Track{TrackID: "trk_gen_main", Kind: videoplan.TrackVideoMain,
		Clips: []videoplan.Clip{{
			ClipID: "clip_gen_1", SrcIn: 0, SrcOut: 50, TlStart: 0, TlEnd: 50,
			Source: videoplan.ClipSource{Kind: "GENERATED", Ref: "gen-0001", ContentHash: hash},
		}}}
	p.Tracks = []videoplan.Track{main}
	// minimal plan 的 caption（70 帧）超出本测试 50 帧时间线——清空并按需重建
	p.Copy.CaptionBlocks = nil
	if withOverlays {
		p.Overlays = []videoplan.Overlay{
			{
				OverlayID: "ov_shop", Intent: "BRAND", Component: "caption.plain",
				Props:      map[string]any{"text": "Noodle Shop - CNY 28", "font": "DejaVu_Sans", "size": float64(28)},
				StartFrame: 0, EndFrame: 50,
				LayoutBox: videoplan.LayoutBox{X: 80, Y: 160, W: 800, H: 48, Anchor: "TL"},
			},
			{
				OverlayID: "ov_aigc", Intent: "COMPLIANCE", Component: "aigc.disclosure",
				Props:      map[string]any{"text": "AI generated demo", "font": "DejaVu_Sans", "size": float64(18)},
				StartFrame: 0, EndFrame: 50,
				LayoutBox: videoplan.LayoutBox{X: 760, Y: 1750, W: 260, H: 40, Anchor: "TL"},
			},
		}
	} else {
		p.Overlays = nil
	}
	fontPath := "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"
	if _, err := os.Stat(fontPath); err != nil {
		t.Skipf("缺 DejaVu 字体: %v", err)
	}
	fbs, _ := os.ReadFile(fontPath)
	fsum := sha256.Sum256(fbs)
	fonts := []compiler.Font{{Family: "DejaVu_Sans", Path: fontPath,
		Hash: "sha256:" + hex.EncodeToString(fsum[:])}}
	idx := compiler.MediaIndex{"generated:gen-0001": {
		LocalPath: mediaPath, ContentHash: hash, FPS: 25}}
	return p, idx, fonts
}

func realReq(t *testing.T, dir, outPath string, withOverlays bool) *compiler.RenderRequest {
	t.Helper()
	p, idx, fonts := realMediaPlan(t, dir, withOverlays)
	req, err := compiler.Compile(p, idx, fonts,
		compiler.Output{Path: outPath, Codec: "h264", CRF: 28, Preset: "veryfast"},
		compiler.Modes{Deterministic: true},
		compiler.RendererExpect{FFmpeg: "7.1", Remotion: "4.0.230", Node: "22.11.0"})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	return req
}

// TestRenderRealMediaDecode：GENERATED 轨真实解码——渲出成功、mp4 容器、
// 帧画面来自源媒体（抽帧后帧 digest ≠ 占位模式 digest）。
func TestRenderRealMediaDecode(t *testing.T) {
	requireFFmpeg(t)
	dir := t.TempDir()
	out := filepath.Join(dir, "real.mp4")
	if err := Render(realReq(t, dir, out, false), nil); err != nil {
		t.Fatalf("Render: %v", err)
	}
	bs, _ := os.ReadFile(out)
	if len(bs) < 12 || !strings.Contains(string(bs[4:12]), "ftyp") {
		t.Fatalf("输出不是 mp4 容器")
	}
	// 与占位模式对照：帧 digest 必须不同（真实帧 ≠ 派生纯色帧）
	var real, ph string
	if err := Render(realReq(t, dir, filepath.Join(dir, "r2.mp4"), false), &real); err != nil {
		t.Fatal(err)
	}
	phReq := realReq(t, dir, filepath.Join(dir, "p.mp4"), false)
	phReq.Modes.PlaceholderMedia = true
	if err := Render(phReq, &ph); err != nil {
		t.Fatal(err)
	}
	if real == ph {
		t.Fatal("真实解码帧 digest 与占位模式相同——解码路径未生效")
	}
}

// TestRenderRealMediaBitexact：R-1——真实解码 + 信息层 + AIGC 元数据，
// 同输入双跑输出 digest 全等（L-03 蜕变锚点）。
func TestRenderRealMediaBitexact(t *testing.T) {
	requireFFmpeg(t)
	dir := t.TempDir()
	o1, o2 := filepath.Join(dir, "a.mp4"), filepath.Join(dir, "b.mp4")
	var d1, d2 string
	if err := Render(realReq(t, dir, o1, true), &d1); err != nil {
		t.Fatalf("第一次: %v", err)
	}
	if err := Render(realReq(t, dir, o2, true), &d2); err != nil {
		t.Fatalf("第二次: %v", err)
	}
	if d1 != d2 {
		t.Fatalf("R-1 违例：帧 digest 不等 %s / %s", d1, d2)
	}
	if digestOf(t, o1) != digestOf(t, o2) {
		t.Fatal("R-1 违例：输出 digest 不等")
	}
}

// TestAIGCImplicitMetadata：含 GENERATED clip → 容器元数据携带完整 AIGC
// 隐式标识块（compliance.HasImplicitLabel 判真；GB 45438 双轨的元数据轨）。
func TestAIGCImplicitMetadata(t *testing.T) {
	requireFFmpeg(t)
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("缺 ffprobe")
	}
	dir := t.TempDir()
	out := filepath.Join(dir, "meta.mp4")
	req := realReq(t, dir, out, false)
	if !compliance.DisclosureRequired(&req.Plan) {
		t.Fatal("含 GENERATED clip 的 plan 应触发 AIGC 标识义务")
	}
	if err := Render(req, nil); err != nil {
		t.Fatalf("Render: %v", err)
	}
	raw, err := exec.Command("ffprobe", "-v", "error", "-show_format",
		"-print_format", "json", out).CombinedOutput()
	if err != nil {
		t.Fatalf("ffprobe: %v\n%s", err, raw)
	}
	var probe struct {
		Format struct {
			Tags map[string]string `json:"tags"`
		} `json:"format"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		t.Fatal(err)
	}
	if !compliance.HasImplicitLabel(probe.Format.Tags) {
		t.Fatalf("容器元数据缺完整 AIGC 隐式标识: %v", probe.Format.Tags)
	}
	if probe.Format.Tags["AIGC_LABEL_VERSION"] != compliance.ImplicitLabelVersion {
		t.Fatalf("AIGC_LABEL_VERSION 漂移: %v", probe.Format.Tags)
	}
}

// TestSafeAreaRejection：R-3——overlay 布局盒越出安全区必须拒绝渲染。
func TestSafeAreaRejection(t *testing.T) {
	requireFFmpeg(t)
	dir := t.TempDir()
	req := realReq(t, dir, filepath.Join(dir, "x.mp4"), true)
	req.Plan.Overlays[0].LayoutBox = videoplan.LayoutBox{
		X: 0, Y: 0, W: 240, H: 36, Anchor: "TL"} // x=0 < safe.left=24
	err := Render(req, nil)
	if err == nil || !strings.Contains(err.Error(), "R-3") {
		t.Fatalf("越安全区 overlay 未被拒绝: %v", err)
	}
}

// TestFontHashMismatch：R-4——字体文件哈希与声明不符必须报错（无隐式换字体）。
func TestFontHashMismatch(t *testing.T) {
	requireFFmpeg(t)
	dir := t.TempDir()
	req := realReq(t, dir, filepath.Join(dir, "x.mp4"), true)
	req.Fonts[0].Hash = "sha256:" + strings.Repeat("0", 64)
	err := Render(req, nil)
	if err == nil || !strings.Contains(err.Error(), "哈希漂移") {
		t.Fatalf("字体哈希漂移未被拦截: %v", err)
	}
}

// TestMediaTooShort：R-4——源媒体短于时间线引用必须报错，不静默续帧。
func TestMediaTooShort(t *testing.T) {
	requireFFmpeg(t)
	dir := t.TempDir()
	req := realReq(t, dir, filepath.Join(dir, "x.mp4"), false)
	req.Plan.Tracks[0].Clips[0].TlEnd = 5000 // 时间线引用远超 2s 源
	err := Render(req, nil)
	if err == nil || !strings.Contains(err.Error(), "抽帧不足") {
		t.Fatalf("短素材未被拦截: %v", err)
	}
}

// TestOverlayMissingText：信息层 overlay 缺 text 必须报错（INV-5 显式化）。
func TestOverlayMissingText(t *testing.T) {
	requireFFmpeg(t)
	dir := t.TempDir()
	req := realReq(t, dir, filepath.Join(dir, "x.mp4"), true)
	delete(req.Plan.Overlays[0].Props, "text")
	err := Render(req, nil)
	if err == nil || !strings.Contains(err.Error(), "props.text") {
		t.Fatalf("缺文本 overlay 未被拦截: %v", err)
	}
}
