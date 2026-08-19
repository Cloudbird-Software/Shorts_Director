package entity

import (
	"fmt"
	"time"
)

// ShotState 是 Shot 生命周期的显式 FSM 状态（禁止布尔字段拼装）。
type ShotState string

const (
	ShotUploaded   ShotState = "UPLOADED"
	ShotSegmented  ShotState = "SEGMENTED"
	ShotTagged     ShotState = "TAGGED"
	ShotRejected   ShotState = "REJECTED"
	ShotAvailable  ShotState = "AVAILABLE"
	ShotCooling    ShotState = "COOLING"
	ShotExpired    ShotState = "EXPIRED"
	ShotQuarantine ShotState = "QUARANTINED"
)

// shotTransitions 是唯一合法状态迁移表：
// UPLOADED→SEGMENTED→TAGGED→(REJECTED|AVAILABLE)；AVAILABLE⇄COOLING；
// AVAILABLE→EXPIRED(ttl)/QUARANTINED(合规)；REJECTED 可经重拍回归 UPLOADED。
var shotTransitions = map[ShotState][]ShotState{
	ShotUploaded:   {ShotSegmented},
	ShotSegmented:  {ShotTagged},
	ShotTagged:     {ShotRejected, ShotAvailable},
	ShotRejected:   {ShotUploaded}, // 重拍令完成后按新素材回归
	ShotAvailable:  {ShotCooling, ShotExpired, ShotQuarantine},
	ShotCooling:    {ShotAvailable},
	ShotExpired:    {},
	ShotQuarantine: {ShotAvailable}, // 人工放行或打码完成
}

// CanTransition 报告状态迁移是否合法（FSM 唯一判据）。
func CanTransition(from, to ShotState) bool {
	for _, t := range shotTransitions[from] {
		if t == to {
			return true
		}
	}
	return false
}

// ShotIdentity 是剪辑单元的时间标识（全系统整数帧，禁 float seconds）。
type ShotIdentity struct {
	InFrame        int `json:"in_frame"`                  // ≥0
	OutFrame       int `json:"out_frame"`                 // ≥1
	FPS            int `json:"fps"`                       // ∈{25,30}
	DurationFrames int `json:"duration_frames,omitempty"` // 生成列 = out-in
}

// ShotSemantic 是受控词表语义标签（全部 vocab id）。
type ShotSemantic struct {
	ShotType         string   `json:"shot_type,omitempty"` // vocab/shot_type
	ShotTypeClasses  []string `json:"shot_type_classes,omitempty"`
	CameraMotionType string   `json:"camera_motion_type,omitempty"` // vocab/camera_motion
	CameraMotionDir  *string  `json:"camera_motion_dir"`            // STATIC 时为 null
	Scene            string   `json:"scene,omitempty"`              // vocab/scene.<vertical>
	Subjects         []string `json:"subjects,omitempty"`           // ≤5，按显著性降序
	Actions          []string `json:"actions,omitempty"`            // vocab/action
	Mood             []string `json:"mood,omitempty"`
}

// BBox 是主体包围盒轨迹采样点（帧号 + 像素矩形）。
type BBox struct {
	Frame int `json:"frame"`
	X     int `json:"x"`
	Y     int `json:"y"`
	W     int `json:"w"`
	H     int `json:"h"`
}

// SafeCrop 声明 9:16 竖版裁切可行性（IV-SH-1 判据）。
type SafeCrop struct {
	OK     bool    `json:"ok"`               // 假阳性率必须 <5%（断头比缺素材严重）
	Method *string `json:"method,omitempty"` // CENTER_CROP|RECENTER|PILLARBOX|NONE
}

// CameraMotion 是运镜描述（vocab/camera_motion + 方向 + 置信度）。
type CameraMotion struct {
	Type       *string  `json:"type,omitempty"` // vocab/camera_motion
	Dir        *string  `json:"dir"`            // LEFT/RIGHT/IN/OUT/UP/DOWN/null
	Confidence *float64 `json:"confidence,omitempty"`
}

// ShotAffordance 描述 shot 能被怎样使用（可剪辑性）。
type ShotAffordance struct {
	IsLoopable       *bool         `json:"is_loopable,omitempty"`
	CleanIn          *bool         `json:"clean_in,omitempty"`
	CleanOut         *bool         `json:"clean_out,omitempty"`
	CameraMotion     *CameraMotion `json:"camera_motion,omitempty"`
	NegativeSpace    []string      `json:"negative_space,omitempty"` // TOP/BOTTOM/LEFT/RIGHT
	SubjectBBoxTrack []BBox        `json:"subject_bbox_track,omitempty"`
	SafeCrop9x16     *SafeCrop     `json:"safe_crop_9x16"`
	HasSpeech        *bool         `json:"has_speech,omitempty"`
	HasLipsync       *bool         `json:"has_lipsync,omitempty"`
	MotionEnergy     *float64      `json:"motion_energy,omitempty"` // 0–1
}

// AudioMetrics 是音频技术指标。
type AudioMetrics struct {
	LUFS *float64 `json:"lufs,omitempty"`
	SNR  *float64 `json:"snr,omitempty"`
}

// ShotTechnical 是确定性算法测得的指标（VLM 禁止重新判断，B-2）。
type ShotTechnical struct {
	Sharpness    *float64      `json:"sharpness,omitempty"` // laplacian_var
	NIQE         *float64      `json:"niqe,omitempty"`
	ShakeScore   *float64      `json:"shake_score,omitempty"`
	ExposureHist []int         `json:"exposure_hist,omitempty"`
	ColorTemp    *int          `json:"color_temp,omitempty"`
	FlickerScore *float64      `json:"flicker_score,omitempty"`
	Audio        *AudioMetrics `json:"audio,omitempty"`
	QualityTier  *int          `json:"quality_tier,omitempty"` // 1–4
}

