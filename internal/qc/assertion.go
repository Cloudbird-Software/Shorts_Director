// Package qc 实现 QCAssertion DSL（schema/entities/qc_assertion.schema.json）
// 的求值器：S7 QCService 的核心。设计公理 A5——评估判定题化：
// assertion → bool + 证据，禁止 1–10 打分作门禁。
// probe 算子本体属 C2 算子边界（Runner 注入），本包只做期望比较、
// 适用条件筛选与返修指令渲染。
package qc

import (
	"fmt"
	"strings"

	"github.com/Cloudbird-Software/Shorts_Director/internal/vocab"
)

// Level 是 QC 分层：L0 确定性 / L1 一致性 / L2 生成物缺陷 / L3 合规。
// 阈值策略：L0/L1/L2 宁可漏检（返修成本高），L3 宁可错杀（合规不可逆）。
type Level string

const (
	L0 Level = "L0"
	L1 Level = "L1"
	L2 Level = "L2"
	L3 Level = "L3"
)

// Severity 是断言失败时的处置等级。
type Severity string

const (
	SeverityBlocker Severity = "BLOCKER"
	SeverityMajor   Severity = "MAJOR"
	SeverityMinor   Severity = "MINOR"
)

// Probe 声明"测什么"：可插拔算子 + 参数（29 值冻结白名单）。
type Probe struct {
	Op   string         `json:"op"`
	Args map[string]any `json:"args"`
}

// Expect 声明"期望什么"：受控比较操作 + 期望值（between 为二元数组）。
type Expect struct {
	Op    string `json:"op"`
	Value any    `json:"value"`
}

// Remedy 是返修指令模板（不是错误消息）；模板变量 {{measured}} {{expected}}。
type Remedy struct {
	Action              string  `json:"action"` // vocab/remedy_action
	InstructionTemplate string  `json:"instruction_template"`
	AutoFixable         bool    `json:"auto_fixable,omitempty"`
	AutoFixOp           *string `json:"auto_fix_op"` // auto_fixable=true 时必填
}

// Sampling 是采样策略。
type Sampling struct {
	Frames string `json:"frames,omitempty"` // ALL|EVERY_N|KEYFRAMES|FIRST_LAST|N_UNIFORM
	N      int    `json:"n,omitempty"`
}

// Assertion 是一条完整断言。
type Assertion struct {
	AssertionID string     `json:"assertion_id"`
	Level       Level      `json:"level"`
	Severity    Severity   `json:"severity"`
	Probe       Probe      `json:"probe"`
	Expect      Expect     `json:"expect"`
	Remedy      Remedy     `json:"remedy"`
	Sampling    *Sampling  `json:"sampling,omitempty"`
	AppliesWhen *Predicate `json:"applies_when,omitempty"`
}

// probeOps 是 29 值冻结算子白名单（schema enum 同源；
// 开放词表检测类算子走 vocab 分表校验，值域由 args 各自约束）。
var probeOps = map[string]bool{
	// L0 确定性
	"ffprobe_field": true, "blackdetect_ratio": true, "freezedetect_ratio": true,
	"laplacian_var": true, "optical_flow_magnitude": true, "loudness_lufs": true,
	"true_peak_dbtp": true, "silence_ratio": true, "flicker_index": true,
	"resolution": true, "fps": true,
	// L1 一致性
	"object_present": true, "object_area_ratio": true, "shot_type_match": true,
	"camera_motion_match": true, "negative_space_at": true,
	"subject_bbox_within_safe": true, "vlm_boolean": true,
	// L2 生成物缺陷
	"lipsync_lse_c": true, "lipsync_lse_d": true, "temporal_warp_error": true,
	"nr_video_quality": true, "face_identity_sim": true,
	// L3 合规
	"banned_terms": true, "required_disclaimer": true, "third_party_logo": true,
	"third_party_face": true, "aigc_metadata_present": true, "aigc_overlay_present": true,
}

var expectOps = map[string]bool{
	"gte": true, "lte": true, "eq": true, "neq": true, "between": true,
	"is_true": true, "is_false": true, "contains_none": true, "contains_all": true,
}

