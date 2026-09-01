// pipeline.go 实现形态1 端到端编排（IR-0007 AC-6 / BEH-5）：
//
//	mock 商家（种子图 + 信息表）
//	  → gen_i2v（C2 算子，产物内容寻址）
//	  → BuildPlan（信息层六 overlay 逐字取自信息表，INV-5）
//	  → renderer.Render（真实解码 + drawtext 信息层 + AIGC 双轨）
//	  → 形态1 断言包（探针断言走 qc 引擎；信息层逐字一致按构造断言）
//	  → 内容寻址 run artifact（全链耗时分解：gen/render/assert）
package form1

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Cloudbird-Software/Shorts_Director/internal/compiler"
	"github.com/Cloudbird-Software/Shorts_Director/internal/contracts"
	"github.com/Cloudbird-Software/Shorts_Director/internal/digest"
	"github.com/Cloudbird-Software/Shorts_Director/internal/eval"
	"github.com/Cloudbird-Software/Shorts_Director/internal/operator"
	"github.com/Cloudbird-Software/Shorts_Director/internal/qc"
	"github.com/Cloudbird-Software/Shorts_Director/internal/renderer"
	"github.com/Cloudbird-Software/Shorts_Director/internal/videoplan"
)

// SchemaVersion 是 form1 run artifact 的结构版本。
const SchemaVersion = 1

// ConsumedGoldenOps 声明本包消费 golden 契约的算子（Freeze Gate G7 锚点）：
// internal/operator 的 golden 清单测试据此校验本包 Outputs 字面访问 ⊆
// testdata/golden 清单——上游删改输出字段时此处失败，而不是静默读零值。
var ConsumedGoldenOps = []string{"gen_i2v"}

// Options 是管线执行的注入面。
type Options struct {
	Suite       *eval.Suite          // 条目（种子图/prompt/时长）与生成参数
	Merchants   map[string]*Merchant // entry_id → 商家信息表
	Gen         operator.Runner      // 生成算子执行器
	Engine      *qc.Engine           // 断言引擎（探针已注册）
	Font        compiler.Font        // 信息层字体（R-4 哈希核验）
	RunnerMode  string               // fake|local|docker
	ProfileRef  string               // capability profile 内容寻址引用
	WorkdirRoot string               // 逐条目工作目录根
	// Root 是套件相对路径（种子图）的解析根，默认当前目录。
	Root string
	// Date 是确定性锚（YYYY-MM-DD）：ScheduledDate / 隐式标识 ProduceTime /
	// Provenance.CreatedAt。生产取实验日，测试取固定值。
	Date string
	// Seeds 覆盖套件 seed 集（默认取套件首个 seed——AC-6 每商家 ≥1 条）。
	Seeds []int64
	// RendererExpect 版本钉死清单（R-5）。
	RendererExpect compiler.RendererExpect
}

// Timing 是全链耗时分解（毫秒，墙上时钟——只进指标不进内容）。
type Timing struct {
	GenMs    int64 `json:"gen_ms"`
	RenderMs int64 `json:"render_ms"`
	AssertMs int64 `json:"assert_ms"`
	TotalMs  int64 `json:"total_ms"`
}

// ItemResult 是单条成品的判定明细（复用 eval 条目形态 + 管线特有字段）。
type ItemResult struct {
	eval.ItemResult
	Timing      Timing `json:"timing"`
	PlanDigest  string `json:"plan_digest,omitempty"`
	MerchantRef string `json:"merchant_ref,omitempty"` // 信息表路径（逐字一致断言的证据）
}

// Artifact 是一次形态1 管线执行的完整产物（内容寻址）。
type Artifact struct {
	SchemaVersion int          `json:"schema_version"`
	Suite         eval.Suite   `json:"suite"` // 全文内嵌（IFACE-2 口径）
	RunnerMode    string       `json:"runner_mode"`
	ProfileRef    string       `json:"capability_profile_ref,omitempty"`
	Date          string       `json:"date"`
	FontFamily    string       `json:"font_family"`
	FontHash      string       `json:"font_hash"`
	Items         []ItemResult `json:"items"`
	Yield         eval.Yield   `json:"yield"`
	Digest        string       `json:"digest,omitempty"`
}

