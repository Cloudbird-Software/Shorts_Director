// Package report 聚合 run artifact → 出片率实测报告 + 产能外推
// （IR-0007 AC-5/AC-9/AC-10 / BEH-4 / BEH-8 / DECISION-5/DECISION-6，卡 #122）。
//
// 口径：
//   - 出片率数字一律从 artifact 逐条明细复算（eval.ComputeYield），
//     与 artifact 内嵌聚合不一致 = 硬错误（AC-5「一致可复算」以失败执法）；
//   - 产能外推只以实测耗时/资源计量为基（DECISION-6），显式标注估算，
//     不引入电价变量；A100 只给迁移口径说明，不做换算倍率（DECISION-5）。
package report

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/Cloudbird-Software/Shorts_Director/internal/digest"
	"github.com/Cloudbird-Software/Shorts_Director/internal/eval"
)

// SchemaVersion 是出片率报告的结构版本。
const SchemaVersion = 1

// daySeconds 是外推基准（单卡串行日历日）。
const daySeconds = 86400.0

// artifactDoc 是任意 run artifact（eval / form1 / form4）的统一解码视图。
// 三类 artifact 共享 eval 口径字段（suite/items/yield/digest）；形态管线
// 特有字段（timing/font 等）按需提取，未知字段忽略。
type artifactDoc struct {
	SchemaVersion int        `json:"schema_version"`
	Suite         eval.Suite `json:"suite"`
	RunnerMode    string     `json:"runner_mode"`
	ProfileRef    string     `json:"capability_profile_ref"`
	Items         []docItem  `json:"items"`
	Yield         eval.Yield `json:"yield"`
	Digest        string     `json:"digest"`
}

// docItem 是逐条明细（eval.ItemResult 展开 + 全链耗时）。
type docItem struct {
	eval.ItemResult
	Timing struct {
		TotalMs int64 `json:"total_ms"`
	} `json:"timing"`
}

// totalMs 是单条全链耗时（毫秒）：形态管线取 Timing.TotalMs（生成+渲染+
// 断言），纯生成套件回落 WallMs。
func (d docItem) totalMs() int64 {
	if d.Timing.TotalMs > 0 {
		return d.Timing.TotalMs
	}
	return d.WallMs
}

// ItemDetail 是报告内嵌的逐条判定明细（BEH-4：报告必须含逐条明细）。
type ItemDetail struct {
	eval.ItemResult
	TotalMs int64 `json:"total_ms"`
}

// SuiteReport 是单个 run artifact 的报告视图。
type SuiteReport struct {
	ArtifactPath     string       `json:"artifact_path"`
	ArtifactDigest   string       `json:"artifact_digest"`
	SuiteID          string       `json:"suite_id"`
	GenForm          string       `json:"gen_form"`
	Model            string       `json:"model"`
	RunnerMode       string       `json:"runner_mode"`
	ProfileRef       string       `json:"capability_profile_ref"`
	Items            []ItemDetail `json:"items"`       // 逐条判定明细（复算口径）
	Yield            eval.Yield   `json:"yield"`       // 复算聚合出片率
	AvgItemMs        float64      `json:"avg_item_ms"` // 可用条目平均全链耗时
	WallSecondsTotal float64      `json:"wall_seconds_total"`
	GpuSecondsTotal  float64      `json:"gpu_seconds_total"`
}

// Totals 是跨套件合计。
type Totals struct {
	Suites          int     `json:"suites"`
	EntriesTotal    int     `json:"entries_total"`
	EntriesUsable   int     `json:"entries_usable"`
	ItemsTotal      int     `json:"items_total"`
	ItemsUsable     int     `json:"items_usable"`
	GpuSecondsTotal float64 `json:"gpu_seconds_total"`
}

// Capacity 是产能外推（AC-10：显式标注估算、无电价变量、A100 迁移口径）。
type Capacity struct {
	Estimated  bool   `json:"estimated"`   // 恒 true——所有数字均为估算
	Unit       string `json:"unit"`        // 条/日（可用成品，单卡串行）
	DailyLow   int    `json:"daily_low"`   // 86400s ÷ 最慢单条
	DailyHigh  int    `json:"daily_high"`  // 86400s ÷ 最快单条
	BasisItems int    `json:"basis_items"` // 外推基样条数（可用条目）
	Method     string `json:"method"`
	A100Note   string `json:"a100_note"`         // DECISION-5 迁移口径
	CostNote   string `json:"cost_note"`         // DECISION-6 成本口径
	Warning    string `json:"warning,omitempty"` // fake 后端计时等警示
}

