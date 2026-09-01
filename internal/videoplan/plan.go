// Package videoplan 是 VideoPlan IR v1（schema/entities/video_plan.schema.json）
// 的 Go 实体层：生成编排（internal/eval）的时间线产物，Compiler（C3）
// 的输入。结构体与 schema 字段一一对应（json tag 对齐），漂移由 round-trip
// 测试发现；跨字段不变式 IV-VP-1..5 在本包 Validate 强制。
package videoplan

import (
	"fmt"
)

// SafeArea 是平台 UI 遮挡硬约束（像素单位）。
type SafeArea struct {
	Top    int `json:"top"`
	Bottom int `json:"bottom"`
	Left   int `json:"left"`
	Right  int `json:"right"`
}

// Canvas 画布常量：1080×1920，fps ∈ {25,30}（schema const/enum 冻结）。
type Canvas struct {
	W        int      `json:"w"`
	H        int      `json:"h"`
	FPS      int      `json:"fps"`
	SafeArea SafeArea `json:"safe_area"`
}

// Timebase 全系统整数帧时基（unit=frame 冻结；rate == canvas.fps 冗余存储）。
type Timebase struct {
	Unit string `json:"unit"`
	Rate int    `json:"rate"`
}

// ClipSource 钉死媒体版本：素材重转码后 content_hash 变化，
// 旧 plan 显式失效而非静默渲出不同结果。
type ClipSource struct {
	Kind        string `json:"kind"` // SHOT | GRAPHIC | GENERATED | COLOR
	Ref         string `json:"ref"`  // shot_id / graphic_id / color hex
	ContentHash string `json:"content_hash,omitempty"`
}

// Crop 是源内裁切矩形（像素）。
type Crop struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

// Position 是画布内落位（像素）。
type Position struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// KenBurns 是缓动推拉（from/to 为 [scale, dx, dy] 参数组）。
type KenBurns struct {
	From   []float64 `json:"from"`
	To     []float64 `json:"to"`
	Easing string    `json:"easing"` // LINEAR | EASE_IN_OUT
}

// Transform 是 clip 的空间变换（必含 crop/scale/position）。
type Transform struct {
	Crop     *Crop     `json:"crop"`
	Scale    float64   `json:"scale"`
	Position *Position `json:"position"`
	KenBurns *KenBurns `json:"ken_burns,omitempty"`
}

// TransitionIn 是入场转场。
type TransitionIn struct {
	Kind           string `json:"kind"` // CUT|FADE|WHIP|ZOOM_PUNCH|WIPE
	DurationFrames int    `json:"duration_frames"`
}

// ColorCorrection 是调色参数。
type ColorCorrection struct {
	LUTID      *string `json:"lut_id,omitempty"`
	Exposure   float64 `json:"exposure,omitempty"`
	Saturation float64 `json:"saturation,omitempty"`
}

// Clip 是时间线上的一段：timeline 区间 [tl_start, tl_end) 对应
// 源区间 [src_in, src_out)。全部整数帧，禁 float seconds。
type Clip struct {
	ClipID          string           `json:"clip_id"`
	BeatRole        string           `json:"beat_role"` // vocab/beat_role 7 值冻结表
	Source          ClipSource       `json:"source"`
	SrcIn           int              `json:"src_in"`
	SrcOut          int              `json:"src_out"`
	TlStart         int              `json:"tl_start"`
	TlEnd           int              `json:"tl_end"`
	Speed           float64          `json:"speed,omitempty"` // [1, 1.15] 工艺上限
	Transform       Transform        `json:"transform"`
	Color           *ColorCorrection `json:"color,omitempty"`
	TransitionIn    TransitionIn     `json:"transition_in"`
	AudioFromSource bool             `json:"audio_from_source,omitempty"`
}

// TrackKind 轨道类型（schema enum 冻结）。
type TrackKind string

const (
	TrackVideoMain   TrackKind = "VIDEO_MAIN"
	TrackVideoInsert TrackKind = "VIDEO_INSERT"
	TrackOverlayRend TrackKind = "OVERLAY_RENDER"
	TrackAudioVO     TrackKind = "AUDIO_VO"
	TrackAudioMusic  TrackKind = "AUDIO_MUSIC"
	TrackAudioSFX    TrackKind = "AUDIO_SFX"
)

