// plan.go 构造形态1 的 VideoPlan（IR-0007 BEH-5）：生成片段为唯一主轨，
// 信息层六 overlay（五要素 + AIGC 披露）逐字取自商家信息表（INV-5：
// 确定性信息不进生成域）。构造确定性：同输入恒同 plan（PlanID 与
// Provenance 从输入内容派生，不读时钟/随机源）。
package form1

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/Cloudbird-Software/Shorts_Director/internal/digest"
	"github.com/Cloudbird-Software/Shorts_Director/internal/videoplan"
)

// 形态1 画布常量（gen_form I2V_AMBIENCE bindings：1080×1920；
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
	shopBox      = videoplan.LayoutBox{X: 100, Y: 200, W: 880, H: 90, Anchor: "TL"}
	signatureBox = videoplan.LayoutBox{X: 100, Y: 330, W: 880, H: 70, Anchor: "TL"}
	priceBox     = videoplan.LayoutBox{X: 100, Y: 440, W: 400, H: 90, Anchor: "TL"}
	addressBox   = videoplan.LayoutBox{X: 100, Y: 1600, W: 880, H: 60, Anchor: "TL"}
	phoneBox     = videoplan.LayoutBox{X: 100, Y: 1690, W: 880, H: 60, Anchor: "TL"}
	aigcBox      = videoplan.LayoutBox{X: 760, Y: 1760, W: 260, H: 36, Anchor: "TL"}
)

// planInput 是 plan 构造的全部输入（digest 材料）。
type planInput struct {
	Merchant   *Merchant
	GenHash    string // gen_i2v 产物 content_hash（生成片段版本钉死）
	Frames     int    // 时间线总帧数（duration × CanvasFPS）
	SuiteID    string
	Seed       int64
	Date       string // YYYY-MM-DD（ScheduledDate / 隐式标识 ProduceTime）
	FontFamily string // 信息层字体族（R-4：渲染时按哈希核验）
}

// BuildPlan 构造形态1 VideoPlan：单 GENERATED 主轨 + 六信息层 overlay +
// AIGC 合规回写块（IV-VP-5：explicit_overlay_id 指向 aigc.disclosure）。
func BuildPlan(in planInput) (videoplan.Plan, error) {
	if in.Frames < 1 {
		return videoplan.Plan{}, fmt.Errorf("form1: 时间线帧数必须 ≥1")
	}
	id := deterministicID(in)
	ov := func(oid, intent, component, text string, box videoplan.LayoutBox, size int) videoplan.Overlay {
		return videoplan.Overlay{
			OverlayID: oid, Intent: intent, Component: component,
			Props:      map[string]any{"text": text, "font": in.FontFamily, "size": float64(size)},
			StartFrame: 0, EndFrame: in.Frames, LayoutBox: box,
		}
	}
	m := in.Merchant
	p := videoplan.Plan{
		SchemaVersion: "video_plan/1",
		PlanID:        id,
		TenantID:      "eval",
		ScheduledDate: in.Date,
		Canvas: videoplan.Canvas{W: CanvasW, H: CanvasH, FPS: CanvasFPS,
			SafeArea: videoplan.SafeArea{Top: 120, Bottom: 120, Left: 60, Right: 60}},
		Timebase:      videoplan.Timebase{Unit: "frame", Rate: CanvasFPS},
		BeatSchemaRef: videoplan.VersionedRef{ID: "form1_ambience", Version: 1},
		StyleThemeRef: videoplan.VersionedRef{ID: "warm_default", Version: 1},
		Tracks: []videoplan.Track{{
			TrackID: "trk_gen_main", Kind: videoplan.TrackVideoMain,
			Clips: []videoplan.Clip{{
				ClipID: "clip_gen_1", BeatRole: "HOOK",
				Source: videoplan.ClipSource{Kind: "GENERATED", Ref: "gen-0001", ContentHash: in.GenHash},
				SrcIn:  0, SrcOut: in.Frames, TlStart: 0, TlEnd: in.Frames,
				Transform:    videoplan.Transform{Scale: 1, Position: &videoplan.Position{X: 0, Y: 0}},
				TransitionIn: videoplan.TransitionIn{Kind: "CUT"},
			}},
		}},
		Overlays: []videoplan.Overlay{
			ov("ov_shop", "LOGO_WATERMARK", "caption.plain", m.Info.ShopName, shopBox, 64),
			ov("ov_signature", "STATIC_CAPTION", "caption.plain", m.Info.SignatureItem, signatureBox, 44),
			ov("ov_price", "PRICE_TAG", "badge.price", m.Info.Price, priceBox, 56),
			ov("ov_address", "LOCATION_CARD", "caption.plain", m.Info.Address, addressBox, 36),
			ov("ov_phone", "STATIC_CAPTION", "caption.plain", m.Info.Phone, phoneBox, 36),
			ov("ov_aigc", "AIGC_DISCLOSURE", "aigc.disclosure", m.AIGCDisclosure, aigcBox, 20),
		},
		Provenance: videoplan.Provenance{
			GeneratedBy:   videoplan.GeneratedByDeterministic,
			ModelID:       "internal/form1@0.1.0",
			PromptVersion: "form1-v1",
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
		return videoplan.Plan{}, fmt.Errorf("form1: 构造的 plan 不合法: %w", err)
	}
	return p, nil
}

// deterministicID 从输入内容派生 plan id（确定性，非 UUIDv7 时间源）。
func deterministicID(in planInput) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("form1|%s|%s|%d|%s|%d",
		in.Merchant.ID, in.GenHash, in.Frames, in.SuiteID, in.Seed)))
	return "f1-" + hex.EncodeToString(h[:16])
}

// inputDigest 对构造输入做 JCS 摘要（Provenance.InputDigest）。
func inputDigest(in planInput) string {
	d, err := digest.ValueDigest(map[string]any{
		"merchant_id": in.Merchant.ID,
		"info":        in.Merchant.Info,
		"aigc":        in.Merchant.AIGCDisclosure,
		"gen_hash":    in.GenHash,
		"frames":      in.Frames,
		"suite_id":    in.SuiteID,
		"seed":        in.Seed,
	})
	if err != nil {
		return "sha256:undigestable"
	}
	return d
}
