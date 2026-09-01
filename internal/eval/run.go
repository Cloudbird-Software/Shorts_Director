// run.go 实现套件执行与 run artifact（IFACE-2：内嵌套件定义全文 +
// capability profile 引用，自 artifact 可复算聚合）。
package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Cloudbird-Software/Shorts_Director/internal/contracts"
	"github.com/Cloudbird-Software/Shorts_Director/internal/digest"
	"github.com/Cloudbird-Software/Shorts_Director/internal/operator"
	"github.com/Cloudbird-Software/Shorts_Director/internal/qc"
)

// 条目状态四值：生成成功 / 生成失败 / 断言失败 / 断言基础设施故障。
const (
	ItemOK            = "OK"
	ItemGenFail       = "GEN_FAIL"
	ItemAssertFail    = "ASSERT_FAIL"
	ItemAssertError   = "ASSERT_ERROR"
	ItemSkippedBudget = "SKIPPED_BUDGET"
)

// ConsumedGoldenOps 声明本包消费 golden 契约的算子（Freeze Gate G7 锚点）。
var ConsumedGoldenOps = []string{"gen_i2v"}

// ItemResult 是单次抽卡（entry×seed）的判定明细。
type ItemResult struct {
	EntryID       string            `json:"entry_id"`
	Seed          int64             `json:"seed"`
	Status        string            `json:"status"`
	VideoPath     string            `json:"video_path,omitempty"`
	ContentHash   string            `json:"content_hash,omitempty"`
	WallMs        int64             `json:"wall_ms,omitempty"`
	GpuSeconds    float64           `json:"gpu_seconds,omitempty"`
	PeakMemMB     float64           `json:"peak_mem_mb,omitempty"`
	ModelVersions map[string]string `json:"model_versions,omitempty"`
	Assertions    []qc.Result       `json:"assertions,omitempty"`
	Usable        bool              `json:"usable"`
	Error         string            `json:"error,omitempty"`
}

// Yield 是聚合产物（IFACE-5 唯一口径：可复算）。
type Yield struct {
	EntriesTotal       int     `json:"entries_total"`       // 已尝试条目（≥1 个非跳过抽卡）
	EntriesWithUsable  int     `json:"entries_with_usable"` // 至少 1 条可用的条目数
	YieldRatio         float64 `json:"yield_ratio"`         // 条目出片率 ∈ [0,1]
	ItemsTotal         int     `json:"items_total"`
	ItemsUsable        int     `json:"items_usable"`
	ItemsGenFail       int     `json:"items_gen_fail"`
	ItemsAssertFail    int     `json:"items_assert_fail"`
	ItemsAssertError   int     `json:"items_assert_error"`
	ItemsSkippedBudget int     `json:"items_skipped_budget"`
	GpuSecondsTotal    float64 `json:"gpu_seconds_total"`
}

// RunArtifact 是一次套件执行的完整产物（内容寻址）。
type RunArtifact struct {
	SchemaVersion        int          `json:"schema_version"`
	Suite                Suite        `json:"suite"` // 全文内嵌（IFACE-2）
	CapabilityProfileRef string       `json:"capability_profile_ref"`
	RunnerMode           string       `json:"runner_mode"` // fake|local|docker
	StartedAt            string       `json:"started_at"`
	FinishedAt           string       `json:"finished_at"`
	BudgetTruncated      bool         `json:"budget_truncated"`
	Items                []ItemResult `json:"items"`
	Yield                Yield        `json:"yield"`
	Digest               string       `json:"digest,omitempty"`
}

// RunOptions 是套件执行的注入面。
type RunOptions struct {
	Suite       *Suite
	Gen         operator.Runner // 生成算子执行器
	Engine      *qc.Engine      // 断言引擎（probe 已注册）
	ProfileRef  string          // capability profile 内容寻址引用（sha256:…）
	RunnerMode  string          // fake|local|docker（执行环境签名）
	WorkdirRoot string          // 逐条目工作目录根（内容寻址）
	Now         func() time.Time
}

// ComputeYield 从判定明细序列复算聚合（口径唯一入口；property test 守护）。
// 条目"已尝试"= 至少一个非 SKIPPED_BUDGET 抽卡。
func ComputeYield(items []ItemResult) Yield {
	var y Yield
	entryAttempted := map[string]bool{}
	entryUsable := map[string]bool{}
	for _, it := range items {
		y.ItemsTotal++
		switch it.Status {
		case ItemSkippedBudget:
			y.ItemsSkippedBudget++
			continue
		case ItemGenFail:
			y.ItemsGenFail++
		case ItemAssertFail:
			y.ItemsAssertFail++
		case ItemAssertError:
			y.ItemsAssertError++
		}
		entryAttempted[it.EntryID] = true
		if it.Usable {
			y.ItemsUsable++
			entryUsable[it.EntryID] = true
		}
		y.GpuSecondsTotal += it.GpuSeconds
	}
	for id := range entryAttempted {
		y.EntriesTotal++
		if entryUsable[id] {
			y.EntriesWithUsable++
		}
	}
	if y.EntriesTotal > 0 {
		y.YieldRatio = float64(y.EntriesWithUsable) / float64(y.EntriesTotal)
	}
	return y
}

