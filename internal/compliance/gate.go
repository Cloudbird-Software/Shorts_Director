// Package compliance 实现 S8 ComplianceGate：QC 之后、Delivery 之前的
// 唯一强制串行门禁，不允许旁路（Engineering_plan §S8）。
// 人工干预同样必须走这条管线：运营编辑只能产生新的 VideoPlan 再过全链 Gate。
//
// 设计原则：
//   - Gate 无状态、顺序执行，任一 BLOCKER 即停（剩余 Gate 记为 skipped）；
//   - 外部事实（词库/类目策略/授权记录/shot 风险标记）全部经 Input 注入，
//     Gate 本身绝不查库、绝不取系统时间——同一 Input 恒同一 GateResult；
//   - Finding.Risk 取值冻结为 vocab/compliance_risk v1。
package compliance

import (
	"context"
	"fmt"
	"sort"
)

// Decision 是整条 Gate 链的结论。
type Decision string

const (
	DecisionPass   Decision = "PASS"   // 全部通过，可交付
	DecisionReview Decision = "REVIEW" // 强制人审（如类目准入灰区）
	DecisionBlock  Decision = "BLOCK"  // 任一 BLOCKER，停止后续 Gate
)

// Severity 是单条 Finding 的级别（违禁词库分级 §S8）。
type Severity string

const (
	SeverityBlocker Severity = "BLOCKER" // 绝对禁止
	SeverityMajor   Severity = "MAJOR"   // 需资质/人审
	SeverityMinor   Severity = "MINOR"   // 建议规避
)

// Finding 是一个 Gate 产出的一条合规发现。
type Finding struct {
	Gate     string   `json:"gate"`               // 产出 Gate 的名字
	Risk     string   `json:"risk"`               // vocab/compliance_risk id
	Severity Severity `json:"severity"`           // BLOCKER|MAJOR|MINOR
	Detail   string   `json:"detail"`             // 人类可读、确定性描述
	Evidence string   `json:"evidence,omitempty"` // 定位线索（文本片段/shot_id 等）
}

// GateResult 是整条链的执行记录（审计凭证，回写 plan.compliance 的数据源）。
type GateResult struct {
	Decision     Decision  `json:"decision"`
	Findings     []Finding `json:"findings"`
	ChecksPassed []string  `json:"checks_passed"` // 依次通过的 Gate 名
	Skipped      []string  `json:"skipped"`       // 因 BLOCK 短路未执行的 Gate
}

// Gate 是单个门禁。返回该门的发现；err 仅表示执行故障（与合规结论正交）。
type Gate interface {
	Name() string
	Check(ctx context.Context, in *Input) []Finding
}

// Chain 顺序执行 Gate 列表：任一 BLOCKER 立即停止（§S8"任一 BLOCK 即停"）。
// REVIEW 级发现不短路，但会把最终 Decision 压到 REVIEW（除非后续 BLOCK）。
func Chain(ctx context.Context, gates []Gate, in *Input) *GateResult {
	res := &GateResult{Decision: DecisionPass}
	for _, g := range gates {
		findings := g.Check(ctx, in)
		if len(findings) == 0 {
			res.ChecksPassed = append(res.ChecksPassed, g.Name())
			continue
		}
		res.Findings = append(res.Findings, findings...)
		if hasBlocker(findings) {
			res.Decision = DecisionBlock
			markSkipped(res, gates, g.Name())
			return res
		}
		if res.Decision == DecisionPass && hasMajor(findings) {
			res.Decision = DecisionReview
		}
	}
	return res
}

// markSkipped 把 blocker 之后的 Gate 记为 skipped（顺序保持注入序）。
func markSkipped(res *GateResult, gates []Gate, blockedAt string) {
	seen := false
	for _, g := range gates {
		if g.Name() == blockedAt {
			seen = true
			continue
		}
		if seen {
			res.Skipped = append(res.Skipped, g.Name())
		}
	}
}

func hasBlocker(fs []Finding) bool {
	for _, f := range fs {
		if f.Severity == SeverityBlocker {
			return true
		}
	}
	return false
}

func hasMajor(fs []Finding) bool {
	for _, f := range fs {
		if f.Severity == SeverityMajor {
			return true
		}
	}
	return false
}

// SortFindings 就地按 (Gate, Risk, Detail) 排序——让输出完全确定。
func SortFindings(fs []Finding) {
	sort.SliceStable(fs, func(i, j int) bool {
		a, b := fs[i], fs[j]
		if a.Gate != b.Gate {
			return a.Gate < b.Gate
		}
		if a.Risk != b.Risk {
			return a.Risk < b.Risk
		}
		return a.Detail < b.Detail
	})
}

// Summary 返回一行人类可读摘要（日志用，确定性）。
func (r *GateResult) Summary() string {
	return fmt.Sprintf("decision=%s passed=%d findings=%d skipped=%d",
		r.Decision, len(r.ChecksPassed), len(r.Findings), len(r.Skipped))
}
