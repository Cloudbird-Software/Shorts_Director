// Package renderer 是 C3 渲染契约的 Phase 0 最小渲染路径（#43 DoD 缺口）。
//
// Phase 0 占位语义（Engineering_plan §7 Phase 0 DoD：「能用手写 plan.json
// 渲出一条可播放视频，哪怕素材是假的」）：
//   - 帧画面 = 确定性纯色：SHOT/GENERATED 取 content_hash 前 6 hex 派生色，
//     GRAPHIC/COLOR 取 ref 派生色；VIDEO_INSERT 轨以居中矩形叠加合成。
//   - 音频：库音乐/VO 在 Phase 0 不解码（resolved_media 不含音频，见
//     compiler 注释），有音频轨时以静音轨保证产出是合规 A/V 容器。
//   - 真实素材解码、字幕排版、alpha overlay 是 Phase 1 S6 三阶段职责。
//
// 契约遵守（R-1..R-5 中渲染侧可自证的部分）：
//   - R-1 确定性：纯 Go 帧生成无时钟/随机源；ffmpeg 以 bitexact 参数调用，
//     同输入同版本恒同字节（测试断言双跑 digest 全等）。
//   - R-2 帧数守恒：先落满 TotalFrames 张帧再合成，帧数不符即失败。
//   - R-5 版本含字体哈希：Version() 拼接 ffmpeg 版本 + 排序后的字体哈希。
//
// 依赖纪律（AGENTS.md 硬规则 3）：零新增 Go 依赖——视频编码经外部
// ffmpeg 二进制（os/exec），ubuntu-latest runner 预装。
package renderer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Cloudbird-Software/Shorts_Director/internal/compiler"
	"github.com/Cloudbird-Software/Shorts_Director/internal/contracts"
	"github.com/Cloudbird-Software/Shorts_Director/internal/videoplan"
)

// ErrFFmpegMissing 表示运行环境没有 ffmpeg 二进制（R-4：显式报错，不静默降级）。
var ErrFFmpegMissing = fmt.Errorf("renderer: 环境缺 ffmpeg 二进制（Phase 0 最小渲染的外部依赖）")

// Version 实现 R-5：渲染器版本 = ffmpeg 版本 + 字体包哈希清单。
// 字体哈希排序后拼接——同字体集不同顺序不产生两个"版本"。
func Version(expect compiler.RendererExpect, fonts []compiler.Font) string {
	hs := make([]string, 0, len(fonts))
	for _, f := range fonts {
		hs = append(hs, f.Hash)
	}
	sort.Strings(hs)
	return fmt.Sprintf("ffmpeg:%s|fonts:%s", expect.FFmpeg, strings.Join(hs, ","))
}

// hashColor 从内容哈希/ref 确定性派生一个不刺眼的不透明色。
// hash 前 3 字节 → RGB；亮度压到 [64,208] 区间（占位帧可长时间观看）。
func hashColor(seed string) color.RGBA {
	sum := sha256.Sum256([]byte("placeholder:" + seed))
	r, g, b := sum[0], sum[1], sum[2]
	return color.RGBA{R: 64 + r%144, G: 64 + g%144, B: 64 + b%144, A: 255}
}

// clipColorAt 返回第 frame 帧上某轨道的活跃 clip 派生色。
// 不活跃（该帧无 clip 覆盖）返回 ok=false。GRAPHIC/COLOR 用 ref 作种子，
// SHOT/GENERATED 用 plan 钉死的 content_hash 作种子（版本变更即变色）。
func clipColorAt(t videoplan.Track, frame int) (color.RGBA, bool) {
	for _, c := range t.Clips {
		if frame >= c.TlStart && frame < c.TlEnd {
			seed := c.Source.Ref
			if c.Source.Kind == "SHOT" || c.Source.Kind == "GENERATED" {
				seed = c.Source.ContentHash
			}
			return hashColor(seed), true
		}
	}
	return color.RGBA{}, false
}

// RenderFrames 把整个时间线渲成帧序列（纯 Go、确定性）。
// 返回写入的帧数；R-2 校验：必须等于 plan.TotalFrames()。
func RenderFrames(req *compiler.RenderRequest, dir string, h hash.Hash) (int, error) {
	if req.ContractVersion != contracts.ContractRender {
		return 0, fmt.Errorf("renderer: 契约版本 %d 不受支持（期望 %d）", req.ContractVersion, contracts.ContractRender)
	}
	total := req.Plan.TotalFrames()
	w, hpx := req.Plan.Canvas.W, req.Plan.Canvas.H

	var mainTrack, insertTrack *videoplan.Track
	for i := range req.Plan.Tracks {
		switch req.Plan.Tracks[i].Kind {
		case videoplan.TrackVideoMain:
			mainTrack = &req.Plan.Tracks[i]
		case videoplan.TrackVideoInsert:
			insertTrack = &req.Plan.Tracks[i]
		}
	}

	for f := 0; f < total; f++ {
		img := image.NewRGBA(image.Rect(0, 0, w, hpx))
		base := color.RGBA{R: 16, G: 16, B: 16, A: 255} // 帧空洞：深灰（可观测，不静默）
		if mainTrack != nil {
			if c, ok := clipColorAt(*mainTrack, f); ok {
				base = c
			}
		}
		draw.Draw(img, img.Bounds(), &image.Uniform{base}, image.Point{}, draw.Src)
		// insert 轨：居中 60% 宽高的矩形（overlay 合成的最小形态）
		if insertTrack != nil {
			if c, ok := clipColorAt(*insertTrack, f); ok {
				iw, ih := w*3/5, hpx*3/5
				x0, y0 := (w-iw)/2, (hpx-ih)/2
				draw.Draw(img, image.Rect(x0, y0, x0+iw, y0+ih), &image.Uniform{c}, image.Point{}, draw.Src)
			}
		}
		path := filepath.Join(dir, fmt.Sprintf("frame_%06d.png", f))
		fh, err := os.Create(path)
		if err != nil {
			return f, fmt.Errorf("renderer: 建帧文件 %s: %w", path, err)
		}
		if err := png.Encode(fh, img); err != nil {
			fh.Close()
			return f, fmt.Errorf("renderer: PNG 编码帧 %d: %w", f, err)
		}
		fh.Close()
		if h != nil {
			bs, err := os.ReadFile(path)
			if err != nil {
				return f, err
			}
			h.Write(bs)
		}
	}
	return total, nil
}