// Run 执行形态1 端到端管线：逐商家逐 seed 生成→渲染→断言。
func Run(ctx context.Context, opts Options) (*Artifact, error) {
	if err := opts.Suite.Validate(); err != nil {
		return nil, fmt.Errorf("form1: %w", err)
	}
	if opts.Date == "" {
		return nil, fmt.Errorf("form1: Date 必填（确定性锚，YYYY-MM-DD）")
	}
	if opts.Font.Family == "" || opts.Font.Path == "" || opts.Font.Hash == "" {
		return nil, fmt.Errorf("form1: Font 三元组必填（family/path/hash——R-4）")
	}
	if opts.RendererExpect == (compiler.RendererExpect{}) {
		return nil, fmt.Errorf("form1: RendererExpect 必填（R-5 版本钉死）")
	}
	root := opts.Root
	if root == "" {
		root = "."
	}
	seeds := opts.Seeds
	if len(seeds) == 0 {
		seeds = opts.Suite.Seeds[:1] // AC-6：每商家至少 1 条
	}
	art := &Artifact{
		SchemaVersion: SchemaVersion, Suite: *opts.Suite,
		RunnerMode: opts.RunnerMode, ProfileRef: opts.ProfileRef,
		Date: opts.Date, FontFamily: opts.Font.Family, FontHash: opts.Font.Hash,
	}
	for _, entry := range opts.Suite.Entries {
		m, ok := opts.Merchants[entry.ID]
		if !ok {
			return nil, fmt.Errorf("form1: 条目 %s 无对应商家信息表", entry.ID)
		}
		for _, seed := range seeds {
			art.Items = append(art.Items, runItem(ctx, opts, entry, m, seed, root))
		}
	}
	art.Yield = eval.ComputeYield(evalItems(art.Items))
	if d, err := art.ComputeDigest(); err == nil {
		art.Digest = d
	}
	return art, nil
}

// runItem 执行一条：生成 → plan 构造 → 渲染 → 断言（探针 + 构造）。
func runItem(ctx context.Context, opts Options, entry eval.Entry, m *Merchant, seed int64, root string) ItemResult {
	total0 := time.Now()
	it := ItemResult{ItemResult: eval.ItemResult{EntryID: entry.ID, Seed: seed}}
	it.MerchantRef = m.Path
	workdir := filepath.Join(opts.WorkdirRoot, entry.ID, fmt.Sprintf("seed-%d", seed))
	outPath := filepath.Join(workdir, "final.mp4")

	// 1) 生成（gen_i2v）
	imgPath, err := filepath.Abs(filepath.Join(root, entry.ImagePath))
	if err != nil {
		it.Status, it.Error = eval.ItemGenFail, err.Error()
		return it
	}
	gen0 := time.Now()
	resp, err := opts.Gen.Run(ctx, operator.Request{
		ContractVersion: contracts.ContractOperator,
		Op:              opts.Suite.Op,
		Inputs: map[string]any{
			"image_path":   imgPath,
			"prompt":       entry.Prompt,
			"duration_sec": entry.DurationSec,
			"fps":          opts.Suite.FPS,
		},
		Params:      withModel(opts.Suite.Params, opts.Suite.Model),
		Workdir:     filepath.Join(workdir, "gen"),
		Determinism: operator.Determinism{Seed: &seed},
	})
	it.Timing.GenMs = time.Since(gen0).Milliseconds()
	if err != nil {
		it.Status, it.Error = eval.ItemGenFail, err.Error()
		return it
	}
	if resp.Status != operator.StatusOK {
		msg := "算子故障"
		if resp.Error != nil {
			msg = resp.Error.Message
		}
		it.Status, it.Error = eval.ItemGenFail, fmt.Sprintf("%s: %s", resp.Status, msg)
		return it
	}
	genPath, _ := resp.Outputs["video_path"].(string)
	genHash, _ := resp.Outputs["content_hash"].(string)
	it.GpuSeconds, it.PeakMemMB = resp.Metrics.GpuSecond, resp.Metrics.PeakMemMB
	it.ModelVersions = resp.ModelVersions
	if genPath == "" || genHash == "" {
		it.Status, it.Error = eval.ItemGenFail, "算子 OK 但缺 video_path/content_hash"
		return it
	}

	// 2) plan 构造（信息层逐字取自信息表——INV-5）
	frames := int(entry.DurationSec * CanvasFPS)
	plan, err := BuildPlan(planInput{
		Merchant: m, GenHash: genHash, Frames: frames,
		SuiteID: opts.Suite.SuiteID, Seed: seed,
		Date: opts.Date, FontFamily: opts.Font.Family,
	})
	if err != nil {
		it.Status, it.Error = eval.ItemGenFail, err.Error()
		return it
	}
	if d, err := digest.ValueDigest(plan); err == nil {
		it.PlanDigest = d
	}

	// 3) 渲染（真实解码 + 信息层 + AIGC 双轨）
	render0 := time.Now()
	req, err := compiler.Compile(plan,
		compiler.MediaIndex{"generated:gen-0001": {
			LocalPath: genPath, ContentHash: genHash, FPS: opts.Suite.FPS}},
		[]compiler.Font{opts.Font},
		compiler.Output{Path: outPath, Codec: "h264", CRF: 28, Preset: "veryfast"},
		compiler.Modes{Deterministic: true}, opts.RendererExpect)
	if err == nil {
		err = renderer.Render(req, nil)
	}
	it.Timing.RenderMs = time.Since(render0).Milliseconds()
	if err != nil {
		it.Status, it.Error = eval.ItemGenFail, "渲染失败: "+err.Error()
		return it
	}
	it.VideoPath = outPath
	od, err := fileDigest(outPath)
	if err != nil {
		it.Status, it.Error = eval.ItemGenFail, err.Error()
		return it
	}
	it.ContentHash = od

	// 4) 断言：探针断言（qc 引擎，重试 ≤1）+ 信息层构造断言
	assert0 := time.Now()
	subj := &qc.Subject{
		MediaURI: outPath, MediaHash: od,
		Spec: map[string]any{"ref_media_path": genPath},
		Fields: map[string]any{"gen_form": opts.Suite.GenForm, "model": opts.Suite.Model,
			"entry_id": entry.ID, "seed": seed},
	}
	var rep *qc.Report
	for attempt := 0; attempt < 2; attempt++ {
		rep, err = opts.Engine.Run(ctx, subj, assertionPack())
		if err == nil {
			break
		}
	}
	if err != nil {
		it.Status, it.Error = eval.ItemAssertError, err.Error()
		it.Timing.TotalMs = time.Since(total0).Milliseconds()
		return it
	}
	it.Assertions = append(rep.Results, infoLayerResults(plan, m)...)
	it.Timing.AssertMs = time.Since(assert0).Milliseconds()
	it.Timing.TotalMs = time.Since(total0).Milliseconds()
	if rep.Pass() && infoLayerPass(it.Assertions) {
		it.Status, it.Usable = eval.ItemOK, true
	} else {
		it.Status = eval.ItemAssertFail
	}
	return it
}

