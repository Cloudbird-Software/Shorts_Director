package compliance

import (
	"github.com/Cloudbird-Software/Shorts_Director/internal/videoplan"
)

// Input 聚合被检对象与全部外部事实。Gate 不查库、不取系统时间——
// 词库、类目策略、授权记录、shot 风险标记、当前日期都由调用方注入，
// 保证同一 Input 恒同一 GateResult（可重放审计）。
type Input struct {
	Plan     *videoplan.Plan
	Artifact *RenderArtifact

	// ---- 外部事实（均为确定性快照） ----
	Category       string              // 租户经营类目（如 food/medical/beauty）
	CategoryPolicy CategoryPolicy      // 类目准入策略
	BannedTerms    []BannedTerm        // 违禁词库（冷启动产物，§S8）
	ShotRiskFlags  map[string][]string // shot_id → entity.Shot.Compliance.RiskFlags
	Authorizations []Authorization     // 出镜/声音授权记录
	Now            string              // 判定基准日 YYYY-MM-DD（禁 time.Now）
}

// RenderArtifact 是被检渲染产物（隐式标识判定需要读回元数据）。
type RenderArtifact struct {
	Path     string            `json:"path"`
	Metadata map[string]string `json:"metadata"` // ffprobe 读回的全量元数据
}

// Authorization 是一条授权记录（肖像/声音）。
type Authorization struct {
	Kind       string `json:"kind"`        // PORTRAIT | VOICE
	SubjectID  string `json:"subject_id"`  // 出镜人 id / TTS 音色 id
	State      string `json:"state"`       // ACTIVE | REVOKED | EXPIRED
	ValidUntil string `json:"valid_until"` // YYYY-MM-DD；空 = 不限期
}

// ActiveOn 报告该授权在基准日是否有效。
func (a Authorization) ActiveOn(now string) bool {
	if a.State != "ACTIVE" {
		return false
	}
	if a.ValidUntil == "" {
		return true
	}
	return a.ValidUntil >= now // ISO 日期字典序即时间序
}

// BannedTerm 是违禁词库的一条记录（§S8 分级：BLOCKER/MAJOR/MINOR）。
type BannedTerm struct {
	Term     string `json:"term"`
	Risk     string `json:"risk"`     // vocab/compliance_risk id
	Severity string `json:"severity"` // BLOCKER | MAJOR | MINOR
}

// CategoryPolicy 是类目准入策略（MISSING_QUALIFICATION 的判据）。
type CategoryPolicy struct {
	Category              string   `json:"category"`
	Admission             string   `json:"admission"` // ALLOWED | REVIEW | REJECTED
	RequiredDisclaimers   []string `json:"required_disclaimers,omitempty"`
	RequiresQualification bool     `json:"requires_qualification,omitempty"`
	QualificationProof    string   `json:"qualification_proof,omitempty"` // 资质凭证 URI（REVIEW/REJECTED 类目）
}

// CopyTexts 汇出 plan 内全部受检文案（字幕/尾板/话题标签）。
func (in *Input) CopyTexts() []string {
	if in.Plan == nil {
		return nil
	}
	var out []string
	for _, b := range in.Plan.Copy.CaptionBlocks {
		out = append(out, b.Text)
	}
	out = append(out, in.Plan.Copy.PostText)
	out = append(out, in.Plan.Copy.Hashtags...)
	return out
}

// ReferencedShotIDs 汇出视频轨引用的全部 shot id（去重保序）。
func (in *Input) ReferencedShotIDs() []string {
	if in.Plan == nil {
		return nil
	}
	seen := map[string]bool{}
	var ids []string
	for _, t := range in.Plan.Tracks {
		if t.Kind != videoplan.TrackVideoMain && t.Kind != videoplan.TrackVideoInsert {
			continue
		}
		for _, c := range t.Clips {
			if c.Source.Kind == "SHOT" && !seen[c.Source.Ref] {
				seen[c.Source.Ref] = true
				ids = append(ids, c.Source.Ref)
			}
		}
	}
	return ids
}
