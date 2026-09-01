// probes_test.go —— 卡 #118：QC 探针算子（ffprobe_field/resolution/
// blackdetect_ratio/aigc_metadata_present/aigc_overlay_present）。
// 媒体由 ffmpeg lavfi 现场合成（确定性），无外部 fixture 依赖。
package operators

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Cloudbird-Software/Shorts_Director/internal/jsoncmp"
	"github.com/Cloudbird-Software/Shorts_Director/internal/operator"
)

func requireProbeTools(t *testing.T) {
	t.Helper()
	for _, bin := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("缺 %s: %v", bin, err)
		}
	}
}

// makeVideo 合成一段测试媒体（确定性 lavfi）。
func makeProbeVideo(t *testing.T, dir, name, src string, extraArgs ...string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	args := append([]string{"-y", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", src, "-c:v", "libx264", "-preset", "veryfast",
		"-pix_fmt", "yuv420p"}, extraArgs...)
	args = append(args, path)
	if out, err := exec.Command("ffmpeg", args...).CombinedOutput(); err != nil {
		t.Fatalf("合成测试媒体: %v\n%s", err, out)
	}
	return path
}

// probeReq 构造探针请求（Inputs 逐项并入）。
func qcProbeReq(op, path string, inputs map[string]any, workdir string) operator.Request {
	if inputs == nil {
		inputs = map[string]any{}
	}
	inputs["media_path"] = path
	return operator.Request{
		ContractVersion: 1, Op: op, Inputs: inputs, Workdir: workdir,
	}
}

func TestFFProbeFieldAndResolution(t *testing.T) {
	requireProbeTools(t)
	dir := t.TempDir()
	path := makeProbeVideo(t, dir, "v.mp4", "testsrc2=s=540x960:d=2:r=25")

	ctx := context.Background()
	for _, c := range []struct {
		op   string
		h    Handler
		args map[string]any
		want float64
	}{
		{"ffprobe_field", (&FFProbeFieldOp{}).Handle, map[string]any{"field": "width"}, 540},
		{"ffprobe_field", (&FFProbeFieldOp{}).Handle, map[string]any{"field": "height"}, 960},
		{"ffprobe_field", (&FFProbeFieldOp{}).Handle, map[string]any{"field": "duration_sec"}, 2},
		{"resolution", (&ResolutionOp{}).Handle, map[string]any{"dim": "width"}, 540},
		{"resolution", (&ResolutionOp{}).Handle, map[string]any{"dim": "height"}, 960},
	} {
		resp, err := c.h(ctx, qcProbeReq(c.op, path, c.args, dir))
		if err != nil {
			t.Fatalf("op %s: %v", c.op, err)
		}
		if resp.Status != operator.StatusOK {
			t.Fatalf("op %s: status %s (%v)", c.op, resp.Status, resp.Error)
		}
		got, ok := jsoncmp.Float(resp.Outputs["value"])
		if !ok || got != c.want {
			t.Errorf("op %s: want %v, got %#v", c.op, c.want, resp.Outputs["value"])
		}
		if resp.Op != c.op {
			t.Errorf("op %s: response.op 漂移 %q", c.op, resp.Op)
		}
	}
}

// TestProbeBadInputs：缺参/未知字段/相对路径/非媒体一律 INPUT_ERROR，
// 不允许静默放行（A5 判定题化：坏输入即坏证据）。
func TestProbeBadInputs(t *testing.T) {
	requireProbeTools(t)
	dir := t.TempDir()
	path := makeProbeVideo(t, dir, "v.mp4", "testsrc2=s=540x960:d=1:r=25")
	ctx := context.Background()

	cases := []struct {
		name   string
		req    operator.Request
		handle Handler
	}{
		{"缺 media_path", operator.Request{ContractVersion: 1, Op: "ffprobe_field",
			Inputs: map[string]any{"field": "width"}, Workdir: dir}, (&FFProbeFieldOp{}).Handle},
		{"相对路径", operator.Request{ContractVersion: 1, Op: "ffprobe_field",
			Inputs: map[string]any{"media_path": "v.mp4", "field": "width"}, Workdir: dir}, (&FFProbeFieldOp{}).Handle},
		{"缺 field", qcProbeReq("ffprobe_field", path, nil, dir), (&FFProbeFieldOp{}).Handle},
		{"未知 field", qcProbeReq("ffprobe_field", path,
			map[string]any{"field": "nope"}, dir), (&FFProbeFieldOp{}).Handle},
		{"dim 非法", qcProbeReq("resolution", path,
			map[string]any{"dim": "depth"}, dir), (&ResolutionOp{}).Handle},
		{"文件不存在", qcProbeReq("resolution", filepath.Join(dir, "no.mp4"),
			map[string]any{"dim": "width"}, dir), (&ResolutionOp{}).Handle},
		{"overlay 缺参照", qcProbeReq("aigc_overlay_present", path,
			map[string]any{"x": 0.0, "y": 0.0, "w": 10.0, "h": 10.0}, dir), (&AIGCOverlayOp{}).Handle},
		{"overlay 区域非法", qcProbeReq("aigc_overlay_present", path,
			map[string]any{"w": 0.0, "h": 0.0, "ref_media_path": path}, dir), (&AIGCOverlayOp{}).Handle},
	}
	for _, c := range cases {
		resp, err := c.handle(ctx, c.req)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if resp.Status != operator.StatusInputError {
			t.Errorf("%s: 期望 INPUT_ERROR，得到 %s", c.name, resp.Status)
		}
		if resp.Error == nil || resp.Error.Message == "" {
			t.Errorf("%s: INPUT_ERROR 必须带可执行错误信息", c.name)
		}
	}
}