// ShotCompliance 是合规扫描结果（IV-SH-2 判据）。
type ShotCompliance struct {
	ThirdPartyFaces []string `json:"third_party_faces,omitempty"`
	ThirdPartyLogos []string `json:"third_party_logos,omitempty"`
	Plates          []string `json:"plates,omitempty"`
	OCRText         []string `json:"ocr_text,omitempty"`
	RiskFlags       []string `json:"risk_flags,omitempty"` // vocab/compliance_risk
}

// ShotLifecycle 是复用与时效策略。
type ShotLifecycle struct {
	ShotDate       *string  `json:"shot_date,omitempty"` // YYYY-MM-DD
	Seasons        []string `json:"seasons,omitempty"`   // vocab/season
	TTLClass       *string  `json:"ttl_class,omitempty"` // vocab/ttl_class
	TTLAt          *string  `json:"ttl_at,omitempty"`    // YYYY-MM-DD
	LinkedSKUs     []string `json:"linked_skus,omitempty"`
	LinkedCampaign []string `json:"linked_campaigns,omitempty"`
	UseCount       int      `json:"use_count,omitempty"`
	LastUsedAt     *string  `json:"last_used_at,omitempty"` // YYYY-MM-DD
}

// Shot 是可用的最小剪辑单元（L2 物料层，schema/entities/shot.json）。
type Shot struct {
	ID            string         `json:"id"` // UUIDv7
	AssetID       string         `json:"asset_id"`
	TenantID      string         `json:"tenant_id"`
	State         ShotState      `json:"state"`
	Identity      ShotIdentity   `json:"identity"`
	Semantic      ShotSemantic   `json:"semantic"`
	Affordance    ShotAffordance `json:"affordance"`
	Technical     ShotTechnical  `json:"technical"`
	Compliance    ShotCompliance `json:"compliance"`
	Lifecycle     ShotLifecycle  `json:"lifecycle"`
	TagProvenance Provenance     `json:"tag_provenance"`
}

// Validate 校验 Shot 的跨字段不变式（IV-SH-*）与类型级约束。
func (s Shot) Validate() error {
	if s.ID == "" || s.AssetID == "" || s.TenantID == "" {
		return fmt.Errorf("entity/shot: id/asset_id/tenant_id 必填")
	}
	switch s.State {
	case ShotUploaded, ShotSegmented, ShotTagged, ShotRejected,
		ShotAvailable, ShotCooling, ShotExpired, ShotQuarantine:
	default:
		return fmt.Errorf("entity/shot: state 非法 %q", s.State)
	}
	if s.Identity.OutFrame <= s.Identity.InFrame {
		return fmt.Errorf("entity/shot: out_frame(%d) 必须 > in_frame(%d)",
			s.Identity.OutFrame, s.Identity.InFrame)
	}
	if s.Identity.FPS != 25 && s.Identity.FPS != 30 {
		return fmt.Errorf("entity/shot: fps 必须 ∈{25,30}，得到 %d", s.Identity.FPS)
	}
	if d := s.Identity.OutFrame - s.Identity.InFrame; s.Identity.DurationFrames != 0 &&
		s.Identity.DurationFrames != d {
		return fmt.Errorf("entity/shot: duration_frames(%d) ≠ out-in(%d)",
			s.Identity.DurationFrames, d)
	}
	if len(s.Semantic.Subjects) > 5 {
		return fmt.Errorf("entity/shot: subjects 超 maxItems=5（%d）", len(s.Semantic.Subjects))
	}
	if err := s.TagProvenance.Validate(); err != nil {
		return fmt.Errorf("entity/shot: %w", err)
	}
	// IV-SH-2：risk_flags 非空 ⇒ 不得处于可用态（须 QUARANTINED，直到人工放行）。
	if len(s.Compliance.RiskFlags) > 0 && s.State == ShotAvailable {
		return fmt.Errorf("entity/shot: IV-SH-2 违反——risk_flags 非空时状态不得为 AVAILABLE")
	}
	return nil
}

// PillarboxDeclared 报告是否声明了 PILLARBOX 处理（IV-SH-1 的豁免条件）。
func (s Shot) PillarboxDeclared() bool {
	sc := s.Affordance.SafeCrop9x16
	return sc != nil && !sc.OK && sc.Method != nil && *sc.Method == "PILLARBOX"
}

// EligibleForVertical 报告 shot 能否进入 9:16 slot 候选池（IV-SH-1）。
// safe_crop_9x16.ok=false 且未声明 pillarbox ⇒ 不可入选。
func (s Shot) EligibleForVertical() bool {
	sc := s.Affordance.SafeCrop9x16
	if sc == nil || sc.OK {
		return true
	}
	return s.PillarboxDeclared()
}

// IsConsumable 报告 shot 当前是否可作为候选（状态 + 合规 + 时效的合取）。
// IV-SH-3：ttl_at < now ⇒ 自动出候选池。
func (s Shot) IsConsumable(now time.Time) bool {
	if s.State != ShotAvailable {
		return false
	}
	if len(s.Compliance.RiskFlags) > 0 {
		return false
	}
	if s.Lifecycle.TTLAt != nil {
		if ttl, err := time.Parse("2006-01-02", *s.Lifecycle.TTLAt); err == nil && ttl.Before(now) {
			return false
		}
	}
	return true
}
