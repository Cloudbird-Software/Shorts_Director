// Package calibrate 实现裁判校准（IR-0007 AC-8 / BEH-7，实验 E6）：
// vlm_boolean 评审探针判定 vs 人工标注集 → 混淆矩阵 + 一致率，
// 内容寻址报告落盘（结论登记进 README 假设看板 E6）。
//
// 口径：正例 = 人工标注可用（human_label=true）；一致率 =
// (TP+TN)/已判定条数（探针报错条目单列，不混入矩阵）。
package calibrate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Cloudbird-Software/Shorts_Director/internal/contracts"
	"github.com/Cloudbird-Software/Shorts_Director/internal/digest"
	"github.com/Cloudbird-Software/Shorts_Director/internal/operator"
)

// SchemaVersion 是校准报告的结构版本。
const SchemaVersion = 1

// ConsumedGoldenOps 声明本包消费 golden 契约的算子（Freeze Gate G7 锚点）。
var ConsumedGoldenOps = []string{"vlm_boolean"}

// Label 是一条人工可用性标注（evals/human_labels/labels.json 条目）。
type Label struct {
	ItemID     string `json:"item_id"`
	MediaPath  string `json:"media_path"` // 相对 labels.json 所在目录
	Question   string `json:"question"`
	HumanLabel bool   `json:"human_label"`
	Labeler    string `json:"labeler"`
	Notes      string `json:"notes,omitempty"`
}

// labelFile 是标注集文件结构。
type labelFile struct {
	SchemaVersion int     `json:"schema_version"`
	Labels        []Label `json:"labels"`
}

// LoadLabels 读取并校验标注集（媒体文件必须落盘存在）。
func LoadLabels(path string) ([]Label, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("calibrate: 读标注集失败: %w", err)
	}
	var lf labelFile
	if err := json.Unmarshal(raw, &lf); err != nil {
		return nil, fmt.Errorf("calibrate: 标注集 %s 不是合法 JSON: %w", path, err)
	}
	if lf.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("calibrate: 标注集 schema_version 必须 %d", SchemaVersion)
	}
	if len(lf.Labels) == 0 {
		return nil, fmt.Errorf("calibrate: 标注集为空")
	}
	root := filepath.Dir(path)
	seen := map[string]bool{}
	for i, l := range lf.Labels {
		if l.ItemID == "" || seen[l.ItemID] {
			return nil, fmt.Errorf("calibrate: labels[%d].item_id 必填且不得重复", i)
		}
		seen[l.ItemID] = true
		if l.MediaPath == "" || l.Question == "" || l.Labeler == "" {
			return nil, fmt.Errorf("calibrate: labels[%d] 缺 media_path/question/labeler", i)
		}
		if _, err := os.Stat(filepath.Join(root, l.MediaPath)); err != nil {
			return nil, fmt.Errorf("calibrate: labels[%d] 媒体缺失: %s: %w", i, l.MediaPath, err)
		}
	}
	return lf.Labels, nil
}

// Matrix 是混淆矩阵（正例 = 人工标注可用）。
type Matrix struct {
	TruePositive  int `json:"true_positive"`  // 探针 true × 人工 true
	FalsePositive int `json:"false_positive"` // 探针 true × 人工 false（漏判对抗样本）
	TrueNegative  int `json:"true_negative"`  // 探针 false × 人工 false
	FalseNegative int `json:"false_negative"` // 探针 false × 人工 true
}

// Total 是已判定条目数。
func (m Matrix) Total() int {
	return m.TruePositive + m.FalsePositive + m.TrueNegative + m.FalseNegative
}

// Agreement 是一致率（(TP+TN)/已判定；无判定条目时 0）。
func (m Matrix) Agreement() float64 {
	t := m.Total()
	if t == 0 {
		return 0
	}
	return float64(m.TruePositive+m.TrueNegative) / float64(t)
}

// ItemVerdict 是单条比对明细。
type ItemVerdict struct {
	ItemID   string `json:"item_id"`
	Human    bool   `json:"human_label"`
	Probe    bool   `json:"probe_answer"`
	Agree    bool   `json:"agree"`
	Evidence string `json:"evidence,omitempty"`
	Error    string `json:"error,omitempty"` // 探针四态失败（单列，不进矩阵）
}