// TestBlackdetectRatio：全黑视频占比≈1、内容视频占比≈0，证据日志落盘。
func TestBlackdetectRatio(t *testing.T) {
	requireProbeTools(t)
	dir := t.TempDir()
	ctx := context.Background()
	op := &BlackdetectRatioOp{}

	black := makeProbeVideo(t, dir, "black.mp4", "color=c=black:s=540x960:d=2:r=25")
	resp, err := op.Handle(ctx, qcProbeReq("blackdetect_ratio", black, nil, dir))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != operator.StatusOK {
		t.Fatalf("黑视频: %s (%v)", resp.Status, resp.Error)
	}
	if r := resp.Outputs["value"].(float64); r < 0.9 {
		t.Fatalf("全黑视频黑帧占比应 ≥0.9，得到 %v", r)
	}
	if ev, _ := resp.Outputs["evidence_uri"].(string); ev == "" || !filepath.IsAbs(ev) {
		t.Fatalf("缺证据 URI: %+v", resp.Outputs)
	} else if _, err := os.Stat(ev); err != nil {
		t.Fatalf("证据未落盘: %v", err)
	}

	colorful := makeProbeVideo(t, dir, "live.mp4", "testsrc2=s=540x960:d=2:r=25")
	wd := filepath.Join(dir, "wd2")
	resp, err = op.Handle(ctx, qcProbeReq("blackdetect_ratio", colorful, nil, wd))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != operator.StatusOK {
		t.Fatalf("内容视频: %s (%v)", resp.Status, resp.Error)
	}
	if r := resp.Outputs["value"].(float64); r > 0.01 {
		t.Fatalf("内容视频黑帧占比应 ≈0，得到 %v", r)
	}
}

func TestParseBlackdetect(t *testing.T) {
	log := "[blackdetect @ 0x1] black_start:0 black_end:1.5 black_duration:1.5\n" +
		"[blackdetect @ 0x1] black_start:2 black_end:2.5 black_duration:0.5\n"
	if d := parseBlackdetect(log); d != 2.0 {
		t.Fatalf("黑段加总 = %v，期望 2.0", d)
	}
	if d := parseBlackdetect("无黑段"); d != 0 {
		t.Fatalf("无黑段应得 0，得到 %v", d)
	}
}

// TestAIGCMetadataProbe：容器元数据 AIGC 块齐备判真、缺失判假。
func TestAIGCMetadataProbe(t *testing.T) {
	requireProbeTools(t)
	dir := t.TempDir()
	ctx := context.Background()
	op := &AIGCMetadataOp{}

	// mp4 muxer 默认丢弃未知键——须 use_metadata_tags 才写进 format tags
	// （渲染器写 AIGC 隐式标识块同此路径，探针只负责读回判定）。
	labeled := makeProbeVideo(t, dir, "labeled.mp4", "color=c=red:s=540x960:d=1:r=25",
		"-movflags", "use_metadata_tags",
		"-metadata", `AIGC={"ContentProducer":"Shorts_Director","ProduceTime":"2026-09-01","Identifier":"f1-demo"}`)
	resp, err := op.Handle(ctx, qcProbeReq("aigc_metadata_present", labeled, nil, dir))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != operator.StatusOK || resp.Outputs["value"] != true {
		t.Fatalf("完整 AIGC 块应判真: %+v (%v)", resp.Outputs, resp.Error)
	}

	plain := makeProbeVideo(t, dir, "plain.mp4", "color=c=red:s=540x960:d=1:r=25")
	resp, err = op.Handle(ctx, qcProbeReq("aigc_metadata_present", plain, nil, dir))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != operator.StatusOK || resp.Outputs["value"] != false {
		t.Fatalf("无 AIGC 块应判假: %+v (%v)", resp.Outputs, resp.Error)
	}
}

