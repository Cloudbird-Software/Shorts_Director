package compliance

import (
	"context"
	"fmt"
	"strings"
)

// StandardGates 返回 §S8 冻结的 8 门顺序（唯一强制串行门禁，不允许旁路）。
func StandardGates() []Gate {
	return []Gate{
		CategoryAdmissionGate{},
		BannedTermsGate{},
		RequiredDisclaimerGate{},
		ThirdPartyRightsGate{},
		MusicLicenseGate{},
		VoiceAuthorizationGate{},
		AIGCLabelGate{},
		PortraitAuthorizationGate{},
	}
}

// ---- 1. 类目准入：医疗/医美/金融/招商 → 强制人审或拒接 ----

type CategoryAdmissionGate struct{}

func (CategoryAdmissionGate) Name() string { return "category_admission" }

func (CategoryAdmissionGate) Check(_ context.Context, in *Input) []Finding {
	switch in.CategoryPolicy.Admission {
	case "ALLOWED":
		return nil
	case "REVIEW":
		if in.CategoryPolicy.RequiresQualification && in.CategoryPolicy.QualificationProof == "" {
			return []Finding{{
				Gate: "category_admission", Risk: "MISSING_QUALIFICATION", Severity: SeverityMajor,
				Detail: fmt.Sprintf("类目 %s 需资质凭证方可人审，凭证缺失", in.Category),
			}}
		}
		return []Finding{{
			Gate: "category_admission", Risk: "MISSING_QUALIFICATION", Severity: SeverityMajor,
			Detail: fmt.Sprintf("类目 %s 灰区，强制人审", in.Category),
		}}
	default: // REJECTED 及未配置（宁枉勿纵：未知类目按拒接）
		return []Finding{{
			Gate: "category_admission", Risk: "MISSING_QUALIFICATION", Severity: SeverityBlocker,
			Detail: fmt.Sprintf("类目 %s 不予准入（admission=%s）", in.Category, in.CategoryPolicy.Admission),
		}}
	}
}

// ---- 2. 违禁词：广告法极限词 + 类目资质词（AC 自动机的精确子串层） ----

type BannedTermsGate struct{}

func (BannedTermsGate) Name() string { return "banned_terms" }

func (BannedTermsGate) Check(_ context.Context, in *Input) []Finding {
	texts := in.CopyTexts()
	var fs []Finding
	for _, t := range in.BannedTerms {
		for _, text := range texts {
			if text != "" && strings.Contains(text, t.Term) {
				fs = append(fs, Finding{
					Gate: "banned_terms", Risk: t.Risk, Severity: Severity(t.Severity),
					Detail: fmt.Sprintf("文案命中违禁词 %q", t.Term),
					Evidence: text,
				})
				break // 同词只报一次
			}
		}
	}
	SortFindings(fs)
	return fs
}

// ---- 3. 必需声明：类目要求的声明必须在受检文案中出现 ----

type RequiredDisclaimerGate struct{}

func (RequiredDisclaimerGate) Name() string { return "required_disclaimer" }

func (RequiredDisclaimerGate) Check(_ context.Context, in *Input) []Finding {
	var fs []Finding
	for _, d := range in.CategoryPolicy.RequiredDisclaimers {
		found := false
		for _, text := range in.CopyTexts() {
			if strings.Contains(text, d) {
				found = true
				break
			}
		}
		if !found {
			fs = append(fs, Finding{
				Gate: "required_disclaimer", Risk: "REQUIRED_DISCLAIMER_MISSING",
				Severity: SeverityBlocker,
				Detail:   fmt.Sprintf("类目 %s 必需声明 %q 缺失", in.Category, d),
			})
		}
	}
	return fs
}

// ---- 4. 三方权利：引用 shot 的 TRADEMARK / THIRD_PARTY_CONTENT 风险标记 ----

type ThirdPartyRightsGate struct{}

func (ThirdPartyRightsGate) Name() string { return "third_party_rights" }

func (ThirdPartyRightsGate) Check(_ context.Context, in *Input) []Finding {
	var fs []Finding
	for _, id := range in.ReferencedShotIDs() {
		for _, flag := range in.ShotRiskFlags[id] {
			switch flag {
			case "TRADEMARK", "THIRD_PARTY_CONTENT":
				fs = append(fs, Finding{
					Gate: "third_party_rights", Risk: flag, Severity: SeverityBlocker,
					Detail: fmt.Sprintf("shot %s 带风险标记 %s", id, flag),
					Evidence: id,
				})
			}
		}
	}
	return fs
}

