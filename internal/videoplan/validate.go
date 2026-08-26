package videoplan

import (
	"errors"
	"fmt"
	"sort"
)

// overlayComponentWhitelist 是 IV-VP-4 的渲染组件注册表
// （渲染层 Remotion 组件 id；Phase 1 先冻结 5 个，扩表走 CHANGELOG）。
var overlayComponentWhitelist = map[string]bool{
	"caption.karaoke":        true,
	"caption.plain":          true,
	"aigc.disclosure":        true,
	"card.terminal_fallback": true,
	"badge.price":            true,
}

// Validate 校验 VideoPlan 的跨字段不变式（IV-VP-1..5）。
// 结构级约束（required/const/enum）由 TS 侧 G1 harness 负责；
// 本方法只做"结构合法之后"的业务不变式——边界见 entity 包注释。
func (p Plan) Validate() error {
	if p.SchemaVersion != "video_plan/1" {
		return fmt.Errorf("videoplan: schema_version 必须 video_plan/1，得到 %q", p.SchemaVersion)
	}
	if p.PlanID == "" || p.TenantID == "" || p.ScheduledDate == "" {
		return errors.New("videoplan: plan_id/tenant_id/scheduled_date 必填")
	}
	// Canvas 冻结常量。
	if p.Canvas.W != 1080 || p.Canvas.H != 1920 {
		return fmt.Errorf("videoplan: canvas 必须 1080×1920，得到 %d×%d", p.Canvas.W, p.Canvas.H)
	}
	if p.Canvas.FPS != 25 && p.Canvas.FPS != 30 {
		return fmt.Errorf("videoplan: canvas.fps 必须 ∈{25,30}，得到 %d", p.Canvas.FPS)
	}
	// Timebase 冗余存储必须与 canvas 一致（禁 float seconds 进入时基）。
	if p.Timebase.Unit != "frame" {
		return fmt.Errorf("videoplan: timebase.unit 必须 frame（禁 float seconds），得到 %q", p.Timebase.Unit)
	}
	if p.Timebase.Rate != p.Canvas.FPS {
		return fmt.Errorf("videoplan: timebase.rate(%d) ≠ canvas.fps(%d)", p.Timebase.Rate, p.Canvas.FPS)
	}
	if err := p.BeatSchemaRef.Validate(); err != nil {
		return fmt.Errorf("videoplan: beat_schema_ref: %w", err)
	}
	if err := p.StyleThemeRef.Validate(); err != nil {
		return fmt.Errorf("videoplan: style_theme_ref: %w", err)
	}
	if err := p.Provenance.Validate(); err != nil {
		return fmt.Errorf("videoplan: provenance: %w", err)
	}
	if p.Budget.PlannedCostCents < 0 || p.Budget.LLMCalls < 0 || p.Budget.RenderCount < 0 {
		return errors.New("videoplan: budget 不得为负")
	}
	if err := p.validateTracks(); err != nil {
		return err
	}
	if err := p.validateCopy(); err != nil {
		return err
	}
	if err := p.validateAudio(); err != nil {
		return err
	}
	if err := p.validateOverlays(); err != nil {
		return err
	}
	return p.validateCompliance()
}

// validateCompliance 强制 IV-VP-5：aigc_disclosure.required=true 时
// explicit_overlay_id 必须非空且指向 aigc.disclosure 组件的 overlay。
func (p Plan) validateCompliance() error {
	c := p.Compliance
	if c == nil { // Gate 未跑（QC 之前的状态），Delivery 前由 ComplianceGate 补写
		return nil
	}
	if !c.AIGCDisclosure.Required {
		return nil
	}
	id := c.AIGCDisclosure.ExplicitOverlayID
	if id == nil || *id == "" {
		return errors.New("videoplan: IV-VP-5 aigc_disclosure.required=true 但 explicit_overlay_id 为空")
	}
	for _, o := range p.Overlays {
		if o.OverlayID == *id {
			if o.Component != "aigc.disclosure" {
				return fmt.Errorf(
					"videoplan: IV-VP-5 explicit_overlay_id %q 指向 %q，必须指向 aigc.disclosure",
					*id, o.Component)
			}
			return nil
		}
	}
	return fmt.Errorf("videoplan: IV-VP-5 explicit_overlay_id %q 不存在于 overlays", *id)
}