// Track 是同类 clip 的有序容器。
type Track struct {
	TrackID string    `json:"track_id"`
	Kind    TrackKind `json:"kind"`
	Clips   []Clip    `json:"clips"`
}

// WordTiming 是词级时间戳（帧单位，来自强制对齐算子）。
type WordTiming struct {
	W string `json:"w"`
	S int    `json:"s"`
	E int    `json:"e"`
}

// CaptionBlock 是一段字幕（含词级对齐与强调词）。
type CaptionBlock struct {
	BlockID      string       `json:"block_id"`
	Text         string       `json:"text"` // ≤60 字符
	StartFrame   int          `json:"start_frame"`
	EndFrame     int          `json:"end_frame"`
	CopyFunction string       `json:"copy_function"`
	WordTimings  []WordTiming `json:"word_timings"`
	Emphasis     []int        `json:"emphasis,omitempty"`
}

// Copy 是文案块：屏幕字幕 + 发布文案 + 话题标签。
type Copy struct {
	CaptionBlocks []CaptionBlock `json:"caption_blocks"`
	PostText      string         `json:"post_text"`
	Hashtags      []string       `json:"hashtags"`
}

// LicensedRef 携带授权凭证的媒体引用（schema/common/licensed_ref.json）。
// 非 PLATFORM_LIBRARY 必须 LicenseProofURI（授权合规硬约束）。
type LicensedRef struct {
	ID              string `json:"id"`
	Version         int    `json:"version"`
	LicenseKind     string `json:"license_kind"` // PLATFORM_LIBRARY|COMMERCIAL|CC0
	LicenseProofURI string `json:"license_proof_uri,omitempty"`
}

// VORef 是 TTS 产物引用（内容寻址）。
type VORef struct {
	TTSID string `json:"tts_id"`
	Hash  string `json:"hash"`
}

// Ducking 是音乐闪避参数。
type Ducking struct {
	Enabled  *bool   `json:"enabled,omitempty"`
	AmountDB float64 `json:"amount_db,omitempty"`
}

// Audio 是音频总线：目标响度 / 音轨引用 / 闪避 / 卡点网格。
type Audio struct {
	TargetLUFS float64      `json:"target_lufs"`
	MusicRef   *LicensedRef `json:"music_ref"` // nil = 静音 + 卡点标记模式
	VORef      *VORef       `json:"vo_ref"`    // nil = 静音模式
	Ducking    Ducking      `json:"ducking"`
	BeatGrid   []int        `json:"beat_grid,omitempty"`
}

// LayoutBox 是 overlay 落位矩形 + 锚点（IV-VP-3 判据）。
type LayoutBox struct {
	X      int    `json:"x"`
	Y      int    `json:"y"`
	W      int    `json:"w"`
	H      int    `json:"h"`
	Anchor string `json:"anchor"` // TL|TC|TR|CL|CC|CR|BL|BC|BR
}

// Overlay 是一个渲染组件实例（IV-VP-4：component 必须在注册表白名单）。
type Overlay struct {
	OverlayID  string         `json:"overlay_id"`
	Intent     string         `json:"intent"` // vocab/overlay_intent
	Component  string         `json:"component"`
	Props      map[string]any `json:"props"`
	StartFrame int            `json:"start_frame"`
	EndFrame   int            `json:"end_frame"`
	LayoutBox  LayoutBox      `json:"layout_box"`
}

