// gen_tts_lipsync_test.go —— 卡 #119（IR-0007 AC-7 前置 / DECISION-4）：
// gen_tts / gen_lipsync / syncnet_metric（lipsync_lse_c·lse_d）三算子的
// golden 查表 + LocalRunner 经 run.sh 真实执行（fake 后端，无 GPU 依赖）。
package operator_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Cloudbird-Software/Shorts_Director/internal/operator"
)

// form4 文案（与 evals/suites/form4_digital_human.json 的 yueyan_beauty 条目一致）。
const form4Text = "悦颜美甲美睫工作室，进口光疗 gel 美甲单色 128 元，地址在春熙里 3 号楼 802 室，预约电话 137-0000-9012，本视频由 AI 生成仅作演示。"

func genTTSFakeRequest(t *testing.T, workdir string) operator.Request {
	t.Helper()
	seed := int64(7)
	return operator.Request{
		ContractVersion: 1,
		Op:              "gen_tts",
		Inputs:          map[string]any{"text": form4Text},
		Params:          map[string]any{"model": "fake"},
		Workdir:         workdir,
		Determinism:     operator.Determinism{Seed: &seed},
	}
}

func genLipsyncFakeRequest(t *testing.T, workdir string) operator.Request {
	t.Helper()
	seed := int64(7)
	return operator.Request{
		ContractVersion: 1,
		Op:              "gen_lipsync",
		Inputs: map[string]any{
			"image_path": "evals/merchants/yueyan_beauty/seed_portrait.png",
			"audio_path": "out/eval/work/yueyan_beauty/seed-7/out_fake.wav",
			"fps":        25,
		},
		Params:      map[string]any{"model": "fake", "width": float64(1080), "height": float64(1920)},
		Workdir:     workdir,
		Determinism: operator.Determinism{Seed: &seed},
	}
}

func lipsyncMetricFakeRequest(op, mediaPath, workdir string) operator.Request {
	return operator.Request{
		ContractVersion: 1,
		Op:              op,
		Inputs:          map[string]any{"media_path": mediaPath},
		Params:          map[string]any{"model": "fake"},
		Workdir:         workdir,
		Determinism:     operator.Determinism{Seed: nil},
	}
}

// TestGenTTSGoldenFakeRunner：FakeRunner 按请求摘要命中 gen_tts golden fixture。
func TestGenTTSGoldenFakeRunner(t *testing.T) {
	resp, err := (&operator.FakeRunner{Dir: committedGoldenRoot}).Run(
		context.Background(), genTTSFakeRequest(t, "/mnt/work/gentts-f4"))
	if err != nil {
		t.Fatalf("gen_tts golden fixture 未命中: %v", err)
	}
	if resp.Status != operator.StatusOK {
		t.Fatalf("期望 OK，得到 %s: %+v", resp.Status, resp.Error)
	}
	for _, field := range []string{"audio_path", "content_hash", "duration_sec"} {
		if _, ok := resp.Outputs[field]; !ok {
			t.Fatalf("输出契约缺失 %s: %+v", field, resp.Outputs)
		}
	}
	if resp.OperatorVersion == "" || len(resp.ModelVersions) == 0 {
		t.Fatalf("operator_version/model_versions 缺失: %+v", resp)
	}
}

