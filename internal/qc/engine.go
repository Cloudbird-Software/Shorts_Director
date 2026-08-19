// engine.go 实现 S7 QCService 的断言引擎编排（Engineering_plan §S7）：
// applies_when 过滤 → probe 分组去重 → 成本排序 → BLOCKER 短路 →
// remedy 指令渲染。probe 算子本体属 C2 边界，通过 Probe 接口注入。
package qc

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Cloudbird-Software/Shorts_Director/internal/digest"
	"github.com/Cloudbird-Software/Shorts_Director/internal/entity"
	"github.com/Cloudbird-Software/Shorts_Director/internal/slotquery"
)

// Measurement 是一次 probe 测量的产物：值 + 证据 URI（A2：非确定性
// 显式落盘，证据可追溯）。Value 的形态由 probe 决定（数值/bool/命中列表）。
type Measurement struct {
	Value       any    `json:"value"`
	EvidenceURI string `json:"evidence_uri,omitempty"`
}

// CostTier 是 probe 的成本档位，驱动执行顺序（便宜的先跑）。
type CostTier int

const (
	CostFree  CostTier = iota // 元数据级（ffprobe 字段）
	CostLight                 // 轻量 CV（黑帧/清晰度/检测器）
	CostHeavy                 // 重量模型（SyncNet/VLM）
)

// ProbeOperator 是 QC 算子接口（C2 Operator 边界；实现住在算子仓库，
// 引擎只依赖此接口——深接口设计：编排与执行解耦）。与 DSL 的
// Probe 结构体（断言里"测什么"的声明）相对：一个声明、一个执行。
type ProbeOperator interface {
	// ID 与 probe.op 同名（如 "blackdetect_ratio"）。
	ID() string
	// Measure 对被检对象执行测量。args 已完成模板渲染。
	Measure(ctx context.Context, subj *Subject, args map[string]any) (Measurement, error)
	// Cost 返回成本档位（引擎按此排序，BLOCKER 短路前置）。
	Cost() CostTier
}

// Subject 是被检对象：渲染产物/生成物/素材，及其 QC 上下文。
// Fields 是 applies_when 的求值属性（含不在 Shot 上的关联字段，
// 如 Asset 的 source_kind）；Spec 是制作令 spec，供模板变量 {{spec.*}}。
type Subject struct {
	MediaURI  string         `json:"media_uri"`
	MediaHash string         `json:"media_hash,omitempty"` // 内容寻址；缓存键
	Shot      *entity.Shot   `json:"shot,omitempty"`       // 关联 shot（可空）
	Spec      map[string]any `json:"spec,omitempty"`       // 制作令 spec
	Fields    map[string]any `json:"fields,omitempty"`     // applies_when 属性
}

// Attrs 返回 applies_when 求值属性集：Shot 白名单字段展平，
// Fields 覆盖同名字段（关联字段与运行期覆盖优先）。
func (s *Subject) Attrs() map[string]any {
	attrs := map[string]any{}
	if s.Shot != nil {
		attrs = slotquery.FlattenShot(s.Shot)
	}
	for k, v := range s.Fields {
		attrs[k] = v
	}
	return attrs
}

// Result 是单条断言的判定产物（A5：bool + 证据，无打分）。
type Result struct {
	AssertionID string   `json:"assertion_id"`
	Level       Level    `json:"level"`
	Severity    Severity `json:"severity"`
	Pass        bool     `json:"pass"`
	Measured    any      `json:"measured,omitempty"`
	Expected    any      `json:"expected,omitempty"`
	EvidenceURI string   `json:"evidence_uri,omitempty"`
	Instruction string   `json:"instruction,omitempty"` // 失败时渲染的人话返修指令
	Skipped     string   `json:"skipped,omitempty"`     // 跳过原因（条件不符/短路）
}

// RemedyInstruction 是 remedy_sheet 的一条可执行返修指令。
type RemedyInstruction struct {
	AssertionID string   `json:"assertion_id"`
	Action      string   `json:"action"` // vocab/remedy_action
	Severity    Severity `json:"severity"`
	Instruction string   `json:"instruction"`
	AutoFixable bool     `json:"auto_fixable"`
	AutoFixOp   string   `json:"auto_fix_op,omitempty"`
}