// FallbackUsed 记录一次降级（constraints_report.fallbacks_used）。
type FallbackUsed struct {
	SlotID string `json:"slot_id,omitempty"`
	Level  int    `json:"level,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// RejectedCandidate 记录一个被淘汰的候选与规则。
type RejectedCandidate struct {
	ShotID string `json:"shot_id,omitempty"`
	Rule   string `json:"rule,omitempty"`
}

// ConstraintsReport 是求解过程可解释性记录——
// 调试"为什么今天这条这么烂"的唯一途径。
type ConstraintsReport struct {
	HardSatisfied      []string            `json:"hard_satisfied"`
	SoftScores         map[string]float64  `json:"soft_scores"`
	FallbacksUsed      []FallbackUsed      `json:"fallbacks_used"`
	RejectedCandidates []RejectedCandidate `json:"rejected_candidates"`
}

// DiversitySignature 是多样性判重的依据（§8.2 叙事疲劳防线）。
type DiversitySignature struct {
	Structural       string   `json:"structural"` // IV-BS-3 结构签名
	VisualPhashFirst string   `json:"visual_phash_first"`
	CopyNgrams       []string `json:"copy_ngrams"`
	MusicID          string   `json:"music_id"`
	StyleID          string   `json:"style_id"`
	ShotIDs          []string `json:"shot_ids,omitempty"`
}

// Budget 是成本预算的申报块。
type Budget struct {
	PlannedCostCents int     `json:"planned_cost_cents"`
	LLMCalls         int     `json:"llm_calls"`
	GPUSeconds       float64 `json:"gpu_seconds"`
	RenderCount      int     `json:"render_count"`
}

// AIGCDisclosure 是 AIGC 标识义务的回写块（common/compliance_result 局部）。
// IV-VP-5：required=true 时 explicit_overlay_id 必须指向 aigc.disclosure overlay。
type AIGCDisclosure struct {
	Required          bool           `json:"required"`
	ExplicitOverlayID *string        `json:"explicit_overlay_id"`
	ImplicitMetadata  map[string]any `json:"implicit_metadata"`
}

// ComplianceResult 是 ComplianceGate 执行结果的回写块——
// QC 之后、Delivery 之前的唯一强制门禁留下的一致性凭证。
type ComplianceResult struct {
	AIGCDisclosure AIGCDisclosure `json:"aigc_disclosure"`
	ChecksPassed   []string       `json:"checks_passed"`
}

// Plan 是单条视频的语义 IR——系统的心脏：
// 语义完整（能回答"为什么这么剪"）+ 确定性（同一 IR 永远渲出同一帧）+ 可 diff。
type Plan struct {
	SchemaVersion      string             `json:"schema_version"` // "video_plan/1"
	PlanID             string             `json:"plan_id"`        // UUIDv7
	TenantID           string             `json:"tenant_id"`
	ScheduledDate      string             `json:"scheduled_date"` // YYYY-MM-DD
	Canvas             Canvas             `json:"canvas"`
	Timebase           Timebase           `json:"timebase"`
	BeatSchemaRef      VersionedRef       `json:"beat_schema_ref"`
	StyleThemeRef      VersionedRef       `json:"style_theme_ref"`
	Tracks             []Track            `json:"tracks"`
	Copy               Copy               `json:"copy"`
	Audio              Audio              `json:"audio"`
	Overlays           []Overlay          `json:"overlays"`
	ConstraintsReport  ConstraintsReport  `json:"constraints_report"`
	DiversitySignature DiversitySignature `json:"diversity_signature"`
	Budget             Budget             `json:"budget"`
	Compliance         *ComplianceResult  `json:"compliance,omitempty"` // ComplianceGate 回写
	Provenance         Provenance         `json:"provenance"`
}

// TotalFrames 返回主时间线总时长（IV-VP-1 的权威值）。
func (p Plan) TotalFrames() int {
	total := 0
	for _, t := range p.Tracks {
		for _, c := range t.Clips {
			if c.TlEnd > total {
				total = c.TlEnd
			}
		}
	}
	return total
}

// Describe 返回人类可读摘要（日志/调试用，保持确定性）。
func (p Plan) Describe() string {
	return fmt.Sprintf("plan=%s date=%s schema=%s clips=%d overlays=%d total=%df",
		p.PlanID, p.ScheduledDate, p.BeatSchemaRef.ID, p.clipCount(), len(p.Overlays), p.TotalFrames())
}

func (p Plan) clipCount() int {
	n := 0
	for _, t := range p.Tracks {
		n += len(t.Clips)
	}
	return n
}