// validateTracks 强制 IV-VP-1（帧数守恒）与 IV-VP-2（clip 不越界）。
func (p Plan) validateTracks() error {
	if len(p.Tracks) == 0 {
		return errors.New("videoplan: tracks 为空")
	}
	for ti, t := range p.Tracks {
		if t.TrackID == "" {
			return fmt.Errorf("videoplan: tracks[%d].track_id 必填", ti)
		}
		for ci, c := range t.Clips {
			where := fmt.Sprintf("tracks[%d](%s).clips[%d](%s)", ti, t.TrackID, ci, c.ClipID)
			// IV-VP-2：src 区间至少 1 帧、tl 区间严格递增。
			if c.SrcOut-c.SrcIn < 1 {
				return fmt.Errorf("videoplan: IV-VP-2 %s src_out-src_in=%d <1", where, c.SrcOut-c.SrcIn)
			}
			if c.TlEnd <= c.TlStart {
				return fmt.Errorf("videoplan: IV-VP-2 %s tl_end(%d) ≤ tl_start(%d)", where, c.TlEnd, c.TlStart)
			}
			if c.TlStart < 0 || c.SrcIn < 0 {
				return fmt.Errorf("videoplan: IV-VP-2 %s tl_start/src_in 不得为负", where)
			}
			// 工艺上限：加速播放最多 1.15x（Engineering_plan §S5 钉死）。
			if c.Speed != 0 && (c.Speed < 1 || c.Speed > 1.15) {
				return fmt.Errorf("videoplan: %s speed=%v 越界 [1,1.15]", where, c.Speed)
			}
		}
	}
	total := p.TotalFrames()
	if total < 1 {
		return errors.New("videoplan: 时间线为空（无任何 clip）")
	}
	// IV-VP-1 帧数守恒：VIDEO_MAIN 轨的 clips 无缝铺满 [0, total]
	// （起点为 0、相邻首尾相接、终于 total）；其余轨不得超出 total。
	var main *Track
	for i := range p.Tracks {
		if p.Tracks[i].Kind == TrackVideoMain {
			main = &p.Tracks[i]
			break
		}
	}
	if main == nil || len(main.Clips) == 0 {
		return errors.New("videoplan: IV-VP-1 缺少 VIDEO_MAIN 轨 clip")
	}
	ordered := append([]Clip(nil), main.Clips...)
	sort.SliceStable(ordered, func(i, j int) bool { // 稳定排序（clips 通常已有序）
		return ordered[i].TlStart < ordered[j].TlStart
	})
	if ordered[0].TlStart != 0 {
		return fmt.Errorf("videoplan: IV-VP-1 VIDEO_MAIN 首段 tl_start=%d ≠ 0", ordered[0].TlStart)
	}
	for i := 1; i < len(ordered); i++ {
		if ordered[i].TlStart != ordered[i-1].TlEnd {
			return fmt.Errorf("videoplan: IV-VP-1 VIDEO_MAIN [%d,%d) 与 [%d,%d) 不无缝",
				ordered[i-1].TlStart, ordered[i-1].TlEnd, ordered[i].TlStart, ordered[i].TlEnd)
		}
	}
	if last := ordered[len(ordered)-1]; last.TlEnd != total {
		return fmt.Errorf("videoplan: IV-VP-1 VIDEO_MAIN 末段终于 %d ≠ 总时长 %d（尾洞）",
			last.TlEnd, total)
	}
	for ti, t := range p.Tracks {
		for ci, c := range t.Clips {
			if c.TlEnd > total {
				return fmt.Errorf(
					"videoplan: IV-VP-1 tracks[%d].clips[%d] tl_end(%d) 超出总时长(%d)",
					ti, ci, c.TlEnd, total)
			}
		}
	}
	return nil
}

// validateCopy 校验字幕区间落在总时长内、文本长度与词时间戳单调。
func (p Plan) validateCopy() error {
	total := p.TotalFrames()
	for i, b := range p.Copy.CaptionBlocks {
		if b.BlockID == "" {
			return fmt.Errorf("videoplan: caption_blocks[%d].block_id 必填", i)
		}
		if len([]rune(b.Text)) > 60 {
			return fmt.Errorf("videoplan: caption_blocks[%d] 文本超 60 字符", i)
		}
		if b.StartFrame < 0 || b.EndFrame <= b.StartFrame {
			return fmt.Errorf("videoplan: caption_blocks[%d] 帧区间非法 [%d,%d]",
				i, b.StartFrame, b.EndFrame)
		}
		if b.EndFrame > total { // IV-VP-3 的字幕半边
			return fmt.Errorf("videoplan: IV-VP-3 caption_blocks[%d] end(%d) 超总时长(%d)",
				i, b.EndFrame, total)
		}
		for j, w := range b.WordTimings {
			if w.S < b.StartFrame || w.E > b.EndFrame || w.E <= w.S {
				return fmt.Errorf("videoplan: caption_blocks[%d].word_timings[%d] 越出字幕区间",
					i, j)
			}
		}
	}
	return nil
}