// ---- 5. 音乐授权：凭证存在且未过期 ----

type MusicLicenseGate struct{}

func (MusicLicenseGate) Name() string { return "music_license" }

func (MusicLicenseGate) Check(_ context.Context, in *Input) []Finding {
	if in.Plan == nil || in.Plan.Audio.MusicRef == nil {
		return nil // 静音模式无音轨
	}
	ref := in.Plan.Audio.MusicRef
	switch ref.LicenseKind {
	case "PLATFORM_LIBRARY", "CC0":
		return nil
	case "COMMERCIAL":
		if ref.LicenseProofURI == "" {
			return []Finding{{
				Gate: "music_license", Risk: "MUSIC_LICENSE", Severity: SeverityBlocker,
				Detail: fmt.Sprintf("商用音乐 %s 缺授权凭证（license_proof_uri 为空）", ref.ID),
			}}
		}
		return nil
	default:
		return []Finding{{
			Gate: "music_license", Risk: "MUSIC_LICENSE", Severity: SeverityBlocker,
			Detail: fmt.Sprintf("音乐 %s 的 license_kind 非法 %q", ref.ID, ref.LicenseKind),
		}}
	}
}

// ---- 6. 声音授权：TTS 音色的克隆授权必须 ACTIVE ----

type VoiceAuthorizationGate struct{}

func (VoiceAuthorizationGate) Name() string { return "voice_authorization" }

func (VoiceAuthorizationGate) Check(_ context.Context, in *Input) []Finding {
	if in.Plan == nil || in.Plan.Audio.VORef == nil {
		return nil // 静音模式
	}
	tts := in.Plan.Audio.VORef.TTSID
	for _, a := range in.Authorizations {
		if a.Kind == "VOICE" && a.SubjectID == tts && a.ActiveOn(in.Now) {
			return nil
		}
	}
	return []Finding{{
		Gate: "voice_authorization", Risk: "VOICE_RIGHT", Severity: SeverityBlocker,
		Detail: fmt.Sprintf("TTS %s 无有效声音授权（需 ACTIVE 且 %s 前有效）", tts, in.Now),
	}}
}

// ---- 7. AIGC 标识：显式 overlay + 隐式元数据（GB 45438-2025 双轨） ----

type AIGCLabelGate struct{}

func (AIGCLabelGate) Name() string { return "aigc_label" }

func (AIGCLabelGate) Check(_ context.Context, in *Input) []Finding {
	if in.Plan == nil || !DisclosureRequired(in.Plan) {
		return nil
	}
	var fs []Finding

	// 显式：必须存在可见的 aigc.disclosure overlay（不可被 StyleTheme 隐藏）。
	if id := FindDisclosureOverlay(in.Plan); id == "" {
		fs = append(fs, Finding{
			Gate: "aigc_label", Risk: "AIGC_LABEL_REQUIRED", Severity: SeverityBlocker,
			Detail: "生成内容缺显式标识：无可见的 aigc.disclosure overlay",
		})
	}

	// 隐式：产物元数据必须含可解析的 AIGC 标识块（字段映射版本化，见 aigc.go）。
	if in.Artifact == nil || !HasImplicitLabel(in.Artifact.Metadata) {
		fs = append(fs, Finding{
			Gate: "aigc_label", Risk: "AIGC_LABEL_REQUIRED", Severity: SeverityBlocker,
			Detail: "生成内容缺隐式标识：产物元数据无 AIGC 块或字段不完整",
		})
	}
	return fs
}

// ---- 8. 肖像授权：带 PORTRAIT_RIGHT 风险的 shot 需 ACTIVE 授权 ----

type PortraitAuthorizationGate struct{}

func (PortraitAuthorizationGate) Name() string { return "portrait_authorization" }

func (PortraitAuthorizationGate) Check(_ context.Context, in *Input) []Finding {
	var flagged []string
	for _, id := range in.ReferencedShotIDs() {
		for _, flag := range in.ShotRiskFlags[id] {
			if flag == "PORTRAIT_RIGHT" || flag == "THIRD_PARTY_FACE" {
				flagged = append(flagged, id)
				break
			}
		}
	}
	if len(flagged) == 0 {
		return nil
	}
	for _, a := range in.Authorizations {
		if a.Kind == "PORTRAIT" && a.ActiveOn(in.Now) {
			return nil
		}
	}
	return []Finding{{
		Gate: "portrait_authorization", Risk: "PORTRAIT_RIGHT", Severity: SeverityBlocker,
		Detail: fmt.Sprintf("shot %v 含人脸但无有效肖像授权", flagged),
	}}
}
