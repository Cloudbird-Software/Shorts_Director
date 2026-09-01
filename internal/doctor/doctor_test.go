// doctor_test.go —— 解析纯函数 fixture 单测 + fake Runner 的 profile 组装
// golden + 无 GPU 降级路径 + 摘要复算（IFACE-2）。CLI 端到端见 cmd 侧构建
// 测试（buildDoctorCLI）。
package doctor

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeRunner 按「命令名 → 输出或错误」注入（缺省返回 not found）。
type fakeRunner struct {
	outs  map[string]string
	errs  map[string]error
	calls []string
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	key := name + " " + strings.Join(args, " ")
	f.calls = append(f.calls, key)
	if err, ok := f.errs[name]; ok {
		return "", err
	}
	if out, ok := f.outs[key]; ok {
		return out, nil
	}
	if out, ok := f.outs[name]; ok {
		return out, nil
	}
	return "", errors.New("exec: " + name + ": not found")
}

var fixedNow = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

const v100Query = "NVIDIA V100-SXM2-32GB, 7.0, 535.104.05, 32768\n"

const v100Banner = `Tue Sep  1 12:00:00 2026
+-----------------------------------------------------------------------------+
| NVIDIA-SMI 535.104.05   Driver Version: 535.104.05   CUDA Version: 12.2     |
|-------------------------------+----------------------+----------------------+
`

func TestParseNvidiaQuery(t *testing.T) {
	name, cc, drv, mem, err := parseNvidiaQuery(v100Query)
	if err != nil {
		t.Fatalf("parseNvidiaQuery: %v", err)
	}
	if name != "NVIDIA V100-SXM2-32GB" || cc != "7.0" || drv != "535.104.05" || mem != 32768 {
		t.Fatalf("got %q %q %q %d", name, cc, drv, mem)
	}
	if _, _, _, _, err := parseNvidiaQuery(""); err == nil {
		t.Fatal("空输出应报错")
	}
	if _, _, _, _, err := parseNvidiaQuery("a, 7.0, 535, 100\nb, 8.0, 535, 100\n"); err == nil {
		t.Fatal("多 GPU 应报错（单卡前提）")
	}
	if _, _, _, _, err := parseNvidiaQuery("V100, 7.0, 535, notanumber\n"); err == nil {
		t.Fatal("显存非整数应报错")
	}
}

func TestParseCUDABanner(t *testing.T) {
	if v := parseCUDABanner(v100Banner); v != "12.2" {
		t.Fatalf("CUDA 版本解析 = %q, want 12.2", v)
	}
	if v := parseCUDABanner("no cuda here"); v != "" {
		t.Fatalf("无 CUDA 行应返回空串，得到 %q", v)
	}
}

func TestParseToolVersions(t *testing.T) {
	if v := parseFFmpegVersion("ffmpeg version 6.1.1-3ubuntu5 Copyright (c) 2000-2023\n"); v != "6.1.1-3ubuntu5" {
		t.Fatalf("ffmpeg 版本 = %q", v)
	}
	if v := parseDockerVersion("Docker version 24.0.7, build afdd53b\n"); v != "24.0.7" {
		t.Fatalf("docker 版本 = %q", v)
	}
	if v := parseFFmpegVersion("garbage"); v != "unknown" {
		t.Fatalf("未知输出应回退 unknown，得到 %q", v)
	}
}

