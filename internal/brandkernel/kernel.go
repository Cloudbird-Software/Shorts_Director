// Package brandkernel 是 BrandKernel（L4 意图层根契约）的 Go 实体层。
// 结构体与 schema/entities/brand_kernel.schema.json 字段一一对应（json tag
// 对齐，漂移由 round-trip 测试发现）；跨字段不变式 IV-BK-1..3 在此运行期校验。
// JSON Schema 结构校验仍由 TS 侧 G1 harness 负责，本包只做"结构合法之后"
// 的业务不变式——边界同 internal/entity。
package brandkernel

import (
	"fmt"

	"github.com/Cloudbird-Software/Shorts_Director/internal/contracts"
	"github.com/Cloudbird-Software/Shorts_Director/internal/entity"
)

// L3MatchThreshold 是进入 L3 匹配的完备度门槛（IV-BK-2，
// 苏格拉底问答的停止条件=槽位覆盖度而非轮次，Engineering_plan §1.2）。
const L3MatchThreshold = 0.75

// Category 是 business_category 受控枚举的临时 Go 侧锚点。
// 词表尚未冻结（schema 描述："后续 PR 冻结"）；冻结后本集合删除，
// 校验切换为 vocab.IsVocabID("business_category", ...)。IV-BK-3。
type Category string

const (
	CategoryFoodNoodle Category = "FOOD_NOODLE"
)

// categories 是 IV-BK-3 的受控枚举（临时，见 Category 文档）。
var categories = map[Category]bool{
	CategoryFoodNoodle: true,
}

// ValidCategories 返回当前受控枚举快照（测试与报批用）。
func ValidCategories() []Category { return []Category{CategoryFoodNoodle} }

// Differentiator 是身份差异点（必须可视觉验证，proof_types 非空）。
type Differentiator struct {
	Claim          string   `json:"claim"` // ≤30 字符
	VisualProvable bool     `json:"visual_provable"`
	ProofTypes     []string `json:"proof_types"`           // vocab/proof_type
	VerifiedAt     *string  `json:"verified_at,omitempty"` // 事实漂移防线（§8.2-①）
}

// Persona 是口播人格（驱动 TTS 与文风）。
type Persona struct {
	VoiceTone    string `json:"voice_tone,omitempty"`    // WARM|EXPERT|STREET|PLAYFUL|CALM
	SpeakingRate string `json:"speaking_rate,omitempty"` // SLOW|NORMAL|FAST
	FirstPerson  string `json:"first_person,omitempty"`  // OWNER|STAFF|BRAND|CUSTOMER
	DialectHint  string `json:"dialect_hint,omitempty"`
}

// Identity 是商家身份块。
type Identity struct {
	Name            string           `json:"name"`
	OneLiner        string           `json:"one_liner"` // ≤40 字符
	Differentiators []Differentiator `json:"differentiators"`
	Persona         Persona          `json:"persona"`
}

// Segment 是受众分层。
type Segment struct {
	Label     string `json:"label"`
	Pain      string `json:"pain"`
	Objection string `json:"objection"` // 抗拒点，驱动 CONTRAST beat
}

// Audience 是本地客群画像。
type Audience struct {
	Segments        []Segment `json:"segments"`         // 1–3
	LocalRadiusKm   float64   `json:"local_radius_km"`  // >0
	DecisionTrigger string    `json:"decision_trigger"` // IMPULSE|PLANNED|REFERRAL|SEASONAL
}

// Pillar 是内容支柱（IV-BK-1 的主体）。
type Pillar struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	Intent      string   `json:"intent"`       // AWARENESS|TRUST|DESIRE|CONVERSION|RETENTION
	ProofTypes  []string `json:"proof_types"`  // vocab/proof_type，≥2（IV-BK-1）
	TargetRatio float64  `json:"target_ratio"` // ∈[0.05,0.6]
}

// DigitalHuman 是数字人授权声明。
type DigitalHuman struct {
	Enabled         bool    `json:"enabled"`
	Source          *string `json:"source"`           // REAL_OWNER|REAL_STAFF|STOCK_AVATAR|null
	AuthorizationID *string `json:"authorization_id"` // 真人授权令，enabled=true 时必填
}

// AssetsIntent 是素材意图清单。
type AssetsIntent struct {
	ShootableScenes   []string     `json:"shootable_scenes"` // vocab/scene.<vertical>
	NonShootableNeeds []string     `json:"non_shootable_needs"`
	DigitalHuman      DigitalHuman `json:"digital_human"`
}

// Offer 是可轮换的 OFFER 池条目（价格/活动事实最易漂移）。
type Offer struct {
	Text       string  `json:"text"`
	ValidFrom  string  `json:"valid_from"` // date
	ValidTo    string  `json:"valid_to"`   // date
	ClaimRisk  string  `json:"claim_risk"` // LOW|MEDIUM|HIGH
	VerifiedAt *string `json:"verified_at,omitempty"`
}

