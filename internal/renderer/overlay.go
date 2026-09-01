// overlay.go 实现信息层叠加（IR-0007 INV-5 / E4）：确定性信息
// （店名/价格/地址/电话/AIGC 披露）只经 overlay → ffmpeg drawtext 进入
// 成品，禁止交给生成域。文本经 textfile 传参（零转义歧义）；
// R-3 安全区拒绝：LayoutBox 越出安全区即报错；R-4：字体文件哈希
// 与 Font.Hash 不符即报错（无隐式换字体）。
package renderer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Cloudbird-Software/Shorts_Director/internal/compiler"
	"github.com/Cloudbird-Software/Shorts_Director/internal/videoplan"
)

// overlayDraw 是一条已解析的 drawtext 指令。
type overlayDraw struct {
	TextFile   string
	FontFile   string
	X, Y       int
	FontSize   int
	FontColor  string
	StartFrame int
	EndFrame   int
}

// resolveOverlays 把 plan overlays 解析成 drawtext 指令：
// 布局盒换算锚点 → 安全区校验 → 字体定位与哈希核验 → 文本落临时文件。
func resolveOverlays(p videoplan.Plan, fonts []compiler.Font, workdir string) ([]overlayDraw, error) {
	byFamily := map[string]compiler.Font{}
	for _, f := range fonts {
		byFamily[f.Family] = f
	}
	var draws []overlayDraw
	for i, o := range p.Overlays {
		if o.EndFrame <= o.StartFrame {
			continue // 不可见（IV-VP-3 已保证不越总时长）
		}
		text, err := overlayText(p, o)
		if err != nil {
			return nil, fmt.Errorf("renderer: overlay %s: %w", o.OverlayID, err)
		}
		// 安全区（R-3）：布局盒必须完整落在 canvas−safe_area 内
		x, y, err := anchorTopLeft(o.LayoutBox, p.Canvas)
		if err != nil {
			return nil, fmt.Errorf("renderer: overlay %s: %w", o.OverlayID, err)
		}
		// 字体：family（font 或 font_family 键）→ 路径 + 哈希核验
		family, _ := o.Props["font"].(string)
		if family == "" {
			family, _ = o.Props["font_family"].(string)
		}
		if family == "" {
			return nil, fmt.Errorf("renderer: overlay %s 缺 props.font（字体族必填）", o.OverlayID)
		}
		font, ok := byFamily[family]
		if !ok {
			return nil, fmt.Errorf("renderer: overlay %s 引用未提供的字体 %q", o.OverlayID, family)
		}
		if err := verifyFontHash(font); err != nil {
			return nil, fmt.Errorf("renderer: overlay %s: %w", o.OverlayID, err)
		}
		size := 48
		if v, ok := o.Props["size"].(float64); ok && v > 0 {
			size = int(v)
		}
		color := "white"
		if v, ok := o.Props["color"].(string); ok && v != "" {
			color = v
		}
		tf := filepath.Join(workdir, fmt.Sprintf("overlay_%03d.txt", i))
		if err := os.WriteFile(tf, []byte(text), 0o644); err != nil {
			return nil, err
		}
		draws = append(draws, overlayDraw{
			TextFile: tf, FontFile: font.Path,
			X: x, Y: y, FontSize: size, FontColor: color,
			StartFrame: o.StartFrame, EndFrame: o.EndFrame,
		})
	}
	return draws, nil
}

// overlayText 解析 overlay 文本：props.text 直取（信息层模板已在上游
// 用信息表渲染——INV-5 确定性信息不进生成域）；caption 类经 block_ref
// 引用 Copy.CaptionBlocks 的文本。
func overlayText(p videoplan.Plan, o videoplan.Overlay) (string, error) {
	if text, ok := o.Props["text"].(string); ok && strings.TrimSpace(text) != "" {
		return text, nil
	}
	if ref, ok := o.Props["block_ref"].(string); ok && ref != "" {
		for _, b := range p.Copy.CaptionBlocks {
			if b.BlockID == ref && strings.TrimSpace(b.Text) != "" {
				return b.Text, nil
			}
		}
		return "", fmt.Errorf("block_ref %q 在 copy.caption_blocks 无非空文本", ref)
	}
	return "", fmt.Errorf("缺 props.text（信息层文本必填；caption 类须带可解析的 block_ref）")
}

// anchorTopLeft 按锚点换算布局盒左上角，并校验安全区（R-3）。
func anchorTopLeft(b videoplan.LayoutBox, cv videoplan.Canvas) (int, int, error) {
	x, y := b.X, b.Y
	switch b.Anchor {
	case "TC", "CC", "BC":
		x = b.X - b.W/2
	case "TR", "CR", "BR":
		x = b.X - b.W
	}
	switch b.Anchor {
	case "CL", "CC", "CR":
		y = b.Y - b.H/2
	case "BL", "BC", "BR":
		y = b.Y - b.H
	}
	if x < cv.SafeArea.Left || y < cv.SafeArea.Top ||
		x+b.W > cv.W-cv.SafeArea.Right || y+b.H > cv.H-cv.SafeArea.Bottom {
		return 0, 0, fmt.Errorf(
			"R-3 安全区拒绝：布局盒 (%d,%d %dx%d) 越出安全区（canvas %dx%d − safe %d/%d/%d/%d）",
			x, y, b.W, b.H, cv.W, cv.H,
			cv.SafeArea.Left, cv.SafeArea.Top, cv.SafeArea.Right, cv.SafeArea.Bottom)
	}
	return x, y, nil
}

// verifyFontHash 校验字体文件内容哈希与声明的 Font.Hash 一致（R-4）。
func verifyFontHash(f compiler.Font) error {
	bs, err := os.ReadFile(f.Path)
	if err != nil {
		return fmt.Errorf("字体文件不可读 %s: %w", f.Path, err)
	}
	sum := sha256.Sum256(bs)
	if got := "sha256:" + hex.EncodeToString(sum[:]); got != f.Hash {
		return fmt.Errorf("字体 %s 哈希漂移：声明 %s，实际 %s（R-4 无隐式换字体）", f.Family, f.Hash, got)
	}
	return nil
}

// drawtextFilter 生成 ffmpeg drawtext 过滤器（textfile 传文本，免转义；
// enable=between(n,start,end) 控制帧窗）。
func drawtextFilter(d overlayDraw) string {
	enable := fmt.Sprintf("between(n,%d,%d)", d.StartFrame, d.EndFrame-1)
	return fmt.Sprintf(
		"drawtext=textfile=%s:fontfile=%s:x=%d:y=%d:fontsize=%d:fontcolor=%s:enable='%s'",
		escapeFilterPath(d.TextFile), escapeFilterPath(d.FontFile),
		d.X, d.Y, d.FontSize, d.FontColor, enable)
}

// escapeFilterPath 转义 filtergraph 值里的特殊字符（\ : , '）。
func escapeFilterPath(p string) string {
	r := strings.NewReplacer(`\`, `\\`, `:`, `\:`, `,`, `\,`, `'`, `\'`)
	return r.Replace(p)
}