func TestCollectV100Golden(t *testing.T) {
	r := &fakeRunner{outs: map[string]string{
		"nvidia-smi --query-gpu=name,compute_cap,driver_version,memory.total --format=csv,noheader,nounits": v100Query,
		"nvidia-smi":       v100Banner,
		"ffmpeg -version":  "ffmpeg version 6.1.1-3ubuntu5 Copyright (c) 2000-2023 the FFmpeg developers\n",
		"docker --version": "Docker version 24.0.7, build afdd53b\n",
	}}
	p := Collect(context.Background(), r, fixedNow)
	if !p.GPU.Present || p.GPU.Name != "NVIDIA V100-SXM2-32GB" || p.GPU.ComputeCap != "7.0" ||
		p.GPU.MemoryMB != 32768 || p.GPU.CUDAVersion != "12.2" || p.GPU.DriverVersion != "535.104.05" {
		t.Fatalf("GPU 探测结果异常: %+v", p.GPU)
	}
	if !p.Tools.FFmpeg.Present || p.Tools.FFmpeg.Version != "6.1.1-3ubuntu5" ||
		!p.Tools.Docker.Present || p.Tools.Docker.Version != "24.0.7" {
		t.Fatalf("工具探测结果异常: %+v", p.Tools)
	}
	// V100-32G：feasible 应含 wan2.1-1.3b/ltx/piper/wav2lip/cosyvoice2；
	// wan2.1-14b（24GB 门槛）也满足显存但门槛 24576 ≤ 32768 → feasible；
	// qwen2.5-vl-7b（16GB）满足。全员 feasible（V100 32G 富余）。
	for _, c := range p.Candidates {
		if !c.Feasible {
			t.Errorf("V100-32G 上 %s 判 infeasible: %v", c.ID, c.Reasons)
		}
	}
	// 摘要复算（IFACE-2）：清空 Digest 再算必须一致。
	want := p.Digest
	if d, err := p.ComputeDigest(); err != nil || d != want {
		t.Fatalf("摘要复算失败: %v (err=%v)", d, err)
	}
	goldenJSON(t, "profile_v100_golden.json", p)
}

func TestCollectNoGPUExplicitInfeasible(t *testing.T) {
	r := &fakeRunner{outs: map[string]string{
		"ffmpeg -version": "ffmpeg version 6.1.1 Copyright\n",
	}}
	p := Collect(context.Background(), r, fixedNow)
	if p.GPU.Present {
		t.Fatal("无 nvidia-smi 时 GPU.Present 必须 false")
	}
	if len(p.GPU.Notes) == 0 || !strings.Contains(p.GPU.Notes[0], "不静默降级") {
		t.Fatalf("缺 GPU 必须显式标注原因: %v", p.GPU.Notes)
	}
	if p.Tools.Docker.Present {
		t.Fatal("docker 缺失应 present=false + note")
	}
	if p.Tools.Docker.Note == "" {
		t.Fatal("docker 缺失原因必须非空（不静默）")
	}
	for _, c := range p.Candidates {
		if c.Kind == "tts" && c.ID == "piper-1.x" {
			if !c.Feasible {
				t.Errorf("CPU 可跑候选不应判 infeasible: %v", c.Reasons)
			}
			continue
		}
		if c.Feasible {
			t.Errorf("无 GPU 时 %s 不应 feasible", c.ID)
		}
		if len(c.Reasons) == 0 {
			t.Errorf("infeasible 判定必须携带原因: %s", c.ID)
		}
	}
	goldenJSON(t, "profile_no_gpu_golden.json", p)
}

func TestJudgeComputeCapBoundary(t *testing.T) {
	// compute cap 不足 + 显存不足 → 双原因。
	g := GPU{Present: true, ComputeCap: "6.1", MemoryMB: 4096}
	ok, reasons := judgeAgainstGPU(requirement{"x", "i2v", 8192, "7.0"}, g)
	if ok {
		t.Fatal("6.1/4GB 不应过 7.0/8GB 门槛")
	}
	if len(reasons) != 2 {
		t.Fatalf("应逐项给出两条原因，得到 %v", reasons)
	}
	less, err := computeCapLess("7.0", "7.0")
	if err != nil || less {
		t.Fatalf("7.0 < 7.0 应为 false")
	}
	if _, err := computeCapLess("x.y", "7.0"); err == nil {
		t.Fatal("非法 compute cap 应报错")
	}
}

// goldenJSON 与 golden 文件逐字段比对（MarshalIndent 确定性输出）。
func goldenJSON(t *testing.T, name string, p Profile) {
	t.Helper()
	raw, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("testdata", name)
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("golden 不可读（首次请落盘）: %v", err)
	}
	if string(want) != string(raw)+"\n" {
		t.Fatalf("profile 与 golden 不一致:\n--- got ---\n%s\n", raw)
	}
}
