// plan.go 构造形态4 的 VideoPlan（IR-0007 BEH-6）：口播视频为唯一主轨
// （GENERATED，gen_lipsync 产物），AUDIO_VO 轨 + plan.Audio.VORef 钉死
// gen_tts 产物版本（渲染时 R-4 hash 核验后混入成品）。信息层最小集：
// 品牌名 + AIGC 披露（AC-7「至少品牌名信息层」）。构造确定性：同输入
// 恒同 plan（PlanID 与 Provenance 从输入内容派生，不读时钟/随机源）。
package form4

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/Cloudbird-Software/Shorts_Director/internal/digest"
	"github.com/Cloudbird-Software/Shorts_Director/internal/videoplan"
)

// 形态4 画布常量（gen_form DIGITAL_HUMAN bindings：1080×1920；
// fps 取冻结值 25——渲染时间线与生成源 fps 解耦）。
const (
	CanvasW   = 1080
	CanvasH   = 1920
	CanvasFPS = 25
)

// 信息层布局（TL 锚点，像素；全部落在 safe_area 内——渲染器 R-3 复检）。
// aigcBox 同时是 aigc_overlay_present 探针的比对区域（探针 args 与此
// 钉死同值，改布局必须同步断言包）。
var (
	brandBox = videoplan.LayoutBox{X: 100, Y: 200, W: 880, H: 90, Anchor: "TL"}
	aigcBox  = videoplan.LayoutBox{X: 760, Y: 1760, W: 260, H: 36, Anchor: "TL"}
)

// planInput 是 plan 构造的全部输入（digest 材料）。
type planInput struct {
	Brand      string // 品牌名（信息层 overlay——逐字取自口播脚本）
	AIGCText   string // AIGC 披露文案（逐字取自商家信息表）
	GenHash    string // gen_lipsync 产物 content_hash（主轨版本钉死）
	VOHash     string // gen_tts 产物 content_hash（VORef 版本钉死）
	Frames     int    // 时间线总帧数（口播时长 × CanvasFPS）
	SuiteID    string
	Seed       int64
	Date       string // YYYY-MM-DD（ScheduledDate / 隐式标识 ProduceTime）
	FontFamily string // 信息层字体族（R-4：渲染时按哈希核验）
}

// BuildPlan 构造形态4 VideoPlan：单 GENERATED 主轨 + AUDIO_VO 轨 +
// 品牌名/AIGC 两 overlay + AIGC 合规回写块（IV-VP-5）。
func BuildPlan(in planInput) (videoplan.Plan, error) {
	if in.Frames < 1 {
		return videoplan.Plan{}, fmt.Errorf("form4: 时间线帧数必须 ≥1")
	}
	id := deterministicID(in)
	ov := func(oid, intent, component, text string, box videoplan.LayoutBox, size int) videoplan.Overlay {
		return videoplan.Overlay{
			OverlayID: oid, Intent: intent, Component: component,
			Props:      map[string]any{"text": text, "font": in.FontFamily, "size": float64(size)},
			StartFrame: 0, EndFrame: in.Frames, LayoutBox: box,
		}
	}
	p := videoplan.Plan{
		SchemaVersion: "video_plan/1",
		PlanID:        id,
		TenantID:      "eval",
		ScheduledDate: in.Date,
		Canvas: videoplan.Canvas{W: CanvasW, H: CanvasH, FPS: CanvasFPS,
			SafeArea: videoplan.SafeArea{Top: 120, Bottom: 120, Left: 60, Right: 60}},
		Timebase:      videoplan.Timebase{Unit: "frame", Rate: CanvasFPS},
		BeatSchemaRef: videoplan.VersionedRef{ID: "form4_digital_human", Version: 1},
		StyleThemeRef: videoplan.VersionedRef{ID: "warm_default", Version: 1},
		Tracks: []videoplan.Track{
			{
				TrackID: "trk_lip_main", Kind: videoplan.TrackVideoMain,
				Clips: []videoplan.Clip{{
					ClipID: "clip_lip_1", BeatRole: "HOOK",
					Source: videoplan.ClipSource{Kind: "GENERATED", Ref: "lip-0001", ContentHash: in.GenHash},
					SrcIn:  0, SrcOut: in.Frames, TlStart: 0, TlEnd: in.Frames,
					Transform:    videoplan.Transform{Scale: 1, Position: &videoplan.Position{X: 0, Y: 0}},
					TransitionIn: videoplan.TransitionIn{Kind: "CUT"},
				}},
			},
			// AUDIO_VO 轨标记音频存在（clips 空为 schema 冻结形态——
			// 音频不走 resolved_media，VO 产物由 vo_ref.hash 钉死、
			// 渲染宿主挂载注入）。
			{TrackID: "trk_vo", Kind: videoplan.TrackAudioVO, Clips: []videoplan.Clip{}},
		},
		Overlays: []videoplan.Overlay{
			ov("ov_brand", "LOGO_WATERMARK", "caption.plain", in.Brand, brandBox, 64),
			ov("ov_aigc", "AIGC_DISCLOSURE", "aigc.disclosure", in.AIGCText, aigcBox, 20),
		},
		Audio: videoplan.Audio{
			TargetLUFS: -16,
			VORef:      &videoplan.VORef{TTSID: "tts-0001", Hash: in.VOHash},
		},
		Provenance: videoplan.Provenance{
			GeneratedBy:   videoplan.GeneratedByDeterministic,
			ModelID:       "internal/form4@0.1.0",
			PromptVersion: "form4-v1",
			InputDigest:   inputDigest(in),
			Seed:          &in.Seed,
			CreatedAt:     in.Date + "T00:00:00Z",
		},
	}
	// AIGC 合规回写（IV-VP-5：显式角标 id 必须指向 aigc.disclosure overlay）
	aigcID := "ov_aigc"
	p.Compliance = &videoplan.ComplianceResult{
		AIGCDisclosure: videoplan.AIGCDisclosure{Required: true, ExplicitOverlayID: &aigcID},
		ChecksPassed:   []string{"aigc_explicit_overlay", "aigc_implicit_metadata"},
	}
	if err := p.Validate(); err != nil {
		return videoplan.Plan{}, fmt.Errorf("form4: 构造的 plan 不合法: %w", err)
	}
	return p, nil
}

// deterministicID 从输入内容派生 plan id（确定性，非 UUIDv7 时间源）。
func deterministicID(in planInput) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("form4|%s|%s|%s|%d|%s|%d",
		in.Brand, in.GenHash, in.VOHash, in.Frames, in.SuiteID, in.Seed)))
	return "f4-" + hex.EncodeToString(h[:16])
}

// inputDigest 对构造输入做 JCS 摘要（Provenance.InputDigest）。
func inputDigest(in planInput) string {
	d, err := digest.ValueDigest(map[string]any{
		"brand":    in.Brand,
		"aigc":     in.AIGCText,
		"gen_hash": in.GenHash,
		"vo_hash":  in.VOHash,
		"frames":   in.Frames,
		"suite_id": in.SuiteID,
		"seed":     in.Seed,
	})
	if err != nil {
		return "sha256:undigestable"
	}
	return d
}
