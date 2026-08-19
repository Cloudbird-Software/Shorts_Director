package compliance

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Cloudbird-Software/Shorts_Director/internal/videoplan"
)

const shotID = "018f6c01-aaaa-7aaa-8aaa-000000000002"

func loadPlan(t *testing.T) *videoplan.Plan {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "schema", "testdata", "video_plan", "valid", "with_vo_and_speed.json"))
	if err != nil {
		t.Fatalf("读样本失败: %v", err)
	}
	var p videoplan.Plan
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("解析 plan: %v", err)
	}
	return &p
}

// passingInput 返回能全绿通过 8 门的事实快照（with_vo_and_speed 样本：
// 含 TTS VO ⇒ AIGC 标识义务成立，需显式 overlay + 隐式元数据 + 声音授权）。
func passingInput(t *testing.T) *Input {
	t.Helper()
	p := loadPlan(t)
	EnsureAIGCDisclosure(p, "AI 生成内容")
	return &Input{
		Plan:           p,
		Artifact:       &RenderArtifact{Path: "/mnt/work/out/final.mp4", Metadata: BuildImplicitLabel("shorts-director", "2026-08-19T00:00:00Z", "plan-1")},
		Category:       "food",
		CategoryPolicy: CategoryPolicy{Category: "food", Admission: "ALLOWED"},
		BannedTerms: []BannedTerm{
			{Term: "全网第一", Risk: "AD_LAW_SUPERLATIVE", Severity: "BLOCKER"},
		},
		ShotRiskFlags: map[string][]string{},
		Authorizations: []Authorization{
			{Kind: "VOICE", SubjectID: "tts-001", State: "ACTIVE", ValidUntil: "2027-01-01"},
		},
		Now: "2026-08-19",
	}
}

// TestChainAllPass：完备事实 → PASS，8 门全部记入 checks_passed。
func TestChainAllPass(t *testing.T) {
	res := Chain(context.Background(), StandardGates(), passingInput(t))
	if res.Decision != DecisionPass {
		t.Fatalf("期望 PASS，得到 %s，findings=%+v", res.Decision, res.Findings)
	}
	if len(res.ChecksPassed) != 8 || len(res.Skipped) != 0 {
		t.Errorf("checks_passed=%v skipped=%v", res.ChecksPassed, res.Skipped)
	}
}

// TestChainDeterministic：同一 Input 两次执行恒同一输出。
func TestChainDeterministic(t *testing.T) {
	in := passingInput(t)
	a := Chain(context.Background(), StandardGates(), in)
	b := Chain(context.Background(), StandardGates(), in)
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if string(ja) != string(jb) {
		t.Error("两次 Chain 输出不一致：确定性破坏")
	}
}

// TestChainBlockShortCircuits：违禁词 BLOCKER ⇒ 停在 banned_terms，
// 后续 6 门记 skipped，Decision=BLOCK。
func TestChainBlockShortCircuits(t *testing.T) {
	in := passingInput(t)
	in.Plan.Copy.CaptionBlocks[0].Text = "全网第一的牛肉面"
	res := Chain(context.Background(), StandardGates(), in)
	if res.Decision != DecisionBlock {
		t.Fatalf("期望 BLOCK，得到 %s", res.Decision)
	}
	if len(res.Skipped) != 6 {
		t.Errorf("短路后应跳过 6 门，得到 %v", res.Skipped)
	}
	if !containsPassed(res.ChecksPassed, "category_admission") {
		t.Errorf("category_admission 应已通过: %v", res.ChecksPassed)
	}
	if containsPassed(res.ChecksPassed, "music_license") {
		t.Errorf("banned_terms 之后的门不应执行: %v", res.ChecksPassed)
	}
}

// TestCategoryAdmission：REJECTED→BLOCK；REVIEW→整体 REVIEW 不短路。
func TestCategoryAdmission(t *testing.T) {
	in := passingInput(t)
	in.Category = "medical"
	in.CategoryPolicy = CategoryPolicy{Category: "medical", Admission: "REJECTED"}
	if res := Chain(context.Background(), StandardGates(), in); res.Decision != DecisionBlock {
		t.Errorf("REJECTED 类目应 BLOCK: %s", res.Decision)
	}

	in = passingInput(t)
	in.Category = "medical"
	in.CategoryPolicy = CategoryPolicy{Category: "medical", Admission: "REVIEW", RequiresQualification: true}
	res := Chain(context.Background(), StandardGates(), in)
	if res.Decision != DecisionReview {
		t.Errorf("REVIEW 类目应整体 REVIEW: %s", res.Decision)
	}
	if len(res.Skipped) != 0 {
		t.Errorf("REVIEW 不应短路: %v", res.Skipped)
	}
}

