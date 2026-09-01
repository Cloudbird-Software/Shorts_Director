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
	"github.com/Cloudbird-Software/Shorts_Director/internal/videoplan"
)

// fixture：手写 minimal_music_plan.json（Phase 0 DoD 的指定样本）。
func loadMinimalPlan(t *testing.T) videoplan.Plan {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "schema", "testdata", "video_plan", "valid", "minimal_music_plan.json"))
	if err != nil {
		t.Fatalf("读样本: %v", err)
	}
	var p videoplan.Plan
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("解析 plan: %v", err)
	}
	return p
}

// placeholderIndex 为 minimal plan 的两个 SHOT 引用提供占位解析
// （content_hash 与 plan 钉死值一致——R-4 版本校验通过；local_path 指向
// 测试内生成的占位文件。Phase 0 帧画面不读源文件）。
func placeholderIndex(t *testing.T, dir string) compiler.MediaIndex {
	t.Helper()
	mk := func(name, hash string) compiler.MediaEntry {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("placeholder:"+name), 0o644); err != nil {
			t.Fatalf("写占位素材: %v", err)
		}
		return compiler.MediaEntry{LocalPath: p, ContentHash: hash, FPS: 25}
	}
	return compiler.MediaIndex{
		"shot:018f6c01-aaaa-7aaa-8aaa-000000000002": mk(
			"shot-0002.placeholder", "sha256:1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a"),
		"shot:018f6c01-aaaa-7aaa-8aaa-000000000003": mk(
			"shot-0003.placeholder", "sha256:2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b"),
	}
}

// fixtureFonts 提供真实可核验的字体（drawtext 信息层需要字体文件 +
// 哈希一致；缺 DejaVu 的环境跳过合成段测试）。
func fixtureFonts(t *testing.T) []compiler.Font {
	t.Helper()
	path := "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"
	bs, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("缺 DejaVu 字体（drawtext 信息层测试需要）: %v", err)
	}
	sum := sha256.Sum256(bs)
	return []compiler.Font{{Family: "HarmonyOS_Sans_Bold", Path: path,
		Hash: "sha256:" + hex.EncodeToString(sum[:])}}
}

func compileReq(t *testing.T, idx compiler.MediaIndex, outPath string) *compiler.RenderRequest {
	t.Helper()
	req, err := compiler.Compile(loadMinimalPlan(t), idx, fixtureFonts(t),
		compiler.Output{Path: outPath, Codec: "h264", CRF: 28, Preset: "veryfast"},
		compiler.Modes{Deterministic: true, PlaceholderMedia: true},
		compiler.RendererExpect{FFmpeg: "7.1", Remotion: "4.0.230", Node: "22.11.0"})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	return req
}

func requireFFmpeg(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("环境无 ffmpeg（CI ubuntu runner 预装；本地缺二进制时跳过合成段）")
	}
}

func digestOf(t *testing.T, path string) string {
	t.Helper()
	bs, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(bs)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// TestRenderMinimalPlanPlayable：Phase 0 DoD——手写 plan.json 经完整链路
// 渲出可播放视频（mp4 容器头 + 体积合理性）。
func TestRenderMinimalPlanPlayable(t *testing.T) {
	requireFFmpeg(t)
	dir := t.TempDir()
	out := filepath.Join(dir, "demo.mp4")
	if err := Render(compileReq(t, placeholderIndex(t, dir), out), nil); err != nil {
		t.Fatalf("Render: %v", err)
	}
	bs, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("读输出: %v", err)
	}
	if len(bs) < 12 || !strings.Contains(string(bs[4:12]), "ftyp") {
		t.Fatalf("输出不是 mp4 容器（前 12 字节: %x）", bs[:12])
	}
	if len(bs) < 10_000 {
		t.Errorf("输出体积异常（%d 字节）——200 帧视频不应如此小", len(bs))
	}
}

// TestRenderFramesConservation：R-2 帧数守恒——落帧数 == plan.TotalFrames()。
func TestRenderFramesConservation(t *testing.T) {
	dir := t.TempDir()
	fdir := filepath.Join(dir, "frames")
	if err := os.MkdirAll(fdir, 0o755); err != nil {
		t.Fatal(err)
	}
	req := compileReq(t, placeholderIndex(t, dir), filepath.Join(dir, "unused.mp4"))
	n, err := RenderFrames(req, fdir, nil)
	if err != nil {
		t.Fatalf("RenderFrames: %v", err)
	}
	want := loadMinimalPlan(t).TotalFrames()
	if n != want {
		t.Fatalf("R-2 违例：落帧 %d != TotalFrames %d", n, want)
	}
	ents, err := os.ReadDir(fdir)
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != want {
		t.Fatalf("R-2 违例：帧文件数 %d != %d", len(ents), want)
	}
}

// TestRenderDeterministic：R-1 确定性——同输入双跑，帧 digest 与输出
// digest 全等（ffmpeg bitexact 参数剥除全部时变元数据）。
func TestRenderDeterministic(t *testing.T) {
	requireFFmpeg(t)
	dir := t.TempDir()
	var d1, d2 string
	out1 := filepath.Join(dir, "a.mp4")
	if err := Render(compileReq(t, placeholderIndex(t, dir), out1), &d1); err != nil {
		t.Fatalf("第一次渲染: %v", err)
	}
	out2 := filepath.Join(dir, "b.mp4")
	if err := Render(compileReq(t, placeholderIndex(t, dir), out2), &d2); err != nil {
		t.Fatalf("第二次渲染: %v", err)
	}
	if d1 != d2 {
		t.Errorf("R-1 违例：帧序列 digest 不等 %s / %s", d1, d2)
	}
	if digestOf(t, out1) != digestOf(t, out2) {
		t.Errorf("R-1 违例：输出 digest 不等 %s / %s", digestOf(t, out1), digestOf(t, out2))
	}
}

// TestVersionIncludesFontHashes：R-5——版本串包含排序后的字体哈希。
func TestVersionIncludesFontHashes(t *testing.T) {
	f1 := []compiler.Font{
		{Family: "A", Hash: "sha256:aaa"},
		{Family: "B", Hash: "sha256:bbb"},
	}
	f2 := []compiler.Font{
		{Family: "B", Hash: "sha256:bbb"},
		{Family: "A", Hash: "sha256:aaa"},
	}
	v := Version(compiler.RendererExpect{FFmpeg: "7.1"}, f1)
	if !strings.Contains(v, "sha256:aaa") || !strings.Contains(v, "sha256:bbb") {
		t.Fatalf("R-5 违例：版本串缺字体哈希: %s", v)
	}
	if Version(compiler.RendererExpect{FFmpeg: "7.1"}, f2) != v {
		t.Fatal("字体顺序不应影响版本串（排序后拼接）")
	}
}
