// gen_i2v_test.go —— 卡 #114（IR-0007 AC-3 / E1）：golden 查表 +
// LocalRunner 经 run.sh 真实执行 Python 算子（fake 后端，无 GPU 依赖）。
package operator_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Cloudbird-Software/Shorts_Director/internal/operator"
)

func sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// genI2VFakeRequest 是 fixture 2865e2… 的对应请求（workdir 不参与摘要）。
func genI2VFakeRequest(t *testing.T, workdir string) operator.Request {
	t.Helper()
	seed := int64(7)
	return operator.Request{
		ContractVersion: 1,
		Op:              "gen_i2v",
		Inputs: map[string]any{
			"image_path":   "/mnt/assets/noodles_hero.jpg",
			"prompt":       "一碗热气腾腾的牛肉面特写，缓慢推近，蒸汽升腾",
			"duration_sec": float64(3),
			"fps":          16,
		},
		Params:      map[string]any{"model": "fake", "width": float64(576), "height": float64(1024)},
		Workdir:     workdir,
		Determinism: operator.Determinism{Seed: &seed},
	}
}

// TestGenI2VGoldenFakeRunner：FakeRunner 按请求摘要命中 gen_i2v golden
// fixture（算子作者的 fixtures 交付义务，Go 业务逻辑零 Python 依赖可测）。
func TestGenI2VGoldenFakeRunner(t *testing.T) {
	resp, err := (&operator.FakeRunner{Dir: committedGoldenRoot}).Run(
		context.Background(), genI2VFakeRequest(t, "/mnt/work/geni2v-e1"))
	if err != nil {
		t.Fatalf("gen_i2v golden fixture 未命中: %v", err)
	}
	if resp.Status != operator.StatusOK {
		t.Fatalf("期望 OK，得到 %s: %+v", resp.Status, resp.Error)
	}
	videoPath, _ := resp.Outputs["video_path"].(string)
	contentHash, _ := resp.Outputs["content_hash"].(string)
	if videoPath == "" || !strings.HasPrefix(contentHash, "sha256:") || len(contentHash) != len("sha256:")+64 {
		t.Fatalf("输出契约缺失: video_path=%q content_hash=%q", videoPath, contentHash)
	}
	if resp.Metrics.WallMs < 0 || resp.OperatorVersion == "" || len(resp.ModelVersions) == 0 {
		t.Fatalf("metrics/operator_version/model_versions 缺失: %+v", resp)
	}
}

// TestGenI2VLocalEndToEnd：run.sh + fake 后端真实执行——
// OK 响应、产物存在、content_hash 与文件一致；同 seed 重放哈希一致（AC-3）。
func TestGenI2VLocalEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 缺失")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg 缺失")
	}
	bin := filepath.Join("..", "..", "operators", "gen_i2v", "run.sh")
	if _, err := os.Stat(bin); err != nil {
		t.Fatalf("算子入口缺失: %v", err)
	}
	imagePath := filepath.Join(t.TempDir(), "seed.jpg")
	if err := os.WriteFile(imagePath, []byte("fake-seed-image"), 0o644); err != nil {
		t.Fatal(err)
	}

	runOnce := func(workdir string) operator.Response {
		t.Helper()
		req := genI2VFakeRequest(t, workdir)
		req.Inputs["image_path"] = imagePath
		req.Params = map[string]any{"model": "fake", "width": 320, "height": 240}
		resp, err := (&operator.LocalRunner{Bin: bin}).Run(context.Background(), req)
		if err != nil {
			t.Fatalf("LocalRunner 执行失败: %v", err)
		}
		if resp.Status != operator.StatusOK {
			t.Fatalf("期望 OK，得到 %s: %+v", resp.Status, resp.Error)
		}
		return resp
	}

	resp1 := runOnce(t.TempDir())
	videoPath, _ := resp1.Outputs["video_path"].(string)
	if videoPath == "" {
		t.Fatalf("缺 video_path: %+v", resp1.Outputs)
	}
	raw, err := os.ReadFile(videoPath)
	if err != nil {
		t.Fatalf("产物不可读 %s: %v", videoPath, err)
	}
	want := "sha256:" + sha256Hex(raw)
	if got, _ := resp1.Outputs["content_hash"].(string); got != want {
		t.Fatalf("content_hash %q 与产物文件摘要 %q 不符", got, want)
	}
	// AC-3 重放：同输入同 seed，不同 workdir，产物内容哈希一致
	resp2 := runOnce(t.TempDir())
	if resp1.Outputs["content_hash"] != resp2.Outputs["content_hash"] {
		t.Fatalf("重放 content_hash 漂移: %v ≠ %v",
			resp1.Outputs["content_hash"], resp2.Outputs["content_hash"])
	}
}

// TestGenI2VInputError：坏输入（相对路径）必须是 INPUT_ERROR + 可执行信息。
func TestGenI2VInputError(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 缺失")
	}
	bin := filepath.Join("..", "..", "operators", "gen_i2v", "run.sh")
	seed := int64(7)
	resp, err := (&operator.LocalRunner{Bin: bin}).Run(context.Background(), operator.Request{
		ContractVersion: 1, Op: "gen_i2v",
		Inputs:      map[string]any{"image_path": "relative.jpg", "prompt": "x", "duration_sec": 2, "fps": 16},
		Params:      map[string]any{"model": "fake"},
		Workdir:     t.TempDir(),
		Determinism: operator.Determinism{Seed: &seed},
	})
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	if resp.Status != operator.StatusInputError || resp.Error == nil ||
		!strings.Contains(resp.Error.Message, "绝对路径") {
		t.Fatalf("期望 INPUT_ERROR（绝对路径提示），得到 %+v", resp)
	}
}