// Report 是一次 QC 运行的完整产物。
type Report struct {
	SubjectHash    string              `json:"subject_hash"`
	Results        []Result            `json:"results"`
	RemedySheet    []RemedyInstruction `json:"remedy_sheet"`
	ShortCircuited bool                `json:"short_circuited"` // BLOCKER 失败触发
}

// Pass 报告是否全部适用断言通过（跳过的不算失败）。
func (r *Report) Pass() bool { return len(r.RemedySheet) == 0 }

// Engine 是断言引擎：probe 注册表 + 编排。
type Engine struct {
	probes map[string]ProbeOperator
}

// NewEngine 构造引擎并注册 probe（ID 重复视为装配错误）。
func NewEngine(probes ...ProbeOperator) (*Engine, error) {
	e := &Engine{probes: map[string]ProbeOperator{}}
	for _, p := range probes {
		if _, dup := e.probes[p.ID()]; dup {
			return nil, fmt.Errorf("qc: probe %q 重复注册", p.ID())
		}
		e.probes[p.ID()] = p
	}
	return e, nil
}

// probeGroup 是共享同一次测量的断言组（去重单元）。
type probeGroup struct {
	op         string
	args       map[string]any // 已渲染
	key        string         // op + args 规范化摘要
	cost       CostTier
	assertions []Assertion
}

// Run 对被检对象执行断言集（Engineering_plan §S7 编排五步）。
// 断言集须预先 Validate；BLOCKER 失败短路——一个 L0 BLOCKER 失败
// 就不该再跑 L2 的 SyncNet（QC 成本控制的核心）。
func (e *Engine) Run(ctx context.Context, subj *Subject, assertions []Assertion) (*Report, error) {
	rep := &Report{SubjectHash: subj.MediaHash}
	attrs := subj.Attrs()
	vars := renderVars(subj)

	// 1) applies_when 过滤 + 2) 按 probe 分组去重
	groups := map[string]*probeGroup{}
	var order []string // 组执行顺序占位（稍后按成本排序）
	for _, a := range assertions {
		if err := a.Validate(); err != nil {
			return nil, fmt.Errorf("qc: 断言集非法: %w", err)
		}
		if a.AppliesWhen != nil {
			apply, err := slotquery.EvaluateFields(*a.AppliesWhen, attrs)
			if err != nil {
				return nil, fmt.Errorf("qc: %s applies_when: %w", a.AssertionID, err)
			}
			if !apply {
				rep.Results = append(rep.Results, Result{
					AssertionID: a.AssertionID, Level: a.Level, Severity: a.Severity,
					Skipped: "applies_when 不满足",
				})
				continue
			}
		}
		args, err := renderArgs(a.Probe.Args, vars)
		if err != nil {
			return nil, fmt.Errorf("qc: %s probe.args 模板渲染失败: %w", a.AssertionID, err)
		}
		key, err := probeKey(a.Probe.Op, args)
		if err != nil {
			return nil, fmt.Errorf("qc: %s probe 去重键: %w", a.AssertionID, err)
		}
		g, seen := groups[key]
		if !seen {
			probe, ok := e.probes[a.Probe.Op]
			if !ok {
				return nil, fmt.Errorf("qc: probe %q 未注册", a.Probe.Op)
			}
			g = &probeGroup{op: a.Probe.Op, args: args, key: key, cost: probe.Cost()}
			groups[key] = g
			order = append(order, key)
		}
		g.assertions = append(g.assertions, a)
	}

	// 3) 按成本排序：先跑便宜的 L0（稳定：同档保持声明序）
	sort.SliceStable(order, func(i, j int) bool {
		return groups[order[i]].cost < groups[order[j]].cost
	})

	// 4) 逐组测量一次，组内逐断言比较；BLOCKER 失败短路
	for _, key := range order {
		g := groups[key]
		probe := e.probes[g.op]
		m, err := probe.Measure(ctx, subj, g.args)
		if err != nil {
			return nil, fmt.Errorf("qc: probe %s 测量失败: %w", g.op, err)
		}
		for _, a := range g.assertions {
			pass, err := compareExpect(a.Expect, m.Value)
			if err != nil {
				return nil, fmt.Errorf("qc: %s expect 比较: %w", a.AssertionID, err)
			}
			res := Result{
				AssertionID: a.AssertionID, Level: a.Level, Severity: a.Severity,
				Pass: pass, Measured: m.Value, Expected: a.Expect.Value,
				EvidenceURI: m.EvidenceURI,
			}
			if !pass {
				instr, err := renderInstruction(a.Remedy, m.Value, a.Expect.Value, vars)
				if err != nil {
					return nil, fmt.Errorf("qc: %s remedy 渲染: %w", a.AssertionID, err)
				}
				res.Instruction = instr
				rep.RemedySheet = append(rep.RemedySheet, RemedyInstruction{
					AssertionID: a.AssertionID, Action: a.Remedy.Action,
					Severity: a.Severity, Instruction: instr,
					AutoFixable: a.Remedy.AutoFixable, AutoFixOp: deref(a.Remedy.AutoFixOp),
				})
			}
			rep.Results = append(rep.Results, res)
		}
		// BLOCKER 失败：短路——剩余组全部标记跳过
		if shortCircuit(rep) {
			rep.ShortCircuited = true
			done := map[string]bool{key: true}
			for _, k := range order {
				if done[k] {
					continue
				}
				for _, a := range groups[k].assertions {
					rep.Results = append(rep.Results, Result{
						AssertionID: a.AssertionID, Level: a.Level, Severity: a.Severity,
						Skipped: "BLOCKER 短路",
					})
				}
			}
			return rep, nil
		}
	}
	return rep, nil
}

