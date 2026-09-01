// Package doctor 是环境探测（IR-0007 AC-1/BEH-1，E0 实验）：
// 探测 GPU 算力版本/显存/驱动/CUDA、ffmpeg、docker，并对候选生成模型
// 逐项给出 feasible|infeasible 判定与原因，产出内容寻址的 capability
// profile（IFACE-2）。判定是静态硬件门槛筛查——最终可行性由 E1 单发
// 冒烟裁决（DECISION-3），本包不预置模型排名。
package doctor

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Cloudbird-Software/Shorts_Director/internal/digest"
)

// SchemaVersion 是 capability profile 的版本锚点。
const SchemaVersion = "capability_profile/1"

// Runner 抽象外部命令执行（测试注入点；生产用 ExecRunner）。
type Runner interface {
	// Run 执行命令返回 stdout；命令不存在或执行失败返回错误。
	Run(ctx context.Context, name string, args ...string) (string, error)
}

// ExecRunner 是真实环境执行器。
type ExecRunner struct{}

// Run 用 exec.LookPath + CombinedOutput 执行（stderr 并入错误消息）。
func (ExecRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	return runCommand(ctx, name, args...)
}

// GPU 是 NVIDIA GPU 探测结果；Present=false 时其余字段为空，原因进 Notes。
type GPU struct {
	Present       bool     `json:"present"`
	Name          string   `json:"name,omitempty"`
	ComputeCap    string   `json:"compute_cap,omitempty"` // 如 "7.0"（sm_70）
	DriverVersion string   `json:"driver_version,omitempty"`
	CUDAVersion   string   `json:"cuda_version,omitempty"`
	MemoryMB      int      `json:"memory_total_mb,omitempty"`
	Notes         []string `json:"notes,omitempty"`
}

// Tool 是单个工具链的探测结果；缺失原因显式进 Note（不静默）。
type Tool struct {
	Present bool   `json:"present"`
	Version string `json:"version,omitempty"`
	Note    string `json:"note,omitempty"`
}

// Tools 是工具链探测结果。
type Tools struct {
	FFmpeg Tool `json:"ffmpeg"`
	Docker Tool `json:"docker"`
}

// Candidate 是候选模型的可行性判定（feasible + 原因）。
type Candidate struct {
	ID       string   `json:"id"`
	Kind     string   `json:"kind"` // i2v | tts | lipsync | vlm
	Feasible bool     `json:"feasible"`
	Reasons  []string `json:"reasons"`
}

// Profile 是一次探测的完整产物（IFACE-2：机器可解析 + 内容寻址）。
type Profile struct {
	SchemaVersion string      `json:"schema_version"`
	ProbedAt      string      `json:"probed_at"` // RFC 3339，调用方注入（可测）
	GPU           GPU         `json:"gpu"`
	Tools         Tools       `json:"tools"`
	Candidates    []Candidate `json:"candidates"`
	Digest        string      `json:"digest,omitempty"` // 覆盖除本字段外的全部内容
}

// ComputeDigest 返回 profile 的内容寻址摘要（RFC 8785 JCS + sha256）。
// Digest 字段本身不参与摘要（自引用），复算时先清空再算。
func (p Profile) ComputeDigest() (string, error) {
	p.Digest = ""
	return digest.ValueDigest(p)
}

// Collect 执行全部探测并组装 profile；now 由调用方注入（确定性可测）。
func Collect(ctx context.Context, r Runner, now time.Time) Profile {
	p := Profile{
		SchemaVersion: SchemaVersion,
		ProbedAt:      now.UTC().Format(time.RFC3339),
	}
	p.GPU = probeGPU(ctx, r)
	p.Tools = probeTools(ctx, r)
	p.Candidates = judgeCandidates(p.GPU, p.Tools)
	if d, err := p.ComputeDigest(); err == nil {
		p.Digest = d
	} else {
		// 摘要失败不静默：作为 note 附在 GPU.Notes（profile 仍产出，
		// 但缺 digest 的 profile 不得被报告引用——消费方校验）。
		p.GPU.Notes = append(p.GPU.Notes, "digest 计算失败: "+err.Error())
	}
	return p
}

// probeGPU 探测 NVIDIA GPU：nvidia-smi 查询表 + banner CUDA 版本。
func probeGPU(ctx context.Context, r Runner) GPU {
	var g GPU
	out, err := r.Run(ctx, "nvidia-smi",
		"--query-gpu=name,compute_cap,driver_version,memory.total",
		"--format=csv,noheader,nounits")
	if err != nil {
		g.Present = false
		g.Notes = append(g.Notes, fmt.Sprintf("nvidia-smi 不可用（%s）——显式 infeasible，不静默降级", errShort(err)))
		return g
	}
	g.Present = true
	name, cc, driver, memMB, err := parseNvidiaQuery(out)
	if err != nil {
		g.Notes = append(g.Notes, "nvidia-smi 查询输出解析失败: "+err.Error())
	}
	g.Name, g.ComputeCap, g.DriverVersion, g.MemoryMB = name, cc, driver, memMB
	// CUDA 版本从 banner 解析（查询表无此字段）。
	if banner, err := r.Run(ctx, "nvidia-smi"); err == nil {
		if v := parseCUDABanner(banner); v != "" {
			g.CUDAVersion = v
		}
	} else {
		g.Notes = append(g.Notes, "nvidia-smi banner 不可用，CUDA 版本未知")
	}
	return g
}

// probeTools 探测 ffmpeg 与 docker（版本指纹）。
func probeTools(ctx context.Context, r Runner) Tools {
	var t Tools
	if out, err := r.Run(ctx, "ffmpeg", "-version"); err != nil {
		t.FFmpeg = Tool{Present: false, Note: "ffmpeg 不可用: " + errShort(err)}
	} else {
		t.FFmpeg = Tool{Present: true, Version: parseFFmpegVersion(out)}
	}
	if out, err := r.Run(ctx, "docker", "--version"); err != nil {
		t.Docker = Tool{Present: false, Note: "docker 不可用: " + errShort(err)}
	} else {
		t.Docker = Tool{Present: true, Version: parseDockerVersion(out)}
	}
	return t
}

// errShort 截断错误消息（避免 stderr 噪声淹没 profile）。
func errShort(err error) string {
	s := strings.TrimSpace(err.Error())
	if i := strings.Index(s, "\n"); i >= 0 {
		s = s[:i]
	}
	if len(s) > 160 {
		s = s[:160] + "…"
	}
	return s
}