// infoLayerPass 判定信息层构造断言是否全过（探针断言已由 rep.Pass() 覆盖）。
func infoLayerPass(results []qc.Result) bool {
	for _, r := range results {
		if !r.Pass && r.Skipped == "" && strings.HasPrefix(r.AssertionID, "L1.info_layer.") {
			return false
		}
	}
	return true
}

// assertionPack 是形态1 成品的机器可判定断言包（evals/suites/form1_ambience.json
// 的成品口径）：时长区间/画幅/黑帧走 L0 探针；AIGC 双轨走 L3 探针。
// aigc_overlay_present 的区域与 plan.go aigcBox 钉死同值；生成源路径经
// {{ref_media_path}} 从被检对象 Spec 注入（引擎把 Spec 展平为模板变量域）。
func assertionPack() []qc.Assertion {
	remedy := func(action, tpl string) qc.Remedy {
		return qc.Remedy{Action: action, InstructionTemplate: tpl}
	}
	return []qc.Assertion{
		{
			AssertionID: "L0.duration_in_form_range", Level: qc.L0, Severity: qc.SeverityBlocker,
			Probe:  qc.Probe{Op: "ffprobe_field", Args: map[string]any{"field": "duration_sec"}},
			Expect: qc.Expect{Op: "between", Value: []any{5.0, 8.0}},
			Remedy: remedy("TRIM", "时长 {{measured}}s 超出形态区间 5–8s，裁剪或重抽"),
		},
		{
			AssertionID: "L0.canvas_width", Level: qc.L0, Severity: qc.SeverityBlocker,
			Probe:  qc.Probe{Op: "resolution", Args: map[string]any{"dim": "width"}},
			Expect: qc.Expect{Op: "eq", Value: 1080},
			Remedy: remedy("RE_CROP", "宽 {{measured}} ≠ 1080，按 9:16 画幅重裁"),
		},
		{
			AssertionID: "L0.canvas_height", Level: qc.L0, Severity: qc.SeverityBlocker,
			Probe:  qc.Probe{Op: "resolution", Args: map[string]any{"dim": "height"}},
			Expect: qc.Expect{Op: "eq", Value: 1920},
			Remedy: remedy("RE_CROP", "高 {{measured}} ≠ 1920，按 9:16 画幅重裁"),
		},
		{
			AssertionID: "L0.black_ratio", Level: qc.L0, Severity: qc.SeverityMajor,
			Probe:  qc.Probe{Op: "blackdetect_ratio", Args: map[string]any{}},
			Expect: qc.Expect{Op: "lte", Value: 0.01},
			Remedy: remedy("REGENERATE", "黑帧占比 {{measured}}，生成异常需重抽"),
		},
		{
			AssertionID: "L3.aigc_disclosure_overlay", Level: qc.L3, Severity: qc.SeverityBlocker,
			Probe: qc.Probe{Op: "aigc_overlay_present", Args: map[string]any{
				"x": float64(aigcBox.X), "y": float64(aigcBox.Y),
				"w": float64(aigcBox.W), "h": float64(aigcBox.H),
				"frame":          float64(12),
				"ref_media_path": "{{ref_media_path}}",
			}},
			Expect: qc.Expect{Op: "gte", Value: 0.005},
			Remedy: remedy("ADD_DISCLAIMER", "AIGC 角标区域差异占比 {{measured}}，披露文案未叠加"),
		},
		{
			AssertionID: "L3.aigc_implicit_metadata", Level: qc.L3, Severity: qc.SeverityBlocker,
			Probe:  qc.Probe{Op: "aigc_metadata_present", Args: map[string]any{}},
			Expect: qc.Expect{Op: "is_true", Value: true},
			Remedy: remedy("ADD_DISCLAIMER", "容器元数据缺完整 AIGC 隐式标识块"),
		},
	}
}