// validateAudio 校验授权凭证与非 PLATFORM_LIBRARY 曲目的 proof 要求。
func (p Plan) validateAudio() error {
	if p.Audio.MusicRef != nil {
		m := p.Audio.MusicRef
		switch m.LicenseKind {
		case "PLATFORM_LIBRARY", "COMMERCIAL", "CC0":
		default:
			return fmt.Errorf("videoplan: music_ref.license_kind 非法 %q", m.LicenseKind)
		}
		// DB CHECK 约束的契约层镜像：非平台曲库必须携带授权凭证。
		if m.LicenseKind != "PLATFORM_LIBRARY" && m.LicenseProofURI == "" {
			return errors.New("videoplan: 非 PLATFORM_LIBRARY 音轨必须 license_proof_uri")
		}
		if m.ID == "" || m.Version < 1 {
			return errors.New("videoplan: music_ref 必须 id + version≥1")
		}
	}
	return nil
}

// validateOverlays 强制 IV-VP-3（区间 + safe_area）与 IV-VP-4（组件白名单）。
func (p Plan) validateOverlays() error {
	total := p.TotalFrames()
	for i, o := range p.Overlays {
		if o.EndFrame <= o.StartFrame {
			return fmt.Errorf("videoplan: overlays[%d] 帧区间非法 [%d,%d)",
				i, o.StartFrame, o.EndFrame)
		}
		// IV-VP-3：帧区间落在总时长内。
		if o.EndFrame > total || o.StartFrame < 0 {
			return fmt.Errorf("videoplan: IV-VP-3 overlays[%d] 帧区间 [%d,%d) 越出 [0,%d]",
				i, o.StartFrame, o.EndFrame, total)
		}
		// IV-VP-4：component 必须在渲染组件注册表白名单。
		if !overlayComponentWhitelist[o.Component] {
			return fmt.Errorf("videoplan: IV-VP-4 overlays[%d].component %q 不在注册表",
				i, o.Component)
		}
		// IV-VP-3：layout_box 不越 safe_area（渲染器 R-3 复检，这里先拒）。
		if err := p.checkSafeArea(o); err != nil {
			return fmt.Errorf("videoplan: IV-VP-3 overlays[%d]: %w", i, err)
		}
	}
	return nil
}

// checkSafeArea 按 anchor 语义展开 layout_box 并对照 safe_area 边界。
func (p Plan) checkSafeArea(o Overlay) error {
	x, y := o.LayoutBox.X, o.LayoutBox.Y
	// anchor 决定 (x,y) 是盒子的哪个角；统一换算成左上角再判界。
	switch o.LayoutBox.Anchor {
	case "TL":
	case "TC":
		x -= o.LayoutBox.W / 2
	case "TR":
		x -= o.LayoutBox.W
	case "CL":
		y -= o.LayoutBox.H / 2
	case "CC":
		x -= o.LayoutBox.W / 2
		y -= o.LayoutBox.H / 2
	case "CR":
		x -= o.LayoutBox.W
		y -= o.LayoutBox.H / 2
	case "BL":
		y -= o.LayoutBox.H
	case "BC":
		x -= o.LayoutBox.W / 2
		y -= o.LayoutBox.H
	case "BR":
		x -= o.LayoutBox.W
		y -= o.LayoutBox.H
	default:
		return fmt.Errorf("anchor 非法 %q", o.LayoutBox.Anchor)
	}
	sa := p.Canvas.SafeArea
	if x < sa.Left || y < sa.Top ||
		x+o.LayoutBox.W > p.Canvas.W-sa.Right ||
		y+o.LayoutBox.H > p.Canvas.H-sa.Bottom {
		return fmt.Errorf("layout_box(%d,%d,%d,%d anchor=%s) 越 safe_area(l=%d,t=%d,r=%d,b=%d)",
			x, y, o.LayoutBox.W, o.LayoutBox.H, o.LayoutBox.Anchor,
			sa.Left, sa.Top, sa.Right, sa.Bottom)
	}
	return nil
}
