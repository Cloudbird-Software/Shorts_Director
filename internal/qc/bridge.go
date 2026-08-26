// bridge.go 把 C2 算子 Runner 适配成 qc.ProbeOperator——
// 断言引擎与算子执行两个边界的缝合点。
//
// 输出契约约定：qc 探针算子的 outputs 固定为
//
//	{ "value": <测量值>, "evidence_uri": "<证据，可选>" }
//
// value 的类型由断言的 expect.op 决定（数值/bool/命中列表）。
package qc

import (
	"context"
	"fmt"

	"github.com/Cloudbird-Software/Shorts_Director/internal/contracts"
	"github.com/Cloudbird-Software/Shorts_Director/internal/operator"
)

// RunnerProbeAdapter 把一个 C2 算子包装成一个 qc 探针。
// qc probe.op 与算子 op 同名直通；将来算子拆分（如 resolution → probe）
// 在此加映射表，不影响引擎。
type RunnerProbeAdapter struct {
	Op     string   // qc probe.op（= 算子 op）
	Tier   CostTier // 成本档位（手工标注：元数据级=Free，CV=Light，模型=Heavy）
	Runner operator.Runner
	// Workdir 是该探针的调用工作目录（内容寻址；空则用系统临时目录约定）。
	Workdir string
}

// Operators 从映射表批量构造探针（NewEngine 的入参形态）。
func Operators(runner operator.Runner, workdir string, specs map[string]CostTier) []ProbeOperator {
	ops := make([]ProbeOperator, 0, len(specs))
	for op, tier := range specs {
		ops = append(ops, &RunnerProbeAdapter{Op: op, Tier: tier, Runner: runner, Workdir: workdir})
	}
	return ops
}

func (a *RunnerProbeAdapter) ID() string     { return a.Op }
func (a *RunnerProbeAdapter) Cost() CostTier { return a.Tier }

// ConsumedGoldenOps 声明本包消费 golden 契约的算子（Freeze Gate G7 锚点）：
// internal/operator 的 golden 清单测试据此校验本包 Outputs 字面访问 ⊆
// testdata/golden 清单——上游删改输出字段时此处失败，而不是静默读零值。
var ConsumedGoldenOps = []string{"blackdetect_ratio"}

// Measure 经 C2 Runner 执行算子并提取测量值。
// 算子四态语义：OK → Measurement；INPUT_ERROR → 引擎错误（上游数据问题，
// 由编排层决定换素材/重传）；其余 → 引擎错误（基础设施故障不产生假报告）。
func (a *RunnerProbeAdapter) Measure(ctx context.Context, subj *Subject, args map[string]any) (Measurement, error) {
	inputs := map[string]any{}
	for k, v := range args {
		inputs[k] = v
	}
	inputs["media_path"] = subj.MediaURI
	if subj.MediaHash != "" {
		inputs["media_hash"] = subj.MediaHash
	}
	workdir := a.Workdir
	if workdir == "" {
		workdir = "/tmp/qc/" + a.Op // 算子只用 workdir 落中间产物，路径不进 golden 键
	}
	resp, err := a.Runner.Run(ctx, operator.Request{
		ContractVersion: contracts.ContractOperator,
		Op:              a.Op,
		Inputs:          inputs,
		Workdir:         workdir,
		Determinism:     operator.Determinism{Seed: nil},
	})
	if err != nil {
		return Measurement{}, fmt.Errorf("qc/bridge %s: %w", a.Op, err)
	}
	if resp.Status != operator.StatusOK {
		msg := "算子故障"
		if resp.Error != nil {
			msg = resp.Error.Message
		}
		return Measurement{}, fmt.Errorf("qc/bridge %s: %s: %s", a.Op, resp.Status, msg)
	}
	return Measurement{
		Value:       resp.Outputs["value"],
		EvidenceURI: stringOrEmpty(resp.Outputs["evidence_uri"]),
	}, nil
}

func stringOrEmpty(v any) string {
	s, _ := v.(string)
	return s
}