// infoLayerResults 信息层逐字一致断言（INV-5 构造级判定）：overlay 文本
// 必须与信息表字段逐字相等。证据 = 信息表路径；「文本真的进了画面」由
// R-1 bitexact（渲染确定性）与 aigc_overlay_present 探针共同背书。
func infoLayerResults(p videoplan.Plan, m *Merchant) []qc.Result {
	checks := []struct {
		id, want string
	}{
		{"L1.info_layer.shop_name", m.Info.ShopName},
		{"L1.info_layer.signature_item", m.Info.SignatureItem},
		{"L1.info_layer.price", m.Info.Price},
		{"L1.info_layer.address", m.Info.Address},
		{"L1.info_layer.phone", m.Info.Phone},
		{"L1.info_layer.aigc_disclosure", m.AIGCDisclosure},
	}
	overlayOf := map[string]string{
		"L1.info_layer.shop_name":       "ov_shop",
		"L1.info_layer.signature_item":  "ov_signature",
		"L1.info_layer.price":           "ov_price",
		"L1.info_layer.address":         "ov_address",
		"L1.info_layer.phone":           "ov_phone",
		"L1.info_layer.aigc_disclosure": "ov_aigc",
	}
	byID := map[string]string{}
	for _, o := range p.Overlays {
		if t, ok := o.Props["text"].(string); ok {
			byID[o.OverlayID] = t
		}
	}
	out := make([]qc.Result, 0, len(checks))
	for _, c := range checks {
		got := byID[overlayOf[c.id]]
		out = append(out, qc.Result{
			AssertionID: c.id, Level: qc.L1, Severity: qc.SeverityBlocker,
			Pass: got == c.want, Measured: got, Expected: c.want,
			EvidenceURI: m.Path,
		})
	}
	return out
}

// evalItems 把管线条目适配成 eval 条目（出片率口径唯一入口复用）。
func evalItems(items []ItemResult) []eval.ItemResult {
	out := make([]eval.ItemResult, len(items))
	for i, it := range items {
		out[i] = it.ItemResult
	}
	return out
}

// ComputeDigest 对 artifact（除 Digest 自身）做 JCS 内容寻址摘要。
func (a *Artifact) ComputeDigest() (string, error) {
	copied := *a
	copied.Digest = ""
	return digest.ValueDigest(copied)
}

// Save 落盘 run artifact：文件名 = digest hex，返回路径。
func (a *Artifact) Save(outDir string) (string, error) {
	if a.Digest == "" {
		return "", fmt.Errorf("form1: artifact 未计算 digest")
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

// withModel 把 model 并入 params（与 internal/eval 同语义）。
func withModel(params map[string]any, model string) map[string]any {
	p := map[string]any{}
	for k, v := range params {
		p[k] = v
	}
	p["model"] = model
	return p
}

// fileDigest 是成品 mp4 的 sha256。
func fileDigest(path string) (string, error) {
	bs, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(bs)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}