// hasAudio 报告 plan 是否携带音频轨（决定是否合成静音轨）。
func hasAudio(p videoplan.Plan) bool {
	for _, t := range p.Tracks {
		switch t.Kind {
		case videoplan.TrackAudioMusic, videoplan.TrackAudioVO, videoplan.TrackAudioSFX:
			return true
		}
	}
	return false
}

// Render 执行完整渲染：帧序列 → ffmpeg 合成 → 输出文件。
// framesDigest（非 nil 时）回填帧序列的 sha256（R-1 测试锚点）。
func Render(req *compiler.RenderRequest, framesDigest *string) error {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return ErrFFmpegMissing
	}
	dir, err := os.MkdirTemp("", "shorts-render-")
	if err != nil {
		return fmt.Errorf("renderer: 建临时目录: %w", err)
	}
	defer os.RemoveAll(dir)

	h := sha256.New()
	n, err := RenderFrames(req, dir, h)
	if err != nil {
		return err
	}
	if want := req.Plan.TotalFrames(); n != want {
		return fmt.Errorf("renderer: R-2 帧数守恒违例：落帧 %d != 时间线 %d", n, want)
	}
	if framesDigest != nil {
		*framesDigest = "sha256:" + hex.EncodeToString(h.Sum(nil))
	}

	args := []string{
		"-y", "-hide_banner", "-loglevel", "error",
		"-fflags", "+bitexact",
		"-framerate", fmt.Sprint(req.Plan.Canvas.FPS),
		"-i", filepath.Join(dir, "frame_%06d.png"),
	}
	if hasAudio(req.Plan) {
		args = append(args,
			"-f", "lavfi", "-i", "anullsrc=r=44100:cl=stereo",
			"-shortest",
		)
	}
	args = append(args,
		"-map", "0:v",
		"-map_metadata", "-1",
		"-c:v", "libx264",
		"-flags:v", "+bitexact",
		"-pix_fmt", "yuv420p",
		"-preset", req.Output.Preset,
		"-crf", fmt.Sprint(req.Output.CRF),
	)
	if hasAudio(req.Plan) {
		args = append(args,
			"-map", "1:a",
			"-c:a", "aac", "-b:a", "96k",
			"-flags:a", "+bitexact",
		)
	}
	if req.Output.Codec != "h264" {
		return fmt.Errorf("renderer: codec %q 不受支持（Phase 0 仅 h264）", req.Output.Codec)
	}
	args = append(args, "-movflags", "+faststart", req.Output.Path)

	if err := os.MkdirAll(filepath.Dir(req.Output.Path), 0o755); err != nil {
		return fmt.Errorf("renderer: 建输出目录: %w", err)
	}
	cmd := exec.Command(ffmpeg, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("renderer: ffmpeg 合成失败: %w\n%s", err, out)
	}
	return nil
}

// RenderResponse 汇总渲染结果（C3 response 的最小可判定形态，测试与 CLI 共用）。
type RenderResponse struct {
	ContractVersion int    `json:"contract_version"`
	Status          string `json:"status"` // OK | ERROR
	OutputPath      string `json:"output_path"`
	TotalFrames     int    `json:"total_frames"`
	FramesDigest    string `json:"frames_digest"`
	OutputDigest    string `json:"output_digest"`
	RendererVersion string `json:"renderer_version"` // R-5：含字体哈希
}

// RenderPlanTo 一站式：plan + 索引/字体 → 编译 → 渲染 → 摘要（CLI/测试入口）。
func RenderPlanTo(p videoplan.Plan, idx compiler.MediaIndex, fonts []compiler.Font,
	out compiler.Output, modes compiler.Modes, expect compiler.RendererExpect) (*RenderResponse, error) {
	req, err := compiler.Compile(p, idx, fonts, out, modes, expect)
	if err != nil {
		return nil, err
	}
	var framesDigest string
	if err := Render(req, &framesDigest); err != nil {
		return nil, err
	}
	od, err := fileDigest(out.Path)
	if err != nil {
		return nil, err
	}
	return &RenderResponse{
		ContractVersion: contracts.ContractRender,
		Status:          "OK",
		OutputPath:      out.Path,
		TotalFrames:     p.TotalFrames(),
		FramesDigest:    framesDigest,
		OutputDigest:    od,
		RendererVersion: Version(expect, fonts),
	}, nil
}

func fileDigest(path string) (string, error) {
	bs, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(bs)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}
