// transcribe_test.go —— 卡 #120（IR-0007 AC-7 / BEH-6）：转写算子
// golden 查表 + LocalRunner 真实执行（fake 后端透传 text_hint）。
package operator_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Cloudbird-Software/Shorts_Director/internal/operator"
)

// form4 精简口播文案（三要素：品牌/卖点/CTA——25 字内控制时长 ≤6s）。
const form4Hint = "悦颜美甲，进口gel美甲128元，预约137-0000-9012"

func transcribeFakeRequest(audioPath, workdir string) operator.Request {
	return operator.Request{
		ContractVersion: 1,
		Op:              "transcribe",
		Inputs: map[string]any{
			"audio_path": audioPath,
			"text_hint":  form4Hint,
		},
		Params:      map[string]any{"model": "fake"},
		Workdir:     workdir,
		Determinism: operator.Determinism{Seed: nil},
	}
}

// TestTranscribeGoldenFakeRunner：FakeRunner 按请求摘要命中 transcribe fixture。
func TestTranscribeGoldenFakeRunner(t *testing.T) {
	resp, err := (&operator.FakeRunner{Dir: committedGoldenRoot}).Run(
		context.Background(), transcribeFakeRequest(
			"out/eval/work/yueyan_beauty/seed-7/out_fake.wav", "/mnt/work/tr-f4"))
	if err != nil {
		t.Fatalf("transcribe golden fixture 未命中: %v", err)
	}
	if resp.Status != operator.StatusOK {
		t.Fatalf("期望 OK，得到 %s: %+v", resp.Status, resp.Error)
	}
	if text, _ := resp.Outputs["text"].(string); text != form4Hint {
		t.Fatalf("fake 转写应透传 text_hint: %q", text)
	}
}

// TestTranscribeLocalEndToEnd：run.sh + fake 后端真实执行——透传一致；
// 缺 text_hint 是 INPUT_ERROR（可重传修复，非算子故障）。
func TestTranscribeLocalEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 缺失")
	}
	bin := filepath.Join("..", "..", "operators", "transcribe", "run.sh")
	audioPath := filepath.Join(t.TempDir(), "voice.wav")
	if out, err := exec.Command("ffmpeg", "-y", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "sine=frequency=220:sample_rate=24000:duration=2",
		"-c:a", "pcm_s16le", audioPath).CombinedOutput(); err != nil {
		t.Fatalf("合成语音: %v\n%s", err, out)
	}
	resp, err := (&operator.LocalRunner{Bin: bin}).Run(context.Background(),
		transcribeFakeRequest(audioPath, t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != operator.StatusOK {
		t.Fatalf("期望 OK，得到 %s: %+v", resp.Status, resp.Error)
	}
	if text, _ := resp.Outputs["text"].(string); text != form4Hint {
		t.Fatalf("fake 转写应透传 text_hint: %q", text)
	}

	req := transcribeFakeRequest(audioPath, t.TempDir())
	delete(req.Inputs, "text_hint")
	resp, err = (&operator.LocalRunner{Bin: bin}).Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != operator.StatusInputError || resp.Error.Code != "missing_text_hint" {
		t.Fatalf("缺 text_hint 应 INPUT_ERROR/missing_text_hint: %+v", resp.Error)
	}
}