// TestAIGCOverlayPresent：成品指定区域 vs 归一化源片段——
// 画过字的区域差异占比显著非零；纯重编码占比≈0。
func TestAIGCOverlayPresent(t *testing.T) {
	requireProbeTools(t)
	fontPath := "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"
	if _, err := os.Stat(fontPath); err != nil {
		t.Skipf("缺 DejaVu 字体: %v", err)
	}
	dir := t.TempDir()
	ctx := context.Background()
	op := &AIGCOverlayOp{}

	ref := makeProbeVideo(t, dir, "ref.mp4", "color=c=0x3A7D44:s=540x960:d=2:r=25")
	// 成品：源 + 区域内 drawtext（模拟信息层叠加）
	final := filepath.Join(dir, "final.mp4")
	out, err := exec.Command("ffmpeg", "-y", "-hide_banner", "-loglevel", "error",
		"-i", ref, "-vf",
		"drawtext=fontfile="+fontPath+":text='AI generated demo':x=120:y=400:fontsize=28:fontcolor=white",
		"-c:v", "libx264", "-preset", "veryfast", "-pix_fmt", "yuv420p", final).CombinedOutput()
	if err != nil {
		t.Fatalf("合成叠加成品: %v\n%s", err, out)
	}
	region := map[string]any{"x": 100.0, "y": 380.0, "w": 340.0, "h": 80.0, "ref_media_path": ref}

	resp, err := op.Handle(ctx, qcProbeReq("aigc_overlay_present", final, region, filepath.Join(dir, "wd")))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != operator.StatusOK {
		t.Fatalf("叠加判定: %s (%v)", resp.Status, resp.Error)
	}
	if r := resp.Outputs["value"].(float64); r < 0.005 {
		t.Fatalf("画过字的区域差异占比 %v 应 ≥0.005", r)
	}

	// 对照：纯重编码（无叠加）→ 占比≈0
	plain := filepath.Join(dir, "plain.mp4")
	if out, err := exec.Command("ffmpeg", "-y", "-hide_banner", "-loglevel", "error",
		"-i", ref, "-c:v", "libx264", "-preset", "veryfast", "-pix_fmt", "yuv420p",
		plain).CombinedOutput(); err != nil {
		t.Fatalf("重编码: %v\n%s", err, out)
	}
	resp, err = op.Handle(ctx, qcProbeReq("aigc_overlay_present", plain, region, filepath.Join(dir, "wd2")))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != operator.StatusOK {
		t.Fatalf("对照判定: %s (%v)", resp.Status, resp.Error)
	}
	if r := resp.Outputs["value"].(float64); r >= 0.005 {
		t.Fatalf("纯重编码差异占比 %v 应 <0.005（容差内不计数）", r)
	}
}

// TestHandlerRunnerAdapter：进程内适配——补 Op、拒绝非法请求。
func TestHandlerRunnerAdapter(t *testing.T) {
	requireProbeTools(t)
	dir := t.TempDir()
	path := makeProbeVideo(t, dir, "v.mp4", "testsrc2=s=540x960:d=1:r=25")

	r := HandlerRunner{Op: "resolution", H: (&ResolutionOp{}).Handle}
	resp, err := r.Run(context.Background(), qcProbeReq("resolution", path, map[string]any{"dim": "width"}, dir))
	if err != nil {
		t.Fatal(err)
	}
	if w, ok := jsoncmp.Float(resp.Outputs["value"]); resp.Op != "resolution" || !ok || w != 540 {
		t.Fatalf("HandlerRunner 适配结果异常: %+v", resp)
	}
	// 缺 workdir → 请求校验失败（与 Serve 同前置）
	if _, err := r.Run(context.Background(), operator.Request{
		ContractVersion: 1, Op: "resolution",
		Inputs: map[string]any{"media_path": path, "dim": "width"},
	}); err == nil {
		t.Fatal("非法请求应被拒绝")
	}
}
