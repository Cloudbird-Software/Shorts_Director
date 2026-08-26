// probe.go 实现 probe 算子：媒体元信息（底层 ffprobe）。
// 这是 Ingest 流水线的第一步（upload → probe → dedup），
// 输出喂 content_hash 去重与 QC L0 断言（resolution/fps）。
package operators

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/Cloudbird-Software/Shorts_Director/internal/operator"
)

// ProbeOp 用 ffprobe 读媒体元信息。
type ProbeOp struct {
	FFProbeBin string // 默认 "ffprobe"
}

// ffprobeStream 是 ffprobe 输出中单条流记录的受控子集。
type ffprobeStream struct {
	CodecType    string `json:"codec_type"`
	CodecName    string `json:"codec_name"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	AvgFrameRate string `json:"avg_frame_rate"`
	Duration     string `json:"duration"`
	NBFrames     string `json:"nb_frames"`
}

// ffprobeJSON 是 ffprobe -show_format -show_streams 输出的受控子集。
type ffprobeJSON struct {
	Streams []ffprobeStream `json:"streams"`
	Format  struct {
		Duration string `json:"duration"`
	} `json:"format"`
}

// Handle 执行 probe：inputs.media_path（绝对路径，禁止 URL）。
func (p *ProbeOp) Handle(ctx context.Context, req operator.Request) (operator.Response, error) {
	bin := p.FFProbeBin
	if bin == "" {
		bin = "ffprobe"
	}
	start := time.Now()

	path, _ := req.Inputs["media_path"].(string)
	if path == "" {
		return inputErrorResponse(req.Op, "BAD_INPUTS", "inputs.media_path 必填（绝对路径，禁止 URL）"), nil
	}
	if !strings.HasPrefix(path, "/") {
		return inputErrorResponse(req.Op, "BAD_INPUTS", "inputs.media_path 必须是绝对路径: "+path), nil
	}
	if _, err := os.Stat(path); err != nil {
		return inputErrorResponse(req.Op, "BAD_MEDIA", "媒体文件不可读: "+err.Error()), nil
	}

	raw, err := exec.CommandContext(ctx, bin,
		"-v", "error", "-print_format", "json",
		"-show_format", "-show_streams", path).Output()
	if err != nil {
		return inputErrorResponse(req.Op, "BAD_MEDIA",
			fmt.Sprintf("ffprobe 无法读取该文件（损坏或非媒体）: %v", err)), nil
	}

	meta, err := parseFFProbe(raw)
	if err != nil {
		return inputErrorResponse(req.Op, "BAD_MEDIA", err.Error()), nil
	}

	return okResponse(req.Op, meta, time.Since(start), map[string]string{"ffprobe": ffprobeVersion(bin)}), nil
}

// parseFFProbe 解析 ffprobe 输出为 probe 算子的 outputs。
// 无视频流是 INPUT_ERROR（本系统只收视频素材）。
func parseFFProbe(raw []byte) (map[string]any, error) {
	var f ffprobeJSON
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("ffprobe 输出不可解析: %w", err)
	}
	var video *ffprobeStream
	hasAudio := false
	var acodec string
	for i := range f.Streams {
		s := &f.Streams[i]
		switch s.CodecType {
		case "video":
			if video == nil {
				video = s
			}
		case "audio":
			hasAudio = true
			if acodec == "" {
				acodec = s.CodecName
			}
		}
	}
	if video == nil {
		return nil, fmt.Errorf("无视频流（本系统只收视频素材）")
	}
	fps, err := parseRate(video.AvgFrameRate)
	if err != nil {
		return nil, fmt.Errorf("avg_frame_rate 不可解析: %w", err)
	}
	dur := video.Duration
	if dur == "" {
		dur = f.Format.Duration
	}
	durationSec, _ := strconv.ParseFloat(dur, 64)

	out := map[string]any{
		"width":        video.Width,
		"height":       video.Height,
		"fps":          fps,
		"duration_sec": durationSec,
		"vcodec":       video.CodecName,
		"has_audio":    hasAudio,
		"aspect_ratio": ratio(video.Width, video.Height),
	}
	if hasAudio {
		out["acodec"] = acodec
	}
	if n, err := strconv.Atoi(video.NBFrames); err == nil && n > 0 {
		out["nb_frames"] = n
	}
	return out, nil
}

// parseRate 解析 ffprobe 的分数帧率（"30000/1001" → 29.97）。
func parseRate(s string) (float64, error) {
	if s == "" || s == "0/0" {
		return 0, fmt.Errorf("空帧率")
	}
	parts := strings.SplitN(s, "/", 2)
	num, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, err
	}
	if len(parts) == 2 {
		den, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return 0, err
		}
		if den == 0 {
			return 0, fmt.Errorf("帧率分母为 0")
		}
		return num / den, nil
	}
	return num, nil
}

// ratio 辗转相除求宽高比（540×960 → "9:16"）。
func ratio(w, h int) string {
	if w <= 0 || h <= 0 {
		return "unknown"
	}
	a, b := w, h
	for b != 0 {
		a, b = b, a%b
	}
	return fmt.Sprintf("%d:%d", w/a, h/a)
}

// ffprobeVersion 取 ffprobe 版本指纹（provenance A2）。
func ffprobeVersion(bin string) string {
	out, err := exec.Command(bin, "-version").Output()
	if err != nil {
		return "unknown"
	}
	line := strings.SplitN(string(out), "\n", 2)[0]
	if fields := strings.Fields(line); len(fields) >= 3 {
		return fields[2]
	}
	return line
}
