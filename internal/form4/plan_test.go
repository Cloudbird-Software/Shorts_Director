// plan_test.go —— plan 构造确定性与 VO 版本钉死（包内测试：planInput 未导出）。
package form4

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Cloudbird-Software/Shorts_Director/internal/compiler"
	"github.com/Cloudbird-Software/Shorts_Director/internal/renderer"
	"github.com/Cloudbird-Software/Shorts_Director/internal/videoplan"
)

func testInput() planInput {
	return planInput{
		Brand: "Yueyan", AIGCText: "AI generated demo",
		GenHash: "sha256:" + strings.Repeat("ab", 32),
		VOHash:  "sha256:" + strings.Repeat("cd", 32),
		Frames:  126, SuiteID: "form4_test", Seed: 7,
		Date: "2026-09-01", FontFamily: "DejaVu_Sans",
	}
}

// TestBuildPlanDeterministic：同输入恒同 plan（PlanID/InputDigest
// 内容派生）；VORef 钉死 gen_tts 产物版本；AUDIO_VO 轨存在。
func TestBuildPlanDeterministic(t *testing.T) {
	in := testInput()
	p1, err := BuildPlan(in)
	if err != nil {
		t.Fatal(err)
	}
	p2, err := BuildPlan(in)
	if err != nil {
		t.Fatal(err)
	}
	if p1.PlanID != p2.PlanID || p1.Provenance.InputDigest != p2.Provenance.InputDigest {
		t.Fatal("同输入必须恒同 plan（构造确定性失效）")
	}
	if p1.Audio.VORef == nil || p1.Audio.VORef.Hash != in.VOHash {
		t.Fatalf("VORef 未钉死 VO 产物版本: %+v", p1.Audio.VORef)
	}
	hasVO := false
	for _, tr := range p1.Tracks {
		if tr.Kind == videoplan.TrackAudioVO {
			hasVO = true
		}
	}
	if !hasVO {
		t.Fatal("缺 AUDIO_VO 轨（口播保留的 plan 侧标记）")
	}
	in.Seed++
	p3, err := BuildPlan(in)
	if err != nil {
		t.Fatal(err)
	}
	if p3.PlanID == p1.PlanID {
		t.Fatal("换 seed 应换 PlanID")
	}
}

// TestRenderVOVersionDrift：VO 产物 hash 与 plan 钉死值漂移 →
// 显式报错（R-4 无隐式回退——绝不静默混入错配配音）。
func TestRenderVOVersionDrift(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg 缺失")
	}
	dir := t.TempDir()
	wavA, wavB := filepath.Join(dir, "a.wav"), filepath.Join(dir, "b.wav")
	for i, w := range []string{wavA, wavB} {
		freq := 220 + i*110
		if out, err := exec.Command("ffmpeg", "-y", "-hide_banner", "-loglevel", "error",
			"-f", "lavfi", "-i", fmt.Sprintf("sine=frequency=%d:sample_rate=24000:duration=1", freq),
			"-c:a", "pcm_s16le", w).CombinedOutput(); err != nil {
			t.Fatalf("合成语音: %v\n%s", err, out)
		}
	}
	hashOf := func(p string) string {
		bs, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(bs)
		return "sha256:" + hex.EncodeToString(sum[:])
	}
	in := testInput()
	in.VOHash = hashOf(wavA) // plan 钉死 A
	plan, err := BuildPlan(in)
	if err != nil {
		t.Fatal(err)
	}
	fontPath := "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"
	bs, err := os.ReadFile(fontPath)
	if err != nil {
		t.Skipf("缺 DejaVu 字体: %v", err)
	}
	sum := sha256.Sum256(bs)
	req, err := compiler.Compile(plan,
		compiler.MediaIndex{"generated:lip-0001": {
			LocalPath: "/nonexistent/lip.mp4", ContentHash: in.GenHash, FPS: 25}},
		[]compiler.Font{{Family: "DejaVu_Sans", Path: fontPath,
			Hash: "sha256:" + hex.EncodeToString(sum[:])}},
		compiler.Output{Path: filepath.Join(dir, "out.mp4"), Codec: "h264", CRF: 28, Preset: "veryfast"},
		compiler.Modes{Deterministic: true, PlaceholderMedia: true},
		compiler.RendererExpect{FFmpeg: "test", Remotion: "4.0.230", Node: "22.11.0"})
	if err != nil {
		t.Fatal(err)
	}
	err = renderer.RenderVO(req, nil, wavB) // 宿主挂载的是 B
	if err == nil || !strings.Contains(err.Error(), "VO 版本漂移") {
		t.Fatalf("VO 漂移必须显式报错，得到: %v", err)
	}
}
