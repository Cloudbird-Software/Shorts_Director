// Package renderer 是 C3 渲染契约的执行端（Phase 0 → IR-0007 E4/E5）。
//
// 渲染路径两级：
//   - 真实解码（生产，modes.placeholder_media=false）：SHOT/GENERATED 轨经
//     ffmpeg 抽帧归一化到画布（letterbox），纯 Go 合成时间线帧。
//   - 占位纯色帧（测试模式，modes.placeholder_media=true）：content_hash
//     派生色——保留给无真实媒体的单元测试。
//
// 信息层（INV-5）：确定性信息（文字/价格/电话/AIGC 披露）只经 overlay →
// ffmpeg drawtext（textfile 传参）进入成品；R-3 安全区拒绝、R-4 字体哈希
// 核验。AIGC 双轨：显式角标 overlay + 容器隐式元数据（compliance/aigc）。
//
// 契约遵守（R-1..R-5 渲染侧可自证的部分）：
//   - R-1 确定性：帧生成无时钟/随机源；ffmpeg 全参数 bitexact，
//     同输入同版本恒同字节（双跑 digest 全等测试）。
//   - R-2 帧数守恒：先落满 TotalFrames 张帧再合成，帧数不符即失败。
//   - R-5 版本含字体哈希：Version() 拼接 ffmpeg 版本 + 排序后的字体哈希。
//
// 依赖纪律（AGENTS.md 硬规则 3）：零新增 Go 依赖——视频解码/编码经外部
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
	"github.com/Cloudbird-Software/Shorts_Director/internal/compliance"
	"github.com/Cloudbird-Software/Shorts_Director/internal/contracts"
	"github.com/Cloudbird-Software/Shorts_Director/internal/videoplan"
)

// ErrFFmpegMissing 表示运行环境没有 ffmpeg 二进制（R-4：显式报错，不静默降级）。
var ErrFFmpegMissing = fmt.Errorf("renderer: 环境缺 ffmpeg 二进制（渲染的外部依赖）")

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

// hashColor 从内容哈希/ref 确定性派生一个不刺眼的不透明色（占位模式）。
func hashColor(seed string) color.RGBA {
	sum := sha256.Sum256([]byte("placeholder:" + seed))
	r, g, b := sum[0], sum[1], sum[2]
	return color.RGBA{R: 64 + r%144, G: 64 + g%144, B: 64 + b%144, A: 255}
}

// clipLayerAt 返回第 frame 帧上某轨道的活跃层：
// 真实解码模式返回媒体帧；占位模式/GRAPHIC/COLOR 返回派生色。
func clipLayerAt(t videoplan.Track, frame int, media map[string]*mediaFrames) (image.Image, color.RGBA, bool) {
	for _, c := range t.Clips {
		if frame >= c.TlStart && frame < c.TlEnd {
			if (c.Source.Kind == "SHOT" || c.Source.Kind == "GENERATED") && media != nil {
				if mf, ok := media[mediaKey(c.Source.Kind, c.Source.Ref)]; ok {
					// 帧不足在 frame() 内报错；此处上层已保证帧库够长
					if img, err := mf.frame(frame - c.TlStart); err == nil {
						return img, color.RGBA{}, true
					}
				}
			}
			seed := c.Source.Ref
			if c.Source.Kind == "SHOT" || c.Source.Kind == "GENERATED" {
				seed = c.Source.ContentHash
			}
			return nil, hashColor(seed), true
		}
	}
	return nil, color.RGBA{}, false
}

// RenderFrames 把整个时间线渲成帧序列（纯 Go、确定性）。
// 媒体帧库为 nil（或占位模式）时走纯色占位路径——测试模式入口。
// 返回写入的帧数；R-2 校验：必须等于 plan.TotalFrames()。
func RenderFrames(req *compiler.RenderRequest, dir string, h hash.Hash) (int, error) {
	return renderFrames(req, dir, h, nil)
}

