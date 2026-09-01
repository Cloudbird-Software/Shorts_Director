// media.go 实现 GENERATED/SHOT 轨的真实媒体解码（IR-0007 E4/E5 渲染侧）：
// ffmpeg 逐源抽帧（fps 对齐 + 画幅 letterbox）→ 纯 Go 合成进时间线。
// 确定性：抽帧参数全固定（无时钟/随机），同源文件同版本恒同帧。
package renderer

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/Cloudbird-Software/Shorts_Director/internal/compiler"
	"github.com/Cloudbird-Software/Shorts_Director/internal/videoplan"
)

// mediaFrames 是一个已抽帧媒体源的内容寻址帧库。
type mediaFrames struct {
	dir   string // 帧文件目录 frame_%06d.png
	count int
}

// frame 解码第 i 帧（越界 = 素材短于时间线——R-4 报错不静默）。
func (m *mediaFrames) frame(i int) (image.Image, error) {
	if i < 0 || i >= m.count {
		return nil, fmt.Errorf("renderer: 素材帧不足（要第 %d 帧，共 %d）——时间线引用长于源媒体", i, m.count)
	}
	fh, err := os.Open(filepath.Join(m.dir, fmt.Sprintf("frame_%06d.png", i)))
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	return png.Decode(fh)
}

// prepareMedia 对 resolved_media 逐源抽帧：
//
//	ffmpeg -i <src> -vf fps=<planFPS>,scale=W:H:force_original_aspect_ratio=decrease,
//	       pad=W:H:(ow-iw)/2:(oh-ih)/2:black,format=rgba -start_number 0 out_%06d.png
//
// 帧数上限 = 时间线上引用该源的最长 clip。PlaceholderMedia 模式跳过（测试模式）。
func prepareMedia(req *compiler.RenderRequest, workdir string) (map[string]*mediaFrames, error) {
	if req.Modes.PlaceholderMedia {
		return nil, nil
	}
	need := map[string]int{} // ref → 最大引用帧数
	for _, t := range req.Plan.Tracks {
		if t.Kind != videoplan.TrackVideoMain && t.Kind != videoplan.TrackVideoInsert {
			continue
		}
		for _, c := range t.Clips {
			if c.Source.Kind != "SHOT" && c.Source.Kind != "GENERATED" {
				continue
			}
			ref := mediaKey(c.Source.Kind, c.Source.Ref)
			if n := c.TlEnd - c.TlStart; n > need[ref] {
				need[ref] = n
			}
		}
	}
	out := map[string]*mediaFrames{}
	for _, m := range req.ResolvedMedia {
		n, ok := need[m.Ref]
		if !ok {
			continue
		}
		dir := filepath.Join(workdir, "media", strconv.Itoa(len(out)))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
		if err := extractFrames(m, req.Plan.Canvas, n, dir); err != nil {
			return nil, err
		}
		count, err := countPNGs(dir)
		if err != nil || count < n {
			// R-4 无隐式回退：素材短于时间线引用即报错，不静默续黑帧
			return nil, fmt.Errorf(
				"renderer: 媒体 %s 抽帧不足（得 %d 帧，时间线引用 %d 帧）——源媒体短于 clip: %w",
				m.Ref, count, n, err)
		}
		out[m.Ref] = &mediaFrames{dir: dir, count: count}
	}
	return out, nil
}

// extractFrames 用 ffmpeg 抽帧并归一化到画布（letterbox 黑边填充）。
func extractFrames(m compiler.ResolvedMedia, cv videoplan.Canvas, want int, dir string) error {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return ErrFFmpegMissing
	}
	w, h := cv.W, cv.H
	vf := fmt.Sprintf(
		"fps=%d,scale=%d:%d:force_original_aspect_ratio=decrease,"+
			"pad=%d:%d:(ow-iw)/2:(oh-ih)/2:black,format=rgba",
		cv.FPS, w, h, w, h)
	args := []string{
		"-y", "-hide_banner", "-loglevel", "error",
		"-fflags", "+bitexact",
		"-i", m.LocalPath,
		"-vf", vf,
		"-frames:v", strconv.Itoa(want),
		"-start_number", "0",
		filepath.Join(dir, "frame_%06d.png"),
	}
	if out, err := exec.Command(ffmpeg, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("renderer: 媒体 %s 抽帧失败（%s）: %w\n%s", m.Ref, m.LocalPath, err, out)
	}
	return nil
}

func countPNGs(dir string) (int, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, e := range ents {
		if filepath.Ext(e.Name()) == ".png" {
			n++
		}
	}
	return n, nil
}

// mediaKey 与 compiler 的 ref 规则一致：kind 小写 + ":" + id。
func mediaKey(kind, id string) string {
	if kind == "SHOT" {
		return "shot:" + id
	}
	return "generated:" + id
}
