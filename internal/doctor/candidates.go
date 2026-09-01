// candidates.go —— 候选生成模型清单与静态可行性判定（E0）。
// 清单变更 = 评审焦点（新增/移除候选须在 PR 说明）；判定只基于硬件事实，
// 最终可行性由 E1 单发冒烟裁决（DECISION-3，不预置排名）。
package doctor

import (
	"fmt"
	"strconv"
	"strings"
)

// requirement 是候选模型的硬件门槛（必要条件，非充分条件）。
type requirement struct {
	id            string
	kind          string // i2v | tts | lipsync | vlm
	minVRAMMB     int    // 0 = 无显存门槛（CPU 可跑）
	minComputeCap string // "" = 不需要 GPU
}

// candidateReqs 是候选清单 v1（公开开源、可本地推理为准入，DECISION-3）。
// 显存门槛为社区报告的推理下限量级（fp16/量化），仅供筛查。
var candidateReqs = []requirement{
	// ---- 图生视频（形态1：I2V_AMBIENCE） ----
	{"wan2.1-i2v-1.3b", "i2v", 8192, "7.0"},
	{"wan2.1-i2v-14b", "i2v", 24576, "7.0"},
	{"ltx-video-2b", "i2v", 8192, "7.0"},
	{"cogvideox-5b-i2v", "i2v", 12288, "7.0"},
	{"svd-xt-1.1", "i2v", 10240, "7.0"},
	// ---- 语音合成（形态4：DIGITAL_HUMAN 前置） ----
	{"cosyvoice2-0.5b", "tts", 4096, "7.0"},
	{"piper-1.x", "tts", 0, ""}, // CPU 可跑
	// ---- 口型同步（形态4） ----
	{"wav2lip", "lipsync", 4096, "7.0"},
	{"musetalk", "lipsync", 8192, "7.0"},
	// ---- 布尔评审（vlm_boolean 探针宿主模型） ----
	{"qwen2.5-vl-7b-instruct", "vlm", 16384, "7.0"},
	{"internvl2-8b", "vlm", 16384, "7.0"},
}

// judgeCandidates 逐项判定候选可行性：只做静态硬件门槛筛查，
// feasible 携带门槛依据并显式标注「最终由 E1 冒烟裁决」。
func judgeCandidates(g GPU, t Tools) []Candidate {
	out := make([]Candidate, 0, len(candidateReqs))
	for _, r := range candidateReqs {
		c := Candidate{ID: r.id, Kind: r.kind}
		if r.minComputeCap == "" {
			// 无 GPU 门槛（CPU 可跑）。
			c.Feasible = true
			c.Reasons = []string{"无 GPU 硬件门槛（CPU 可运行）；最终可行性由 E1 单发冒烟裁决"}
			out = append(out, c)
			continue
		}
		if !g.Present {
			c.Reasons = []string{"无 NVIDIA GPU（nvidia-smi 不可用）"}
			out = append(out, c)
			continue
		}
		ok, reasons := judgeAgainstGPU(r, g)
		if ok {
			reasons = append(reasons, "静态硬件门槛满足；最终可行性由 E1 单发冒烟裁决（DECISION-3）")
		}
		c.Feasible = ok
		c.Reasons = reasons
		out = append(out, c)
	}
	return out
}

// judgeAgainstGPU 判定单条 GPU 门槛；不满足时逐项给出原因。
func judgeAgainstGPU(r requirement, g GPU) (bool, []string) {
	var reasons []string
	ok := true
	if g.ComputeCap == "" {
		ok = false
		reasons = append(reasons, "compute cap 未知（nvidia-smi 解析失败），无法判门槛")
	} else if less, err := computeCapLess(g.ComputeCap, r.minComputeCap); err != nil {
		ok = false
		reasons = append(reasons, "compute cap 解析失败: "+err.Error())
	} else if less {
		ok = false
		reasons = append(reasons, fmt.Sprintf("compute cap %s < 门槛 %s", g.ComputeCap, r.minComputeCap))
	} else {
		reasons = append(reasons, fmt.Sprintf("compute cap %s ≥ 门槛 %s", g.ComputeCap, r.minComputeCap))
	}
	if r.minVRAMMB > 0 {
		if g.MemoryMB <= 0 {
			ok = false
			reasons = append(reasons, "显存未知，无法判门槛")
		} else if g.MemoryMB < r.minVRAMMB {
			ok = false
			reasons = append(reasons, fmt.Sprintf("显存 %dMB < 门槛 %dMB", g.MemoryMB, r.minVRAMMB))
		} else {
			reasons = append(reasons, fmt.Sprintf("显存 %dMB ≥ 门槛 %dMB", g.MemoryMB, r.minVRAMMB))
		}
	}
	return ok, reasons
}

// computeCapLess 比较 compute cap 版本串（"7.0" < "8.0"）。
func computeCapLess(a, b string) (bool, error) {
	af, err1 := strconv.ParseFloat(strings.TrimSpace(a), 32)
	bf, err2 := strconv.ParseFloat(strings.TrimSpace(b), 32)
	if err1 != nil || err2 != nil {
		return false, fmt.Errorf("compute cap 非数值: %q vs %q", a, b)
	}
	return af < bf, nil
}