// TestGenTTSLocalEndToEnd：run.sh + fake 后端真实执行——
// OK、产物存在、content_hash 与文件一致、duration_sec>0；同 seed 重放哈希一致（AC-3）。
func TestGenTTSLocalEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 缺失")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg 缺失")
	}
	bin := filepath.Join("..", "..", "operators", "gen_tts", "run.sh")
	if _, err := os.Stat(bin); err != nil {
		t.Fatalf("算子入口缺失: %v", err)
	}
	runOnce := func(workdir string) operator.Response {
		t.Helper()
		resp, err := (&operator.LocalRunner{Bin: bin}).Run(
			context.Background(), genTTSFakeRequest(t, workdir))
		if err != nil {
			t.Fatalf("LocalRunner 执行失败: %v", err)
		}
		if resp.Status != operator.StatusOK {
			t.Fatalf("期望 OK，得到 %s: %+v", resp.Status, resp.Error)
		}
		return resp
	}
	resp1 := runOnce(t.TempDir())
	audioPath, _ := resp1.Outputs["audio_path"].(string)
	raw, err := os.ReadFile(audioPath)
	if err != nil {
		t.Fatalf("产物不可读 %s: %v", audioPath, err)
	}
	if want := "sha256:" + sha256Hex(raw); resp1.Outputs["content_hash"] != want {
		t.Fatalf("content_hash %v 与产物文件摘要 %q 不符", resp1.Outputs["content_hash"], want)
	}
	if d, _ := resp1.Outputs["duration_sec"].(float64); d <= 0 {
		t.Fatalf("duration_sec 应 >0: %+v", resp1.Outputs)
	}
	resp2 := runOnce(t.TempDir())
	if resp1.Outputs["content_hash"] != resp2.Outputs["content_hash"] {
		t.Fatalf("同 seed 重放哈希漂移: %v ≠ %v",
			resp1.Outputs["content_hash"], resp2.Outputs["content_hash"])
	}
}

// TestGenLipsyncGoldenFakeRunner：FakeRunner 按请求摘要命中 gen_lipsync golden fixture。
func TestGenLipsyncGoldenFakeRunner(t *testing.T) {
	resp, err := (&operator.FakeRunner{Dir: committedGoldenRoot}).Run(
		context.Background(), genLipsyncFakeRequest(t, "/mnt/work/genls-f4"))
	if err != nil {
		t.Fatalf("gen_lipsync golden fixture 未命中: %v", err)
	}
	if resp.Status != operator.StatusOK {
		t.Fatalf("期望 OK，得到 %s: %+v", resp.Status, resp.Error)
	}
	if p, _ := resp.Outputs["video_path"].(string); p == "" {
		t.Fatalf("缺 video_path: %+v", resp.Outputs)
	}
	if h, _ := resp.Outputs["content_hash"].(string); len(h) != len("sha256:")+64 {
		t.Fatalf("content_hash 形态非法: %q", h)
	}
}

// TestGenLipsyncLocalEndToEnd：run.sh + fake 后端真实执行——静态图+音轨
// 合成 mp4；content_hash 与文件一致；同 seed 重放哈希一致（AC-3）。
func TestGenLipsyncLocalEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 缺失")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg 缺失")
	}
	bin := filepath.Join("..", "..", "operators", "gen_lipsync", "run.sh")
	dir := t.TempDir()
	imagePath := filepath.Join(dir, "portrait.png")
	if out, err := exec.Command("ffmpeg", "-y", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "color=c=0x3A7D44:s=540x960:d=1:r=25",
		"-frames:v", "1", imagePath).CombinedOutput(); err != nil {
		t.Fatalf("合成人像图: %v\n%s", err, out)
	}
	audioPath := filepath.Join(dir, "voice.wav")
	if out, err := exec.Command("ffmpeg", "-y", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "sine=frequency=220:sample_rate=24000:duration=2",
		"-c:a", "pcm_s16le", audioPath).CombinedOutput(); err != nil {
		t.Fatalf("合成语音: %v\n%s", err, out)
	}
	runOnce := func(workdir string) operator.Response {
		t.Helper()
		req := genLipsyncFakeRequest(t, workdir)
		req.Inputs["image_path"] = imagePath
		req.Inputs["audio_path"] = audioPath
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
	raw, err := os.ReadFile(videoPath)
	if err != nil {
		t.Fatalf("产物不可读 %s: %v", videoPath, err)
	}
	if want := "sha256:" + sha256Hex(raw); resp1.Outputs["content_hash"] != want {
		t.Fatalf("content_hash %v 与产物文件摘要 %q 不符", resp1.Outputs["content_hash"], want)
	}
	resp2 := runOnce(t.TempDir())
	if resp1.Outputs["content_hash"] != resp2.Outputs["content_hash"] {
		t.Fatalf("同 seed 重放哈希漂移: %v ≠ %v",
			resp1.Outputs["content_hash"], resp2.Outputs["content_hash"])
	}
}