// Report 是一次聚合的完整产物（内容寻址）。
type Report struct {
	SchemaVersion int           `json:"schema_version"`
	Date          string        `json:"date"`
	ProfileRef    string        `json:"capability_profile_ref"` // 实验机 profile digest
	ProfilePath   string        `json:"capability_profile_path"`
	Suites        []SuiteReport `json:"suites"`
	Totals        Totals        `json:"totals"`
	Capacity      Capacity      `json:"capacity"`
	Digest        string        `json:"digest,omitempty"`
}

// Options 是报告构建的注入面。
type Options struct {
	ArtifactPaths []string
	ProfilePath   string // capability profile JSON（digest 引用来源，必填）
	Date          string // YYYY-MM-DD 确定性锚
}

// Build 聚合 run artifacts：加载 → 复算出片率（不一致即失败）→ 外推产能。
func Build(opts Options) (*Report, error) {
	if len(opts.ArtifactPaths) == 0 {
		return nil, fmt.Errorf("report: ArtifactPaths 必填非空")
	}
	if opts.ProfilePath == "" {
		return nil, fmt.Errorf("report: ProfilePath 必填（AC-5：报告显式标注 capability profile 引用）")
	}
	profileRef, err := profileDigest(opts.ProfilePath)
	if err != nil {
		return nil, fmt.Errorf("report: %w", err)
	}
	rep := &Report{
		SchemaVersion: SchemaVersion, Date: opts.Date,
		ProfileRef: profileRef, ProfilePath: opts.ProfilePath,
	}
	var usableSecs []float64
	fakeBasis := false
	for _, p := range opts.ArtifactPaths {
		sr, err := loadSuiteReport(p)
		if err != nil {
			return nil, err
		}
		if sr.ProfileRef == "" {
			return nil, fmt.Errorf("report: %s 缺 capability_profile_ref（run 须带 -profile 执行）", p)
		}
		if sr.ProfileRef != profileRef {
			return nil, fmt.Errorf("report: %s 的 profile 引用 %s 与报告 profile %s 不符",
				p, sr.ProfileRef, profileRef)
		}
		rep.Suites = append(rep.Suites, *sr)
		rep.Totals.EntriesTotal += sr.Yield.EntriesTotal
		rep.Totals.EntriesUsable += sr.Yield.EntriesWithUsable
		rep.Totals.ItemsTotal += sr.Yield.ItemsTotal
		rep.Totals.ItemsUsable += sr.Yield.ItemsUsable
		rep.Totals.GpuSecondsTotal += sr.Yield.GpuSecondsTotal
		if sr.RunnerMode == "fake" || sr.Model == "fake" {
			fakeBasis = true
		}
		for _, it := range sr.Items {
			if it.Usable && it.TotalMs > 0 {
				usableSecs = append(usableSecs, float64(it.TotalMs)/1000.0)
			}
		}
	}
	rep.Totals.Suites = len(rep.Suites)
	rep.Capacity = extrapolate(usableSecs, fakeBasis)
	if d, err := rep.ComputeDigest(); err == nil {
		rep.Digest = d
	}
	return rep, nil
}