// shortCircuited 报告是否已有 BLOCKER 级失败。
func shortCircuit(rep *Report) bool {
	for _, r := range rep.RemedySheet {
		if r.Severity == SeverityBlocker {
			return true
		}
	}
	return false
}

// probeKey 生成去重键：op + args 的 JCS 摘要（多处断言共用一次测量）。
func probeKey(op string, args map[string]any) (string, error) {
	raw, err := json.Marshal(args)
	if err != nil {
		return "", err
	}
	canon, err := digest.CanonicalizeJSON(raw)
	if err != nil {
		return "", err
	}
	h, err := digest.ContentDigest(canon)
	if err != nil {
		return "", err
	}
	return op + ":" + h, nil
}

// ── 期望比较（A5：判定题化，assertion → bool）─────────────────

// compareExpect 把测量值与期望比较。数值算子要求双方可数值化；
// contains_none/contains_all 语义为命中列表与禁用列表的交集判定。
func compareExpect(e Expect, measured any) (bool, error) {
	switch e.Op {
	case "gte", "lte", "eq", "neq":
		m, mok := toFloat(measured)
		w, wok := toFloat(e.Value)
		if mok && wok {
			switch e.Op {
			case "gte":
				return m >= w, nil
			case "lte":
				return m <= w, nil
			case "eq":
				return m == w, nil
			default:
				return m != w, nil
			}
		}
		if e.Op == "eq" {
			return equalAny(measured, e.Value), nil
		}
		if e.Op == "neq" {
			return !equalAny(measured, e.Value), nil
		}
		return false, fmt.Errorf("qc: expect.op %s 要求数值，measured=%T expected=%T", e.Op, measured, e.Value)
	case "between":
		arr, ok := e.Value.([]any)
		if !ok || len(arr) != 2 {
			return false, fmt.Errorf("qc: between 的 value 必须是二元数组")
		}
		m, ok := toFloat(measured)
		if !ok {
			return false, fmt.Errorf("qc: between 要求数值 measured，得到 %T", measured)
		}
		lo, ok1 := toFloat(arr[0])
		hi, ok2 := toFloat(arr[1])
		if !ok1 || !ok2 {
			return false, fmt.Errorf("qc: between 的区间端点必须是数值")
		}
		return m >= lo && m <= hi, nil
	case "is_true":
		b, ok := measured.(bool)
		return ok && b, nil
	case "is_false":
		b, ok := measured.(bool)
		return ok && !b, nil
	case "contains_none", "contains_all":
		want, ok := e.Value.([]any)
		if !ok {
			return false, fmt.Errorf("qc: %s 的 value 必须是数组", e.Op)
		}
		hits, ok := measured.([]any)
		if !ok {
			return false, fmt.Errorf("qc: %s 要求 measured 是命中列表，得到 %T", e.Op, measured)
		}
		intersect := false
		for _, w := range want {
			for _, h := range hits {
				if equalAny(h, w) {
					intersect = true
				}
			}
		}
		if e.Op == "contains_none" {
			return !intersect, nil
		}
		return intersect, nil
	}
	return false, fmt.Errorf("qc: 非受控 expect.op %q", e.Op)
}