// Report 是一次校准执行的完整产物（内容寻址）。
type Report struct {
	SchemaVersion int           `json:"schema_version"`
	Model         string        `json:"model"`
	LabelsRef     string        `json:"labels_ref"` // 标注集文件路径
	LabelsDigest  string        `json:"labels_digest"`
	Matrix        Matrix        `json:"matrix"`
	Agreement     float64       `json:"agreement"`
	Errors        int           `json:"errors"` // 探针失败条目数
	Items         []ItemVerdict `json:"items"`
	Digest        string        `json:"digest,omitempty"`
}

// Options 是校准执行的注入面。
type Options struct {
	Labels      []Label
	LabelsPath  string // 标注集文件（LabelsRef/LABELS digest 锚）
	Runner      operator.Runner
	Model       string // vlm_boolean 后端键（fake|qwen-vl）
	WorkdirRoot string
	Seed        *int64 // 真实后端必填（AC-3 重放条款）
}

// Run 逐条执行 vlm_boolean 并与人工标注比对。
func Run(ctx context.Context, opts Options) (*Report, error) {
	if len(opts.Labels) == 0 {
		return nil, fmt.Errorf("calibrate: Labels 必填非空")
	}
	if opts.Runner == nil {
		return nil, fmt.Errorf("calibrate: Runner 必填")
	}
	model := opts.Model
	if model == "" {
		model = "fake"
	}
	if model != "fake" && opts.Seed == nil {
		return nil, fmt.Errorf("calibrate: 真实后端必须 Seed（AC-3 重放条款）")
	}
	labelsDigest := ""
	if opts.LabelsPath != "" {
		if d, err := fileDigest(opts.LabelsPath); err == nil {
			labelsDigest = d
		}
	}
	root := filepath.Dir(opts.LabelsPath)
	if opts.LabelsPath == "" {
		root = "."
	}
	rep := &Report{
		SchemaVersion: SchemaVersion, Model: model,
		LabelsRef: opts.LabelsPath, LabelsDigest: labelsDigest,
	}
	for _, l := range opts.Labels {
		v := ItemVerdict{ItemID: l.ItemID, Human: l.HumanLabel}
		mediaPath := l.MediaPath
		if !filepath.IsAbs(mediaPath) {
			mediaPath = filepath.Join(root, l.MediaPath)
		}
		abs, err := filepath.Abs(mediaPath)
		if err != nil {
			abs = mediaPath
		}
		resp, err := opts.Runner.Run(ctx, operator.Request{
			ContractVersion: contracts.ContractOperator,
			Op:              "vlm_boolean",
			Inputs:          map[string]any{"media_path": abs, "question": l.Question},
			Params:          map[string]any{"model": model},
			Workdir:         filepath.Join(opts.WorkdirRoot, l.ItemID),
			Determinism:     operator.Determinism{Seed: opts.Seed},
		})
		switch {
		case err != nil:
			v.Error = err.Error()
		case resp.Status != operator.StatusOK:
			msg := "算子故障"
			if resp.Error != nil {
				msg = resp.Error.Message
			}
			v.Error = string(resp.Status) + ": " + msg
		default:
			v.Probe, _ = resp.Outputs["answer"].(bool)
			v.Evidence, _ = resp.Outputs["evidence"].(string)
			v.Agree = v.Probe == l.HumanLabel
			switch {
			case v.Probe && l.HumanLabel:
				rep.Matrix.TruePositive++
			case v.Probe && !l.HumanLabel:
				rep.Matrix.FalsePositive++
			case !v.Probe && !l.HumanLabel:
				rep.Matrix.TrueNegative++
			default:
				rep.Matrix.FalseNegative++
			}
		}
		if v.Error != "" {
			rep.Errors++
		}
		rep.Items = append(rep.Items, v)
	}
	rep.Agreement = rep.Matrix.Agreement()
	if d, err := rep.ComputeDigest(); err == nil {
		rep.Digest = d
	}
	return rep, nil
}

// ComputeDigest 对报告（除 Digest 自身）做 JCS 内容寻址摘要。
func (r *Report) ComputeDigest() (string, error) {
	copied := *r
	copied.Digest = ""
	return digest.ValueDigest(copied)
}

// Save 落盘校准报告：文件名 = digest hex，返回路径。
func (r *Report) Save(outDir string) (string, error) {
	if r.Digest == "" {
		return "", fmt.Errorf("calibrate: 报告未计算 digest")
	}
	raw, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(outDir, strings.TrimPrefix(r.Digest, "sha256:")+".json")
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// fileDigest 是文件的 sha256。
func fileDigest(path string) (string, error) {
	bs, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return digest.ContentDigest(bs)
}
