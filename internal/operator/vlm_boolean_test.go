// vlm_boolean_test.go —— 卡 #121（IR-0007 AC-8 / BEH-7）：布尔评审算子
// golden 查表 + LocalRunner 真实执行（fake 后端 hint 透传 / 哈希负对照）。
package operator_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Cloudbird-Software/Shorts_Director/internal/operator"
)

const vlbQuestion = "该画面是否可直接用于营销视频成片（清晰、非全黑、无明显畸变）？"

func vlbRequest(mediaPath, workdir string, hint *bool) operator.Request {
	inputs := map[string]any{
		"media_path": mediaPath,
		"question":   vlbQuestion,
	}
	if hint != nil {
		inputs["answer_hint"] = *hint
	}
	return operator.Request{
		ContractVersion: 1,
		Op:              "vlm_boolean",
		Inputs:          inputs,
		Params:          map[string]any{"model": "fake"},
		Workdir:         workdir,
		Determinism:     operator.Determinism{Seed: nil},
	}
}

// TestVLMBooleanGoldenFakeRunner：FakeRunner 按请求摘要命中 vlm_boolean fixture。
func TestVLMBooleanGoldenFakeRunner(t *testing.T) {
	hint := true
	resp, err := (&operator.FakeRunner{Dir: committedGoldenRoot}).Run(
		context.Background(), vlbRequest(
			"/mnt/work/calib/media/sharp_001.png", "/mnt/work/calib/probe", &hint))
	if err != nil {
		t.Fatalf("vlm_boolean golden fixture 未命中: %v", err)
	}
	if resp.Status != operator.StatusOK {
		t.Fatalf("期望 OK，得到 %s: %+v", resp.Status, resp.Error)
	}
	if answer, _ := resp.Outputs["answer"].(bool); !answer {
		t.Fatalf("fake 评审应透传 answer_hint=true: %+v", resp.Outputs)
	}
	if ev, _ := resp.Outputs["evidence"].(string); ev == "" {
		t.Fatal("评审判定必须携带证据描述（evidence 非空）")
	}
}

// TestVLMBooleanLocalEndToEnd：run.sh + fake 后端真实执行——hint 透传、
// 无 hint 哈希负对照确定性（双跑一致）、坏输入四态 INPUT_ERROR。
func TestVLMBooleanLocalEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 缺失")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg 缺失")
	}
	bin := filepath.Join("..", "..", "operators", "vlm_boolean", "run.sh")
	media := filepath.Join(t.TempDir(), "frame.png")
	if out, err := exec.Command("ffmpeg", "-y", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "gradients=s=256x256:c0=0x3E2B1F:c1=0x0E1418:speed=0.001",
		"-frames:v", "1", media).CombinedOutput(); err != nil {
		t.Fatalf("合成画面: %v\n%s", err, out)
	}
	runner := &operator.LocalRunner{Bin: bin}

	// 1) hint 透传（true / false 两个方向）
	for _, want := range []bool{true, false} {
		hint := want
		resp, err := runner.Run(context.Background(),
			vlbRequest(media, t.TempDir(), &hint))
		if err != nil {
			t.Fatal(err)
		}
		if resp.Status != operator.StatusOK {
			t.Fatalf("期望 OK，得到 %s: %+v", resp.Status, resp.Error)
		}
		if answer, _ := resp.Outputs["answer"].(bool); answer != want {
			t.Fatalf("fake 评审应透传 answer_hint=%v: %+v", want, resp.Outputs)
		}
	}

	// 2) 无 hint：哈希负对照——同输入双跑 answer/evidence 逐字一致
	var first operator.Response
	for i := 0; i < 2; i++ {
		resp, err := runner.Run(context.Background(),
			vlbRequest(media, t.TempDir(), nil))
		if err != nil {
			t.Fatal(err)
		}
		if resp.Status != operator.StatusOK {
			t.Fatalf("期望 OK，得到 %s: %+v", resp.Status, resp.Error)
		}
		if i == 0 {
			first = resp
			continue
		}
		if a1, _ := first.Outputs["answer"].(bool); a1 != resp.Outputs["answer"].(bool) {
			t.Fatalf("哈希负对照不 deterministic: %v ≠ %v",
				first.Outputs["answer"], resp.Outputs["answer"])
		}
		if e1, _ := first.Outputs["evidence"].(string); e1 != resp.Outputs["evidence"].(string) {
			t.Fatalf("负对照 evidence 不 deterministic: %q ≠ %q",
				e1, resp.Outputs["evidence"])
		}
	}

	// 3) 坏输入：媒体缺失 / 缺 question / URL 禁止 / 非媒体扩展名
	txt := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(txt, []byte("not media"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		mediaPath string
		question  string
		code      string
	}{
		"媒体缺失":  {"/nonexistent/a.png", vlbQuestion, "media_missing"},
		"缺问题":   {media, "", "missing_question"},
		"URL禁止": {"https://example.com/a.png", vlbQuestion, "url_forbidden"},
		"非媒体":   {txt, vlbQuestion, "bad_media_type"},
	} {
		req := operator.Request{
			ContractVersion: 1, Op: "vlm_boolean",
			Inputs:      map[string]any{"media_path": tc.mediaPath, "question": tc.question},
			Params:      map[string]any{"model": "fake"},
			Workdir:     t.TempDir(),
			Determinism: operator.Determinism{Seed: nil},
		}
		resp, err := runner.Run(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.Status != operator.StatusInputError || resp.Error.Code != tc.code {
			t.Fatalf("%s 应 INPUT_ERROR/%s，得到 %s/%+v",
				name, tc.code, resp.Status, resp.Error)
		}
	}

	// 4) 真实后端缺 seed → INPUT_ERROR（AC-3 重放条款）
	req := operator.Request{
		ContractVersion: 1, Op: "vlm_boolean",
		Inputs:      map[string]any{"media_path": media, "question": vlbQuestion},
		Params:      map[string]any{"model": "qwen-vl"},
		Workdir:     t.TempDir(),
		Determinism: operator.Determinism{Seed: nil},
	}
	resp, err := runner.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != operator.StatusInputError || resp.Error.Code != "missing_seed" {
		t.Fatalf("真实后端缺 seed 应 INPUT_ERROR/missing_seed: %s/%+v",
			resp.Status, resp.Error)
	}

	// 5) 未注册后端（带 seed——排除 missing_seed 先触发）
	seed := int64(7)
	req.Determinism = operator.Determinism{Seed: &seed}
	req.Params = map[string]any{"model": "gpt5-vision"}
	resp, err = runner.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != operator.StatusInputError || resp.Error.Code != "unknown_model" {
		t.Fatalf("未注册后端应 INPUT_ERROR/unknown_model: %s/%+v", resp.Status, resp.Error)
	}
}