// extrapolate 以可用条目实测全链耗时外推日产能区间（单卡串行口径）。
func extrapolate(usableSecs []float64, fakeBasis bool) Capacity {
	c := Capacity{
		Estimated:  true,
		Unit:       "条/日（可用成品，单卡串行）",
		BasisItems: len(usableSecs),
		Method: "单卡串行口径：日产能区间 = 86400s ÷ 实测单条全链耗时" +
			"（下界取最慢单条、上界取最快单条）；全部数字为估算，非实测直读。",
		A100Note: "向 A100 对照迁移的口径（DECISION-5）：套件定义硬件无关，A100 到位后" +
			"同套件全量重跑（doctor→E1→E2/E3→E7），以 capability profile 与 run artifact " +
			"环境签名自动产出双机 diff；V100 阶段不为 A100 预做代码特化，本报告不设 A100 换算倍率。",
		CostNote: "成本口径（DECISION-6）：月租含电价——本报告只以实测耗时与资源计量外推产能，" +
			"不引入电价变量；gpu_seconds 为 0 时代表 fake 后端无 GPU 计量。",
	}
	if len(usableSecs) == 0 {
		c.Warning = "无可用条目（或耗时计量缺失），无法外推日产能——daily_low/daily_high 为 0。"
		return c
	}
	minS, maxS := math.MaxFloat64, -math.MaxFloat64
	for _, s := range usableSecs {
		minS = math.Min(minS, s)
		maxS = math.Max(maxS, s)
	}
	c.DailyLow = int(daySeconds / maxS)
	c.DailyHigh = int(daySeconds / minS)
	if fakeBasis {
		c.Warning = "外推基含 fake 后端计时（管道联调口径），数字仅供仪器演练，" +
			"不代表真实模型产能——真实外推须在实验机以真实后端重跑同套件。"
	}
	return c
}

// loadSuiteReport 加载单个 run artifact 并复算聚合（AC-5 一致性执法点）。
func loadSuiteReport(path string) (*SuiteReport, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("report: 读 artifact 失败: %w", err)
	}
	var doc artifactDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("report: %s 不是合法 JSON: %w", path, err)
	}
	if doc.Digest == "" {
		return nil, fmt.Errorf("report: %s 缺 digest（须为管线 CLI 内容寻址落盘产物）", path)
	}
	// 内容寻址完整性：文件名 = digest hex（管线 Save 的落盘约定）。
	want := strings.TrimPrefix(doc.Digest, "sha256:") + ".json"
	if filepath.Base(path) != want {
		return nil, fmt.Errorf("report: %s 文件名与 digest 不符（期望 %s）", path, want)
	}
	items := make([]eval.ItemResult, len(doc.Items))
	details := make([]ItemDetail, len(doc.Items))
	var totalMs int64
	for i, d := range doc.Items {
		items[i] = d.ItemResult
		details[i] = ItemDetail{ItemResult: d.ItemResult, TotalMs: d.totalMs()}
		if d.Usable {
			totalMs += d.totalMs()
		}
	}
	y := eval.ComputeYield(items)
	if y != doc.Yield {
		return nil, fmt.Errorf("report: %s 出片率复算不一致（复算 %+v ≠ 内嵌 %+v）——"+
			"报告与 run artifact 明细必须一致（AC-5）", path, y, doc.Yield)
	}
	sr := &SuiteReport{
		ArtifactPath: path, ArtifactDigest: doc.Digest,
		SuiteID: doc.Suite.SuiteID, GenForm: doc.Suite.GenForm,
		Model: doc.Suite.Model, RunnerMode: doc.RunnerMode,
		ProfileRef: doc.ProfileRef, Items: details, Yield: y,
		WallSecondsTotal: float64(sumWallMs(doc.Items)) / 1000.0,
		GpuSecondsTotal:  y.GpuSecondsTotal,
	}
	if y.ItemsUsable > 0 {
		sr.AvgItemMs = float64(totalMs) / float64(y.ItemsUsable)
	}
	return sr, nil
}

// sumWallMs 汇总逐条全链耗时。
func sumWallMs(items []docItem) int64 {
	var t int64
	for _, d := range items {
		t += d.totalMs()
	}
	return t
}

// profileDigest 提取 capability profile 的内容寻址 digest。
func profileDigest(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("读 profile 失败: %w", err)
	}
	var p struct {
		Digest string `json:"digest"`
	}
	if err := json.Unmarshal(raw, &p); err != nil || p.Digest == "" {
		return "", fmt.Errorf("profile %s 缺 digest 字段（须为 make doctor 产物）", path)
	}
	return p.Digest, nil
}

// ComputeDigest 对报告（除 Digest 自身）做 JCS 内容寻址摘要。
func (r *Report) ComputeDigest() (string, error) {
	copied := *r
	copied.Digest = ""
	return digest.ValueDigest(copied)
}

// Save 落盘报告：文件名 = digest hex，返回路径。
func (r *Report) Save(outDir string) (string, error) {
	if r.Digest == "" {
		return "", fmt.Errorf("report: 报告未计算 digest")
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
