// compile.go 是 VideoPlan → RenderRequest 的编译主体。
// 媒体/字体解析经 MediaIndex/FontPack 注入（渲染器不查库）；
// 编译器负责 R-4 完备性校验：引用必须可解析、hash 必须一致。
package compiler

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Cloudbird-Software/Shorts_Director/internal/videoplan"
)

// MediaEntry 是一个媒体引用的解析结果。
type MediaEntry struct {
	LocalPath   string
	ContentHash string // sha256:<hex>；与 plan 内引用比对（R-4）
	FPS         int
}

// MediaIndex 是 ref（shot:<uuid> 等）→ 解析结果。由素材解析服务注入。
type MediaIndex map[string]MediaEntry

// Compile 编译：收集 plan 的全部媒体引用并预解析、校验字体完备、
// 组装 RenderRequest。确定性：输出字段按 ref 排序（同 plan 恒同 request）。
func Compile(p videoplan.Plan, idx MediaIndex, fonts []Font,
	out Output, modes Modes, expect RendererExpect) (*RenderRequest, error) {

	if err := p.Validate(); err != nil {
		return nil, fmt.Errorf("compiler: plan 不合法: %w", err)
	}
	if out.Path == "" || out.Codec == "" {
		return nil, fmt.Errorf("compiler: output.path/codec 必填")
	}

	refs := map[string]string{} // ref → 要求的 content_hash
	need := func(ref, wantHash string) error {
		if prev, dup := refs[ref]; dup {
			if prev != wantHash {
				return fmt.Errorf("compiler: %s 被 plan 以两个 content_hash 引用（%s / %s）", ref, prev, wantHash)
			}
			return nil
		}
		refs[ref] = wantHash
		return nil
	}

	// 视频轨 clip：SHOT/GENERATED 需解析；GRAPHIC/COLOR 渲染器内置
	for _, t := range p.Tracks {
		if t.Kind != videoplan.TrackVideoMain && t.Kind != videoplan.TrackVideoInsert {
			continue
		}
		for _, c := range t.Clips {
			switch c.Source.Kind {
			case "SHOT", "GENERATED":
				ref := mediaRef(c.Source.Kind, c.Source.Ref)
				if err := need(ref, c.Source.ContentHash); err != nil {
					return nil, err
				}
			case "GRAPHIC", "COLOR":
				// 无外部媒体
			default:
				return nil, fmt.Errorf("compiler: clip %s 的 source.kind 非法 %q", c.ClipID, c.Source.Kind)
			}
		}
	}
	// 音频不走 resolved_media（C3 契约样本冻结的形态）：
	// 库音乐由渲染宿主只读挂载按 id 映射；VO 产物由 vo_ref.hash 钉死版本。

	// R-4 完备性：每个引用必须可解析且 hash 一致（无隐式回退）
	sorted := make([]string, 0, len(refs))
	for ref := range refs {
		sorted = append(sorted, ref)
	}
	sort.Strings(sorted) // 确定性输出
	media := make([]ResolvedMedia, 0, len(sorted))
	for _, ref := range sorted {
		entry, ok := idx[ref]
		if !ok {
			return nil, fmt.Errorf("compiler: R-4 无隐式回退：媒体 %s 未解析（缺素材或素材已下线）", ref)
		}
		if entry.LocalPath == "" || entry.FPS <= 0 {
			return nil, fmt.Errorf("compiler: 媒体 %s 解析不完整（local_path/fps）", ref)
		}
		if want := refs[ref]; want != "" && entry.ContentHash != want {
			return nil, fmt.Errorf(
				"compiler: 媒体 %s 版本漂移：plan 钉死 %s，现库 %s——素材重转码后旧 plan 必须显式失效",
				ref, want, entry.ContentHash)
		}
		media = append(media, ResolvedMedia{
			Ref: ref, LocalPath: entry.LocalPath,
			ContentHash: entry.ContentHash, FPS: entry.FPS,
		})
	}

	// 字体完备性：overlay props 引用的 font_family 必须在字体包内
	if err := checkFonts(p, fonts); err != nil {
		return nil, err
	}

	req := &RenderRequest{
		ContractVersion: 1,
		Plan:            p,
		ResolvedMedia:   media,
		Fonts:           fonts,
		Output:          out,
		Modes:           modes,
		RendererExpect:  expect,
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	return req, nil
}

// mediaRef 生成引用键：kind 小写 + ":" + id（shot:<uuid> / music:<id>）。
func mediaRef(kind, id string) string {
	return strings.ToLower(kind) + ":" + id
}

// checkFonts 递归收集 overlay props 中的 font_family 引用并校验完备。
func checkFonts(p videoplan.Plan, fonts []Font) error {
	have := map[string]bool{}
	for _, f := range fonts {
		have[f.Family] = true
	}
	used := map[string]bool{}
	for _, o := range p.Overlays {
		collectFontFamilies(o.Props, used)
	}
	missing := make([]string, 0)
	for family := range used {
		if !have[family] {
			missing = append(missing, family)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("compiler: R-4 无隐式回退：overlay 引用未提供的字体 %v", missing)
	}
	return nil
}

// collectFontFamilies 递归收集 props 中的字体引用。
// 组件词表用 "font"（与 schema 样本一致），"font_family" 为兼容别名。
func collectFontFamilies(v any, out map[string]bool) {
	switch x := v.(type) {
	case map[string]any:
		for k, e := range x {
			if k == "font" || k == "font_family" {
				if s, ok := e.(string); ok && s != "" {
					out[s] = true
				}
				continue
			}
			collectFontFamilies(e, out)
		}
	case []any:
		for _, e := range x {
			collectFontFamilies(e, out)
		}
	}
}