// Completeness 是访谈完备度（IV-BK-2 的判据来源）。
type Completeness struct {
	Score          float64  `json:"score"` // ∈[0,1]
	MissingSlots   []string `json:"missing_slots"`
	InterviewTurns int      `json:"interview_turns"` // ≤18
}

// BrandKernel 是 Onboarding 苏格拉底问答的唯一产物、全系统根契约。
type BrandKernel struct {
	SchemaVersion       string             `json:"schema_version"` // const "brand_kernel/1"
	TenantID            string             `json:"tenant_id"`      // UUIDv7
	Category            Category           `json:"category"`       // IV-BK-3
	Identity            Identity           `json:"identity"`
	Audience            Audience           `json:"audience"`
	Pillars             []Pillar           `json:"pillars"` // IV-BK-1
	AssetsIntent        AssetsIntent       `json:"assets_intent"`
	Offers              []Offer            `json:"offers,omitempty"`
	HardNegatives       []string           `json:"hard_negatives,omitempty"`
	ComplianceProfileID string             `json:"compliance_profile_id"` // 由 category 决定（IV-BK-3）
	Completeness        Completeness       `json:"completeness"`
	Provenance          *entity.Provenance `json:"provenance,omitempty"` // G1 minimal 样本不带
}

// Validate 校验跨字段不变式 IV-BK-1..3 与类型级约束（结构校验归 TS G1）。
func (k BrandKernel) Validate() error {
	if k.SchemaVersion != contracts.SchemaVersion(contracts.SchemaBrandKernel) {
		return fmt.Errorf("brandkernel: schema_version 非法 %q", k.SchemaVersion)
	}
	if k.TenantID == "" {
		return fmt.Errorf("brandkernel: tenant_id 必填")
	}
	// IV-BK-1：≥3 个 ContentPillar，每个 pillar 绑定 ≥2 种 ProofType。
	if len(k.Pillars) < 3 {
		return fmt.Errorf("brandkernel: IV-BK-1 违反——pillars 至少 3 个，得到 %d", len(k.Pillars))
	}
	for i, p := range k.Pillars {
		if len(p.ProofTypes) < 2 {
			return fmt.Errorf("brandkernel: IV-BK-1 违反——pillar[%d] %q 绑定 proof_type %d 种（<2）",
				i, p.ID, len(p.ProofTypes))
		}
	}
	// IV-BK-3：category 必须来自受控枚举（临时集合，词表冻结后切换 vocab），
	// 并决定 compliance_profile_id——禁止 LLM 自由生成。
	if !categories[k.Category] {
		return fmt.Errorf("brandkernel: IV-BK-3 违反——category %q 不在受控枚举内", k.Category)
	}
	if k.ComplianceProfileID == "" {
		return fmt.Errorf("brandkernel: IV-BK-3 违反——category 须决定 compliance_profile_id，当前为空")
	}
	// IV-BK-2 的取值域部分（门槛判定见 ReadyForL3Matching）。
	if k.Completeness.Score < 0 || k.Completeness.Score > 1 {
		return fmt.Errorf("brandkernel: completeness.score ∈[0,1]，得到 %v", k.Completeness.Score)
	}
	if k.Completeness.InterviewTurns < 0 || k.Completeness.InterviewTurns > 18 {
		return fmt.Errorf("brandkernel: interview_turns ∈[0,18]，得到 %d", k.Completeness.InterviewTurns)
	}
	// 可视觉验证的主张必须绑定证明方式（不同iators proof_types ≥1）。
	for i, d := range k.Identity.Differentiators {
		if d.Claim == "" || (d.VisualProvable && len(d.ProofTypes) == 0) {
			return fmt.Errorf("brandkernel: differentiators[%d] 声明可视觉验证但未绑定 proof_type", i)
		}
	}
	if k.Provenance != nil {
		if err := k.Provenance.Validate(); err != nil {
			return fmt.Errorf("brandkernel: %w", err)
		}
	}
	return nil
}

// ReadyForL3Matching 报告是否达到进入 L3 匹配的停止条件（IV-BK-2：
// completeness.score ≥ 0.75——停止条件是槽位覆盖度而非访谈轮次）。
// 这是后续 S1 InterviewService / S4 ParadigmLibrary 的准入 API。
func (k BrandKernel) ReadyForL3Matching() error {
	if err := k.Validate(); err != nil {
		return err
	}
	if k.Completeness.Score < L3MatchThreshold {
		return fmt.Errorf("brandkernel: IV-BK-2 未达标——completeness.score %.2f < %.2f，访谈不得停止",
			k.Completeness.Score, L3MatchThreshold)
	}
	return nil
}