// equalAny 跨 JSON 解码形态比较（number/string/bool）。
func equalAny(a, b any) bool {
	if ab, ok := a.(bool); ok {
		bb, ok2 := b.(bool)
		return ok2 && ab == bb
	}
	if af, ok := toFloat(a); ok {
		if bf, ok2 := toFloat(b); ok2 {
			return af == bf
		}
		return false
	}
	as, aok := a.(string)
	bs, bok := b.(string)
	return aok && bok && as == bs
}

func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int:
		return float64(x), true
	}
	return 0, false
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// ── 模板渲染（remedy 指令 + probe args 的 {{spec.*}} 变量）─────

// templateVar 匹配 {{path.to.var}}。
var templateVar = regexp.MustCompile(`\{\{\s*([a-zA-Z_][a-zA-Z0-9_.]*)\s*\}\}`)

// renderVars 组装模板变量域：spec/compliance 等上下文对象。
func renderVars(subj *Subject) map[string]any {
	vars := map[string]any{}
	for k, v := range subj.Spec {
		vars[k] = v
	}
	return vars
}

// renderArgs 递归渲染 probe.args 中的模板变量（如 "{{spec.subject}}"）。
func renderArgs(args map[string]any, vars map[string]any) (map[string]any, error) {
	out := make(map[string]any, len(args))
	for k, v := range args {
		rv, err := renderValue(v, vars)
		if err != nil {
			return nil, fmt.Errorf("args.%s: %w", k, err)
		}
		out[k] = rv
	}
	return out, nil
}

func renderValue(v any, vars map[string]any) (any, error) {
	switch x := v.(type) {
	case string:
		return renderString(x, vars)
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, e := range x {
			rv, err := renderValue(e, vars)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", k, err)
			}
			out[k] = rv
		}
		return out, nil
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			rv, err := renderValue(e, vars)
			if err != nil {
				return nil, fmt.Errorf("[%d]: %w", i, err)
			}
			out[i] = rv
		}
		return out, nil
	}
	return v, nil
}

// renderInstruction 渲染人话返修指令：{{measured}} {{expected}} {{spec.*}}。
// 未知变量报错——宁失败不静默（错误指令比失败更糟）。指令恒为文本：
// 纯占位模板（如 "{{measured}}"）也渲染成字符串。
func renderInstruction(r Remedy, measured, expected any, vars map[string]any) (string, error) {
	vars["measured"] = fmtValue(measured)
	vars["expected"] = fmtValue(expected)
	v, err := renderString(r.InstructionTemplate, vars)
	if err != nil {
		return "", err
	}
	if s, ok := v.(string); ok {
		return s, nil
	}
	return fmtValue(v), nil
}

// renderString 渲染整串模板；纯变量占位保留原类型（"{{n}}" → 数值），
// 混合文本一律成串。未知变量是契约错误。
func renderString(s string, vars map[string]any) (any, error) {
	m := templateVar.FindStringSubmatch(s)
	if m != nil && strings.TrimSpace(s) == m[0] {
		v, err := lookupPath(vars, m[1])
		if err != nil {
			return nil, err
		}
		return v, nil
	}
	var sbErr error
	out := templateVar.ReplaceAllStringFunc(s, func(match string) string {
		sub := templateVar.FindStringSubmatch(match)
		v, err := lookupPath(vars, sub[1])
		if err != nil {
			if sbErr == nil {
				sbErr = err
			}
			return match
		}
		return fmtValue(v)
	})
	if sbErr != nil {
		return nil, sbErr
	}
	return out, nil
}

// lookupPath 按点路径取嵌套变量（spec.subject → vars["spec"]["subject"]）。
func lookupPath(vars map[string]any, path string) (any, error) {
	parts := strings.Split(path, ".")
	var cur any = vars
	for _, p := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("模板变量 {{%s}} 的路径 %q 不可达", path, p)
		}
		cur, ok = m[p]
		if !ok {
			return nil, fmt.Errorf("模板变量 {{%s}} 未定义", path)
		}
	}
	return cur, nil
}

// fmtValue 把测量/变量值渲染为人话：整数不带小数点，列表用顿号连接。
func fmtValue(v any) string {
	switch x := v.(type) {
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			parts[i] = fmtValue(e)
		}
		return strings.Join(parts, "、")
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", x)
	}
}