// TestSyncnetMetricGoldenFakeRunner：lipsync_lse_c / lipsync_lse_d 两 op
// 均按请求摘要命中 syncnet 指标 golden fixture（value + evidence_uri 契约）。
func TestSyncnetMetricGoldenFakeRunner(t *testing.T) {
	for _, op := range []string{"lipsync_lse_c", "lipsync_lse_d"} {
		resp, err := (&operator.FakeRunner{Dir: committedGoldenRoot}).Run(
			context.Background(), lipsyncMetricFakeRequest(
				op, "out/eval/work/yueyan_beauty/seed-7/out_fake_lipsync.mp4",
				"/mnt/work/syncnet-f4"))
		if err != nil {
			t.Fatalf("%s golden fixture 未命中: %v", op, err)
		}
		if resp.Status != operator.StatusOK {
			t.Fatalf("%s 期望 OK，得到 %s: %+v", op, resp.Status, resp.Error)
		}
		if _, ok := resp.Outputs["value"].(float64); !ok {
			t.Fatalf("%s 缺数值 value: %+v", op, resp.Outputs)
		}
		if _, ok := resp.Outputs["evidence_uri"].(string); !ok {
			t.Fatalf("%s 缺 evidence_uri: %+v", op, resp.Outputs)
		}
	}
}

// TestSyncnetMetricLocalEndToEnd：fake 后端由成品 sha 派生确定指标——
// 同媒体文件重放值恒等（AC-3）；LSE-C ∈ [6.0,9.9]、LSE-D ∈ [5.5,8.4]。
func TestSyncnetMetricLocalEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 缺失")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg 缺失")
	}
	dir := t.TempDir()
	media := filepath.Join(dir, "clip.mp4")
	if out, err := exec.Command("ffmpeg", "-y", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "color=c=0x1A5276:s=540x960:d=2:r=25",
		"-c:v", "libx264", "-preset", "veryfast", "-pix_fmt", "yuv420p",
		media).CombinedOutput(); err != nil {
		t.Fatalf("合成媒体: %v\n%s", err, out)
	}
	for _, op := range []string{"lipsync_lse_c", "lipsync_lse_d"} {
		bin := filepath.Join("..", "..", "operators", "syncnet_metric", "run.sh")
		runOnce := func() float64 {
			t.Helper()
			resp, err := (&operator.LocalRunner{Bin: bin}).Run(context.Background(),
				lipsyncMetricFakeRequest(op, media, t.TempDir()))
			if err != nil {
				t.Fatalf("%s LocalRunner 执行失败: %v", op, err)
			}
			if resp.Status != operator.StatusOK {
				t.Fatalf("%s 期望 OK，得到 %s: %+v", op, resp.Status, resp.Error)
			}
			v, _ := resp.Outputs["value"].(float64)
			return v
		}
		v1, v2 := runOnce(), runOnce()
		if v1 != v2 {
			t.Fatalf("%s 同媒体重放值漂移: %v ≠ %v", op, v1, v2)
		}
		if op == "lipsync_lse_c" && (v1 < 6.0 || v1 > 9.9) {
			t.Fatalf("LSE-C %v 超出 fake 值域 [6.0,9.9]", v1)
		}
		if op == "lipsync_lse_d" && (v1 < 5.5 || v1 > 8.4) {
			t.Fatalf("LSE-D %v 超出 fake 值域 [5.5,8.4]", v1)
		}
	}
}

// TestGenLipsyncBadInputs：坏输入四态走 INPUT_ERROR 且错误可执行（C2 契约）。
func TestGenLipsyncBadInputs(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 缺失")
	}
	bin := filepath.Join("..", "..", "operators", "gen_lipsync", "run.sh")
	req := genLipsyncFakeRequest(t, t.TempDir())
	req.Inputs["image_path"] = "relative/portrait.png" // 相对路径
	resp, err := (&operator.LocalRunner{Bin: bin}).Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != operator.StatusInputError {
		t.Fatalf("相对路径应 INPUT_ERROR，得到 %s: %+v", resp.Status, resp.Error)
	}
}