// TestRequiredDisclaimer：类目必需声明缺失 ⇒ BLOCKER。
func TestRequiredDisclaimer(t *testing.T) {
	in := passingInput(t)
	in.CategoryPolicy.RequiredDisclaimers = []string{"效果因人而异"}
	res := Chain(context.Background(), StandardGates(), in)
	if res.Decision != DecisionBlock {
		t.Fatalf("缺声明应 BLOCK: %s", res.Decision)
	}
	if !strings.Contains(res.Findings[0].Detail, "效果因人而异") {
		t.Errorf("finding 应指明缺失声明: %+v", res.Findings[0])
	}

	// 声明出现在 post_text（由注入逻辑追加过 AIGC 文案）→ 通过。
	in = passingInput(t)
	in.CategoryPolicy.RequiredDisclaimers = []string{"AI 生成内容"}
	if res := Chain(context.Background(), StandardGates(), in); res.Decision != DecisionPass {
		t.Errorf("声明存在时应 PASS: %s findings=%+v", res.Decision, res.Findings)
	}
}

// TestThirdPartyRights：引用 shot 带 TRADEMARK ⇒ BLOCK。
func TestThirdPartyRights(t *testing.T) {
	in := passingInput(t)
	in.ShotRiskFlags[shotID] = []string{"TRADEMARK"}
	res := Chain(context.Background(), StandardGates(), in)
	if res.Decision != DecisionBlock {
		t.Fatalf("TRADEMARK 应 BLOCK: %s", res.Decision)
	}
	if res.Findings[0].Risk != "TRADEMARK" {
		t.Errorf("risk = %s", res.Findings[0].Risk)
	}
}

// TestMusicLicense：商用音乐缺凭证 ⇒ BLOCK；平台库音乐放行。
func TestMusicLicense(t *testing.T) {
	in := passingInput(t)
	in.Plan.Audio.MusicRef.LicenseKind = "COMMERCIAL"
	res := Chain(context.Background(), StandardGates(), in)
	if res.Decision != DecisionBlock || res.Findings[0].Gate != "music_license" {
		t.Fatalf("商用缺凭证应 BLOCK: %s %+v", res.Decision, res.Findings)
	}

	in = passingInput(t)
	in.Plan.Audio.MusicRef.LicenseKind = "COMMERCIAL"
	in.Plan.Audio.MusicRef.LicenseProofURI = "https://proof.example/lic/1"
	if res := Chain(context.Background(), StandardGates(), in); res.Decision != DecisionPass {
		t.Errorf("有凭证应 PASS: %+v", res.Findings)
	}
}

// TestVoiceAuthorization：授权过期/撤销/缺失 ⇒ BLOCK；主体不匹配 ⇒ BLOCK。
func TestVoiceAuthorization(t *testing.T) {
	for _, tc := range []struct {
		name string
		auth []Authorization
	}{
		{"过期", []Authorization{{Kind: "VOICE", SubjectID: "tts-001", State: "ACTIVE", ValidUntil: "2026-01-01"}}},
		{"撤销", []Authorization{{Kind: "VOICE", SubjectID: "tts-001", State: "REVOKED"}}},
		{"主体不符", []Authorization{{Kind: "VOICE", SubjectID: "tts-999", State: "ACTIVE"}}},
	} {
		in := passingInput(t)
		in.Authorizations = tc.auth
		res := Chain(context.Background(), StandardGates(), in)
		if res.Decision != DecisionBlock {
			t.Errorf("%s 应 BLOCK: %s", tc.name, res.Decision)
		}
	}
}

