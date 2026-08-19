// Package compiler 把 VideoPlan IR 编译成 C3 RenderRequest（S6 阶段 0）。
// 核心原则 R-4 无隐式回退：渲染器不查库，媒体/字体由控制面预解析；
// 缺媒体/字体/hash 漂移一律报错，绝不静默降级渲出"差不多的"结果。
package compiler

import (
	"fmt"

	"github.com/Cloudbird-Software/Shorts_Director/internal/contracts"
	"github.com/Cloudbird-Software/Shorts_Director/internal/videoplan"
)

// ResolvedMedia 是控制面预解析的媒体（ref 形如 shot:<uuid> / music:<id>）。
type ResolvedMedia struct {
	Ref         string `json:"ref"`
	LocalPath   string `json:"local_path"`
	ContentHash string `json:"content_hash"`
	FPS         int    `json:"fps"`
}

// Font 是字体三元组（R-5：renderer_version 必须含字体哈希的来源）。
type Font struct {
	Family string `json:"family"`
	Path   string `json:"path"`
	Hash   string `json:"hash"`
}

// Output 是渲染输出规格。
type Output struct {
	Path         string `json:"path"`
	Codec        string `json:"codec"` // h264 | prores4444
	CRF          int    `json:"crf,omitempty"`
	Preset       string `json:"preset,omitempty"`
	AudioBitrate string `json:"audio_bitrate,omitempty"`
}

// Modes 是渲染模式开关。
type Modes struct {
	OverlayOnly   bool `json:"overlay_only"`
	Preview       bool `json:"preview"`
	Deterministic bool `json:"deterministic"` // R-1 开关：禁用一切时间/随机依赖
}

// RendererExpect 是版本钉死清单（防漂移）。
type RendererExpect struct {
	FFmpeg  string `json:"ffmpeg"`
	Remotion string `json:"remotion"`
	Node    string `json:"node"`
}

// RenderRequest 是 C3 RenderRequest v1 的 Go 实体。
type RenderRequest struct {
	ContractVersion int               `json:"contract_version"`
	Plan            videoplan.Plan    `json:"plan"`
	ResolvedMedia   []ResolvedMedia   `json:"resolved_media"`
	Fonts           []Font            `json:"fonts"`
	Output          Output            `json:"output"`
	Modes           Modes             `json:"modes"`
	RendererExpect  RendererExpect    `json:"renderer_expect"`
}

// Validate 校验 RenderRequest 的契约形态（C3 request schema 的 Go 侧镜像）。
func (r RenderRequest) Validate() error {
	if r.ContractVersion != contracts.ContractRender {
		return fmt.Errorf("compiler: contract_version 必须 %d", contracts.ContractRender)
	}
	if err := r.Plan.Validate(); err != nil {
		return fmt.Errorf("compiler: plan: %w", err)
	}
	if len(r.ResolvedMedia) == 0 && r.hasVideoClips() {
		return fmt.Errorf("compiler: 含视频 clip 的 plan 必须 resolved_media 非空")
	}
	seen := map[string]bool{}
	for i, m := range r.ResolvedMedia {
		if m.Ref == "" || m.LocalPath == "" || m.ContentHash == "" || m.FPS <= 0 {
			return fmt.Errorf("compiler: resolved_media[%d] 字段不全（ref/local_path/content_hash/fps）", i)
		}
		if seen[m.Ref] {
			return fmt.Errorf("compiler: resolved_media[%d] ref %q 重复", i, m.Ref)
		}
		seen[m.Ref] = true
	}
	for i, f := range r.Fonts {
		if f.Family == "" || f.Path == "" || f.Hash == "" {
			return fmt.Errorf("compiler: fonts[%d] 字段不全（family/path/hash）", i)
		}
	}
	switch r.Output.Codec {
	case "h264", "prores4444":
	default:
		return fmt.Errorf("compiler: output.codec 非法 %q", r.Output.Codec)
	}
	if r.Output.Path == "" {
		return fmt.Errorf("compiler: output.path 必填")
	}
	if r.RendererExpect.FFmpeg == "" || r.RendererExpect.Remotion == "" || r.RendererExpect.Node == "" {
		return fmt.Errorf("compiler: renderer_expect 三元组必填（ffmpeg/remotion/node）")
	}
	return nil
}

func (r RenderRequest) hasVideoClips() bool {
	for _, t := range r.Plan.Tracks {
		if t.Kind == videoplan.TrackVideoMain || t.Kind == videoplan.TrackVideoInsert {
			if len(t.Clips) > 0 {
				return true
			}
		}
	}
	return false
}