// renderFrames 是帧合成的主体：media 非 nil 时启用真实解码帧。
func renderFrames(req *compiler.RenderRequest, dir string, h hash.Hash, media map[string]*mediaFrames) (int, error) {
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
		draw.Draw(img, img.Bounds(),
			&image.Uniform{color.RGBA{R: 16, G: 16, B: 16, A: 255}}, // 帧空洞：深灰（可观测，不静默）
			image.Point{}, draw.Src)
		if mainTrack != nil {
			if m, c, ok := clipLayerAt(*mainTrack, f, media); ok {
				if m != nil {
					draw.Draw(img, img.Bounds(), m, image.Point{}, draw.Src)
				} else {
					draw.Draw(img, img.Bounds(), &image.Uniform{c}, image.Point{}, draw.Src)
				}
			}
		}
		// insert 轨：居中 60% 宽高区域（overlay 合成的最小形态）
		if insertTrack != nil {
			if m, c, ok := clipLayerAt(*insertTrack, f, media); ok {
				iw, ih := w*3/5, hpx*3/5
				x0, y0 := (w-iw)/2, (hpx-ih)/2
				rect := image.Rect(x0, y0, x0+iw, y0+ih)
				if m != nil {
					draw.Draw(img, rect, m, image.Point{}, draw.Src) // 媒体已归一化画布，裁中显示
				} else {
					draw.Draw(img, rect, &image.Uniform{c}, image.Point{}, draw.Src)
				}
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

// Render 执行完整渲染：抽帧（真实媒体）→ 帧序列 → 信息层 drawtext +
// AIGC 元数据 → ffmpeg 合成 → 输出文件。
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

	media, err := prepareMedia(req, dir)
	if err != nil {
		return err
	}
	fdir := filepath.Join(dir, "frames")
	if err := os.MkdirAll(fdir, 0o755); err != nil {
		return err
	}
	h := sha256.New()
	n, err := renderFrames(req, fdir, h, media)
	if err != nil {
		return err
	}
	if want := req.Plan.TotalFrames(); n != want {
		return fmt.Errorf("renderer: R-2 帧数守恒违例：落帧 %d != 时间线 %d", n, want)
	}
	if framesDigest != nil {
		*framesDigest = "sha256:" + hex.EncodeToString(h.Sum(nil))
	}

	// 信息层：overlays → drawtext 过滤器链（R-3 安全区 / R-4 字体哈希核验）
	draws, err := resolveOverlays(req.Plan, req.Fonts, dir)
	if err != nil {
		return err
	}

	args := []string{
		"-y", "-hide_banner", "-loglevel", "error",
		"-fflags", "+bitexact",
		"-framerate", fmt.Sprint(req.Plan.Canvas.FPS),
		"-i", filepath.Join(fdir, "frame_%06d.png"),
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
	)
	if len(draws) > 0 {
		filters := make([]string, 0, len(draws))
		for _, d := range draws {
			filters = append(filters, drawtextFilter(d))
		}
		args = append(args, "-vf", strings.Join(filters, ","))
	}
	args = append(args,
		"-c:v", "libx264",
		"-flags:v", "+bitexact",
		"-pix_fmt", "yuv420p",
		"-preset", req.Output.Preset,
		"-crf", fmt.Sprint(req.Output.CRF),
	)
	// AIGC 隐式标识（GB 45438 双轨的元数据轨）：确定性来源（无时钟——
	// produceTime 取 plan.ScheduledDate，identifier 取 plan_id）。
	// mp4 muxer 默认只写白名单键，自定义键须 use_metadata_tags。
	movflags := "+faststart"
	if compliance.DisclosureRequired(&req.Plan) {
		label := compliance.BuildImplicitLabel(
			"Shorts_Director", req.Plan.ScheduledDate, req.Plan.PlanID)
		for _, k := range []string{"AIGC", "AIGC_LABEL_VERSION"} {
			args = append(args, "-metadata", k+"="+label[k])
		}
		movflags += "+use_metadata_tags"
	}
	if hasAudio(req.Plan) {
		args = append(args,
			"-map", "1:a",
			"-c:a", "aac", "-b:a", "96k",
			"-flags:a", "+bitexact",
		)
	}
	if req.Output.Codec != "h264" {
		return fmt.Errorf("renderer: codec %q 不受支持（当前仅 h264）", req.Output.Codec)
	}
	args = append(args, "-movflags", movflags, req.Output.Path)

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