// TestAIGCLabel：显式/隐式任缺其一 ⇒ BLOCK。
func TestAIGCLabel(t *testing.T) {
	// 缺隐式（无产物元数据）。
	in := passingInput(t)
	in.Artifact = nil
	res := Chain(context.Background(), StandardGates(), in)
	if res.Decision != DecisionBlock || res.Findings[0].Gate != "aigc_label" {
		t.Fatalf("缺隐式标识应 BLOCK: %s %+v", res.Decision, res.Findings)
	}

	// 缺显式（移除注入的 overlay）。
	in = passingInput(t)
	id := FindDisclosureOverlay(in.Plan)
	var kept []videoplan.Overlay
	for _, o := range in.Plan.Overlays {
		if o.OverlayID != id {
			kept = append(kept, o)
		}
	}
	in.Plan.Overlays = kept
	res = Chain(context.Background(), StandardGates(), in)
	if res.Decision != DecisionBlock {
		t.Errorf("缺显式标识应 BLOCK: %s", res.Decision)
	}
}

// TestAIGCLabelNotRequiredForPureShotPlan：纯实拍+库音乐静音 VO ⇒ 无标识义务。
func TestAIGCLabelNotRequiredForPureShotPlan(t *testing.T) {
	in := passingInput(t)
	in.Plan.Audio.VORef = nil
	in.Artifact = &RenderArtifact{Path: "/o/f.mp4", Metadata: map[string]string{}}
	in.Authorizations = nil
	EnsureAIGCDisclosureCleanup(in.Plan) // 移除样本注入的显式标识
	if res := Chain(context.Background(), StandardGates(), in); res.Decision != DecisionPass {
		t.Errorf("纯实拍应 PASS: %+v", res.Findings)
	}
}

// TestPortraitAuthorization：人脸 shot 无授权 ⇒ BLOCK；有 ACTIVE 授权放行。
func TestPortraitAuthorization(t *testing.T) {
	in := passingInput(t)
	in.ShotRiskFlags[shotID] = []string{"PORTRAIT_RIGHT"}
	res := Chain(context.Background(), StandardGates(), in)
	if res.Decision != DecisionBlock || res.Findings[0].Risk != "PORTRAIT_RIGHT" {
		t.Fatalf("无肖像授权应 BLOCK: %s %+v", res.Decision, res.Findings)
	}

	in.Authorizations = append(in.Authorizations,
		Authorization{Kind: "PORTRAIT", SubjectID: "owner", State: "ACTIVE"})
	if res := Chain(context.Background(), StandardGates(), in); res.Decision != DecisionPass {
		t.Errorf("有 ACTIVE 肖像授权应 PASS: %+v", res.Findings)
	}
}

// TestWriteBack：通过后回写 plan.compliance，且满足 IV-VP-5。
func TestWriteBack(t *testing.T) {
	in := passingInput(t)
	res := Chain(context.Background(), StandardGates(), in)
	if res.Decision != DecisionPass {
		t.Fatalf("前置: %s", res.Decision)
	}
	c := WriteBack(in.Plan, res)
	if !c.AIGCDisclosure.Required || c.AIGCDisclosure.ExplicitOverlayID == nil {
		t.Fatalf("回写块缺标识义务: %+v", c.AIGCDisclosure)
	}
	if len(c.ChecksPassed) != 8 {
		t.Errorf("checks_passed = %v", c.ChecksPassed)
	}
	if err := in.Plan.Validate(); err != nil {
		t.Errorf("回写后 plan 应通过 IV-VP-5: %v", err)
	}
}

// TestIVVP5RejectsInconsistentWriteBack：required 但 overlay 指向错误组件 ⇒ 校验拒绝。
func TestIVVP5RejectsInconsistentWriteBack(t *testing.T) {
	in := passingInput(t)
	res := Chain(context.Background(), StandardGates(), in)
	c := WriteBack(in.Plan, res)
	bad := "cap1" // 指向 caption.karaoke
	c.AIGCDisclosure.ExplicitOverlayID = &bad
	if err := in.Plan.Validate(); err == nil || !strings.Contains(err.Error(), "IV-VP-5") {
		t.Errorf("IV-VP-5 未触发: %v", err)
	}
}

func containsPassed(list []string, name string) bool {
	for _, s := range list {
		if s == name {
			return true
		}
	}
	return false
}

// EnsureAIGCDisclosureCleanup 移除全部 aigc.disclosure overlay（测试专用）。
func EnsureAIGCDisclosureCleanup(p *videoplan.Plan) {
	var kept []videoplan.Overlay
	for _, o := range p.Overlays {
		if o.Component != "aigc.disclosure" {
			kept = append(kept, o)
		}
	}
	p.Overlays = kept
}
