// nvidia.go —— nvidia-smi 输出解析（纯函数，fixture 单测覆盖）。
package doctor

import (
	"fmt"
	"strconv"
	"strings"
)

// parseNvidiaQuery 解析 --format=csv,noheader,nounits 的单 GPU 查询行：
// "NVIDIA A100-SXM4-40GB, 8.0, 535.104.05, 40960"。
// （实验机单卡前提；多卡只取第一张并在返回错误中说明。）
func parseNvidiaQuery(out string) (name, computeCap, driver string, memMB int, err error) {
	lines := nonEmptyLines(out)
	if len(lines) == 0 {
		return "", "", "", 0, fmt.Errorf("nvidia-smi 查询输出为空")
	}
	if len(lines) > 1 {
		return "", "", "", 0, fmt.Errorf("多 GPU 输出 %d 行，本探测仅支持单卡（多卡场景取第一行需产品化）", len(lines))
	}
	// nounits 模式下分隔符为 ", "（两字符）。
	parts := strings.Split(lines[0], ",")
	if len(parts) != 4 {
		return "", "", "", 0, fmt.Errorf("查询行字段数 %d ≠ 4: %q", len(parts), lines[0])
	}
	name = strings.TrimSpace(parts[0])
	computeCap = strings.TrimSpace(parts[1])
	driver = strings.TrimSpace(parts[2])
	mem, convErr := strconv.Atoi(strings.TrimSpace(parts[3]))
	if convErr != nil {
		return "", "", "", 0, fmt.Errorf("显存字段 %q 非整数: %w", parts[3], convErr)
	}
	if name == "" || computeCap == "" || driver == "" {
		return "", "", "", 0, fmt.Errorf("查询行存在空字段: %q", lines[0])
	}
	return name, computeCap, driver, mem, nil
}

// parseCUDABanner 从 nvidia-smi banner 解析 CUDA 版本：
// 首行含 "CUDA Version: 12.2"。找不到返回空串（调用方记 note）。
func parseCUDABanner(out string) string {
	for _, line := range nonEmptyLines(out) {
		if i := strings.Index(line, "CUDA Version:"); i >= 0 {
			rest := strings.TrimSpace(line[i+len("CUDA Version:"):])
			if j := strings.IndexAny(rest, " \t|"); j >= 0 {
				rest = rest[:j]
			}
			if rest != "" {
				return rest
			}
		}
	}
	return ""
}

// parseFFmpegVersion 从 "ffmpeg version 6.1.1-3ubuntu5 Copyright..." 取版本串。
func parseFFmpegVersion(out string) string {
	for _, line := range nonEmptyLines(out) {
		if v, ok := strings.CutPrefix(line, "ffmpeg version "); ok {
			if j := strings.IndexAny(v, " \t"); j >= 0 {
				v = v[:j]
			}
			return v
		}
	}
	return "unknown"
}

// parseDockerVersion 从 "Docker version 24.0.7, build afdd53b" 取版本串。
func parseDockerVersion(out string) string {
	for _, line := range nonEmptyLines(out) {
		if v, ok := strings.CutPrefix(line, "Docker version "); ok {
			if j := strings.IndexAny(v, ", "); j >= 0 {
				v = v[:j]
			}
			return v
		}
	}
	return "unknown"
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}
