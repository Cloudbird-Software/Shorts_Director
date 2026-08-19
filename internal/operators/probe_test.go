package operators

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Cloudbird-Software/Shorts_Director/internal/operator"
)

func TestParseFFProbe(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "ffprobe_sample.json"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := parseFFProbe(raw)
	if err != nil {
		t.Fatal(err)
	}
	// fixture 由 ffmpeg testsrc2=540x960:rate=25 + sine 生成
	if out["width"] != 540 || out["height"] != 960 {
		t.Fatalf("分辨率 %#v", out)
	}
	if out["fps"] != float64(25) {
		t.Fatalf("fps=%v", out["fps"])
	}
	if out["aspect_ratio"] != "9:16" {
		t.Fatalf("aspect_ratio=%v", out["aspect_ratio"])
	}
	if out["has_audio"] != true || out["acodec"] != "aac" {
		t.Fatalf("音频信息缺失: %#v", out)
	}
	if out["vcodec"] != "h264" {
		t.Fatalf("vcodec=%v", out["vcodec"])
	}
}

func TestParseFFProbeNoVideo(t *testing.T) {
	raw := []byte(`{"streams":[{"codec_type":"audio","codec_name":"mp3"}],"format":{"duration":"10"}}`)
	if _, err := parseFFProbe(raw); err == nil {
		t.Fatal("无视频流应报 INPUT_ERROR 语义")
	}
}

func TestParseRate(t *testing.T) {
	cases := []struct {
		in      string
		want    float64
		wantErr bool
	}{
		{"25/1", 25, false},
		{"30000/1001", 30000.0 / 1001, false},
		{"30", 30, false},
		{"0/0", 0, true},
		{"", 0, true},
		{"25/0", 0, true},
	}
	for _, c := range cases {
		got, err := parseRate(c.in)
		if c.wantErr {
			if err == nil {
				t.Fatalf("parseRate(%q) 应报错", c.in)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Fatalf("parseRate(%q)=%v err=%v，期望 %v", c.in, got, err, c.want)
		}
	}
}

func TestRatio(t *testing.T) {
	if r := ratio(540, 960); r != "9:16" {
		t.Fatalf("ratio(540,960)=%s", r)
	}
	if r := ratio(1080, 1920); r != "9:16" {
		t.Fatalf("ratio(1080,1920)=%s", r)
	}
	if r := ratio(0, 100); r != "unknown" {
		t.Fatalf("ratio(0,100)=%s", r)
	}
}

func probeReq(path string) operator.Request {
	return operator.Request{
		ContractVersion: 1, Op: "probe",
		Inputs:  map[string]any{"media_path": path},
		Workdir: "/tmp/probe-test",
	}
}

func TestProbeHandleBadInputs(t *testing.T) {
	p := &ProbeOp{}
	// 相对路径（疑似 URL/相对路径）拒绝
	resp, err := p.Handle(context.Background(), operator.Request{
		ContractVersion: 1, Op: "probe",
		Inputs: map[string]any{"media_path": "https://evil/x.mp4"}, Workdir: "/w",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != operator.StatusInputError {
		t.Fatalf("URL 输入应 INPUT_ERROR: %+v", resp)
	}
	// 文件不存在
	resp, err = p.Handle(context.Background(), probeReq("/no/such/file.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != operator.StatusInputError || resp.Error.Code != "BAD_MEDIA" {
		t.Fatalf("resp=%+v", resp)
	}
}

// 端到端：真实 ffprobe 读真实媒体（无 ffprobe 环境跳过）。
func TestProbeHandleEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("环境无 ffprobe")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("环境无 ffmpeg")
	}
	dir := t.TempDir()
	mp4 := filepath.Join(dir, "sample.mp4")
	gen := exec.Command("ffmpeg", "-y", "-v", "error",
		"-f", "lavfi", "-i", "testsrc2=duration=1:size=540x960:rate=25",
		"-c:v", "libx264", "-preset", "ultrafast", mp4)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("生成测试媒体失败: %v: %s", err, out)
	}
	p := &ProbeOp{}
	resp, err := p.Handle(context.Background(), probeReq(mp4))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != operator.StatusOK {
		t.Fatalf("resp=%+v", resp)
	}
	if resp.Outputs["width"] != 540 || resp.Outputs["fps"] != float64(25) {
		t.Fatalf("outputs=%#v", resp.Outputs)
	}
	if resp.ModelVersions["ffprobe"] == "" || resp.ModelVersions["ffprobe"] == "unknown" {
		t.Fatalf("model_versions 应回填 ffprobe 指纹: %v", resp.ModelVersions)
	}
	if resp.Metrics.WallMs < 0 {
		t.Fatal("wall_ms 不得为负")
	}
}