// Run 执行套件：逐条生成 → 逐条断言（复用 qc 引擎）→ 预算截断（BUDGET-3）。
// 断言基础设施故障重试 ≤1 次（BUDGET-2），超限记 fail 并保留证据。
func Run(ctx context.Context, opts RunOptions) (*RunArtifact, error) {
	if err := opts.Suite.Validate(); err != nil {
		return nil, fmt.Errorf("eval: %w", err)
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	start := now()
	art := &RunArtifact{
		SchemaVersion: SchemaVersion, Suite: *opts.Suite,
		CapabilityProfileRef: opts.ProfileRef, RunnerMode: opts.RunnerMode,
		StartedAt: start.UTC().Format(time.RFC3339),
	}
	var gpuTotal float64
	for _, entry := range opts.Suite.Entries {
		for _, seed := range opts.Suite.Seeds {
			// BUDGET-3：开跑前查预算（wall 用真实耗时，gpu 用已计量累计）
			if elapsed := now().Sub(start).Seconds(); elapsed > opts.Suite.Budget.WallSeconds || gpuTotal > opts.Suite.Budget.GpuSeconds {
				art.Items = append(art.Items, ItemResult{
					EntryID: entry.ID, Seed: seed,
					Status: ItemSkippedBudget,
					Error:  fmt.Sprintf("预算截断：wall %.1fs/%.1fs gpu %.1fs/%.1fs", elapsed, opts.Suite.Budget.WallSeconds, gpuTotal, opts.Suite.Budget.GpuSeconds),
				})
				art.BudgetTruncated = true
				continue
			}
			art.Items = append(art.Items, runItem(ctx, opts, entry, seed))
			gpuTotal += art.Items[len(art.Items)-1].GpuSeconds
		}
	}
	art.FinishedAt = now().UTC().Format(time.RFC3339)
	art.Yield = ComputeYield(art.Items)
	if d, err := art.ComputeDigest(); err == nil {
		art.Digest = d
	}
	return art, nil
}

// runItem 执行一次抽卡：生成（gen op）→ 断言（qc 引擎，重试 ≤1）。
func runItem(ctx context.Context, opts RunOptions, entry Entry, seed int64) ItemResult {
	it := ItemResult{EntryID: entry.ID, Seed: seed}
	workdir := filepath.Join(opts.WorkdirRoot, entry.ID, fmt.Sprintf("seed-%d", seed))
	req := operator.Request{
		ContractVersion: contracts.ContractOperator,
		Op:              opts.Suite.Op,
		Inputs: map[string]any{
			"image_path":   entry.ImagePath,
			"prompt":       entry.Prompt,
			"duration_sec": entry.DurationSec,
			"fps":          opts.Suite.FPS,
		},
		Params:      withModel(opts.Suite.Params, opts.Suite.Model),
		Workdir:     workdir,
		Determinism: operator.Determinism{Seed: &seed},
	}
	resp, err := opts.Gen.Run(ctx, req)
	if err != nil {
		it.Status, it.Error = ItemGenFail, err.Error()
		return it
	}
	if resp.Status != operator.StatusOK {
		msg := "算子故障"
		if resp.Error != nil {
			msg = resp.Error.Message
		}
		it.Status, it.Error = ItemGenFail, fmt.Sprintf("%s: %s", resp.Status, msg)
		return it
	}
	it.VideoPath, _ = resp.Outputs["video_path"].(string)
	it.ContentHash, _ = resp.Outputs["content_hash"].(string)
	it.WallMs, it.GpuSeconds, it.PeakMemMB = resp.Metrics.WallMs, resp.Metrics.GpuSecond, resp.Metrics.PeakMemMB
	it.ModelVersions = resp.ModelVersions

	subj := &qc.Subject{
		MediaURI:  it.VideoPath,
		MediaHash: it.ContentHash,
		Fields: map[string]any{
			"gen_form": opts.Suite.GenForm, "model": opts.Suite.Model,
			"entry_id": entry.ID, "seed": seed,
		},
	}
	var rep *qc.Report
	for attempt := 0; attempt < 2; attempt++ { // BUDGET-2：重试 ≤1
		rep, err = opts.Engine.Run(ctx, subj, opts.Suite.AssertionPack)
		if err == nil {
			break
		}
	}
	if err != nil {
		it.Status, it.Error = ItemAssertError, err.Error()
		return it
	}
	it.Assertions = rep.Results
	if rep.Pass() {
		it.Status, it.Usable = ItemOK, true
	} else {
		it.Status = ItemAssertFail
	}
	return it
}

// ComputeDigest 对 artifact（除 Digest 自身）做 JCS 内容寻址摘要。
func (a *RunArtifact) ComputeDigest() (string, error) {
	copied := *a
	copied.Digest = ""
	return digest.ValueDigest(copied)
}

// Save 落盘 run artifact：文件名 = digest hex（可回查），返回路径。
func (a *RunArtifact) Save(outDir string) (string, error) {
	if a.Digest == "" {
		return "", fmt.Errorf("eval: artifact 未计算 digest")
	}
	raw, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(outDir, strings.TrimPrefix(a.Digest, "sha256:")+".json")
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// withModel 把 model 并入 params（套件级 params 不含 model——模型是套件
// 顶层字段，避免两处声明漂移）。
func withModel(params map[string]any, model string) map[string]any {
	p := map[string]any{}
	for k, v := range params {
		p[k] = v
	}
	p["model"] = model
	return p
}