var samplingFrames = map[string]bool{
	"ALL": true, "EVERY_N": true, "KEYFRAMES": true, "FIRST_LAST": true, "N_UNIFORM": true,
}

// Validate 校验断言的受控域：层级/严重度/算子白名单/词表绑定/采样约束。
func (a Assertion) Validate() error {
	if a.AssertionID == "" {
		return fmt.Errorf("qc: assertion_id 必填")
	}
	prefix := string(a.Level) + "."
	if !strings.HasPrefix(a.AssertionID, prefix) {
		return fmt.Errorf("qc: assertion_id %q 必须以层级前缀 %q 开头", a.AssertionID, prefix)
	}
	switch a.Level {
	case L0, L1, L2, L3:
	default:
		return fmt.Errorf("qc: level 非法 %q", a.Level)
	}
	switch a.Severity {
	case SeverityBlocker, SeverityMajor, SeverityMinor:
	default:
		return fmt.Errorf("qc: severity 非法 %q", a.Severity)
	}
	if !probeOps[a.Probe.Op] {
		return fmt.Errorf("qc: probe.op %q 不在受控白名单", a.Probe.Op)
	}
	if a.Probe.Args == nil {
		return fmt.Errorf("qc: probe.args 必填（可为空对象）")
	}
	if !expectOps[a.Expect.Op] {
		return fmt.Errorf("qc: expect.op %q 不在受控白名单", a.Expect.Op)
	}
	if a.Expect.Value == nil {
		return fmt.Errorf("qc: expect.value 必填")
	}
	if err := a.validateExpectShape(); err != nil {
		return err
	}
	if err := a.validateRemedy(); err != nil {
		return err
	}
	if err := a.validateSampling(); err != nil {
		return err
	}
	if a.AppliesWhen != nil {
		// applies_when 谓词：字段白名单与词表受控校验（自 slotquery 迁移精简）。
		if err := a.AppliesWhen.Validate(); err != nil {
			return fmt.Errorf("qc: applies_when: %w", err)
		}
	}
	return nil
}

// validateExpectShape 校验 expect.op 与 value 形态的一致性。
func (a Assertion) validateExpectShape() error {
	switch a.Expect.Op {
	case "between":
		arr, ok := a.Expect.Value.([]any)
		if !ok || len(arr) != 2 {
			return fmt.Errorf("qc: between 的 value 必须是二元数组")
		}
	case "is_true", "is_false":
		if b, ok := a.Expect.Value.(bool); !ok || !b {
			return fmt.Errorf("qc: %s 的 value 必须为 true", a.Expect.Op)
		}
	case "contains_none", "contains_all":
		if _, ok := a.Expect.Value.([]any); !ok {
			return fmt.Errorf("qc: %s 的 value 必须是数组", a.Expect.Op)
		}
	}
	return nil
}

// validateRemedy 校验返修动作在词表内，且 auto_fixable 语义自洽（实现侧校验）。
func (a Assertion) validateRemedy() error {
	if !vocab.IsVocabID("remedy_action", a.Remedy.Action) {
		return fmt.Errorf("qc: remedy.action %q 不在词表 remedy_action", a.Remedy.Action)
	}
	if a.Remedy.InstructionTemplate == "" {
		return fmt.Errorf("qc: remedy.instruction_template 必填")
	}
	if a.Remedy.AutoFixable && (a.Remedy.AutoFixOp == nil || *a.Remedy.AutoFixOp == "") {
		return fmt.Errorf("qc: auto_fixable=true 时 auto_fix_op 必填")
	}
	return nil
}

// validateSampling 校验采样策略形态。
func (a Assertion) validateSampling() error {
	if a.Sampling == nil {
		return nil
	}
	if a.Sampling.N < 0 {
		return fmt.Errorf("qc: sampling.n 不得为负")
	}
	if !samplingFrames[a.Sampling.Frames] {
		return fmt.Errorf("qc: sampling.frames %q 非法", a.Sampling.Frames)
	}
	if (a.Sampling.Frames == "EVERY_N" || a.Sampling.Frames == "N_UNIFORM") && a.Sampling.N < 1 {
		return fmt.Errorf("qc: sampling.frames=%s 需要 n≥1", a.Sampling.Frames)
	}
	return nil
}
