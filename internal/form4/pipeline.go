// pipeline.go 实现形态4（数字人口播）端到端编排（IR-0007 AC-7 / BEH-6）：
//
//	mock 商家（人像照 + 口播文案 + 三要素脚本 + 信息表）
//	  → gen_tts（口播语音，产物内容寻址）
//	  → gen_lipsync（人像照 × 语音 → 口播视频）
//	  → transcribe（对口播语音独立转写，验证三要素齐全：品牌/卖点/CTA）
//	  → BuildPlan（品牌名信息层逐字取自脚本，INV-5）
//	  → renderer.RenderVO（真实解码 + 信息层 + AIGC 双轨 + 口播音轨保留）
//	  → 形态4 断言包（探针断言走 qc 引擎；转写三要素与信息层按构造断言）
//	  → 内容寻址 run artifact（全链耗时分解：tts/lipsync/render/assert）
package form4

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
	"github.com/Cloudbird-Software/Shorts_Director/internal/form1"
	"github.com/Cloudbird-Software/Shorts_Director/internal/operator"
	"github.com/Cloudbird-Software/Shorts_Director/internal/qc"
	"github.com/Cloudbird-Software/Shorts_Director/internal/renderer"
	"github.com/Cloudbird-Software/Shorts_Director/internal/videoplan"
)

// SchemaVersion 是 form4 run artifact 的结构版本。
const SchemaVersion = 1

// ConsumedGoldenOps 声明本包消费 golden 契约的算子（Freeze Gate G7 锚点）：
// internal/operator 的 golden 清单测试据此校验本包 Outputs 字面访问 ⊆
// testdata/golden 清单——上游删改输出字段时此处失败，而不是静默读零值。
var ConsumedGoldenOps = []string{"gen_tts", "gen_lipsync", "transcribe"}

// Options 是管线执行的注入面。
type Options struct {
	Suite      *eval.Suite                // 条目（人像照/口播文案）与生成参数
	Merchants  map[string]*form1.Merchant // entry_id → 商家信息表（AIGC 文案来源）
	Scripts    map[string]*Script         // entry_id → 口播脚本（三要素期望锚）
	TTS        operator.Runner            // gen_tts 执行器
	Lipsync    operator.Runner            // gen_lipsync 执行器
	Transcribe operator.Runner            // transcribe 执行器（转写三要素验证）
	// TranscribeModel 是转写后端键（fake|whisper）；空取 fake。
	// fake 后端透传 text_hint（联调冒烟），真实链路用 whisper 独立转写。
	TranscribeModel string
	Engine          *qc.Engine // 断言引擎（探针已注册，含 lipsync_lse_c/d）
	Font            compiler.Font
	RunnerMode      string
	ProfileRef      string // capability profile 内容寻址引用
	WorkdirRoot     string
	// Root 是套件相对路径（人像照）的解析根，默认当前目录。
	Root string
	// Date 是确定性锚（YYYY-MM-DD）。
	Date string
	// Seeds 覆盖套件 seed 集（默认取套件首个 seed——AC-7 每商家 ≥1 条）。
	Seeds []int64
	// RendererExpect 版本钉死清单（R-5）。
	RendererExpect compiler.RendererExpect
}

// Timing 是全链耗时分解（毫秒，墙上时钟——只进指标不进内容）。
type Timing struct {
	TTSMs     int64 `json:"tts_ms"`
	LipsyncMs int64 `json:"lipsync_ms"`
	RenderMs  int64 `json:"render_ms"`
	AssertMs  int64 `json:"assert_ms"`
	TotalMs   int64 `json:"total_ms"`
}

// ItemResult 是单条成品的判定明细（复用 eval 条目形态 + 管线特有字段）。
type ItemResult struct {
	eval.ItemResult
	Timing          Timing `json:"timing"`
	PlanDigest      string `json:"plan_digest,omitempty"`
	ScriptRef       string `json:"script_ref,omitempty"`       // 三要素脚本路径（断言证据）
	TranscribedText string `json:"transcribed_text,omitempty"` // 转写文本（三要素判定对象）
	AudioHash       string `json:"audio_hash,omitempty"`       // VO 产物内容寻址
}

// Artifact 是一次形态4 管线执行的完整产物（内容寻址）。
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

// Run 执行形态4 端到端管线：逐商家逐 seed 语音合成→口型同步→转写→渲染→断言。
func Run(ctx context.Context, opts Options) (*Artifact, error) {
	if err := opts.Suite.Validate(); err != nil {
		return nil, fmt.Errorf("form4: %w", err)
	}
	if opts.Date == "" {
		return nil, fmt.Errorf("form4: Date 必填（确定性锚，YYYY-MM-DD）")
	}
	if opts.Font.Family == "" || opts.Font.Path == "" || opts.Font.Hash == "" {
		return nil, fmt.Errorf("form4: Font 三元组必填（family/path/hash——R-4）")
	}
	if opts.RendererExpect == (compiler.RendererExpect{}) {
		return nil, fmt.Errorf("form4: RendererExpect 必填（R-5 版本钉死）")
	}
	if opts.Suite.Op != "gen_lipsync" {
		return nil, fmt.Errorf("form4: 套件 op 必须 gen_lipsync（得到 %q）", opts.Suite.Op)
	}
	root := opts.Root
	if root == "" {
		root = "."
	}
	seeds := opts.Seeds
	if len(seeds) == 0 {
		seeds = opts.Suite.Seeds[:1] // AC-7：每商家至少 1 条
	}
	art := &Artifact{
		SchemaVersion: SchemaVersion, Suite: *opts.Suite,
		RunnerMode: opts.RunnerMode, ProfileRef: opts.ProfileRef,
		Date: opts.Date, FontFamily: opts.Font.Family, FontHash: opts.Font.Hash,
	}
	for _, entry := range opts.Suite.Entries {
		m, ok := opts.Merchants[entry.ID]
		if !ok {
			return nil, fmt.Errorf("form4: 条目 %s 无对应商家信息表", entry.ID)
		}
		s, ok := opts.Scripts[entry.ID]
		if !ok {
			return nil, fmt.Errorf("form4: 条目 %s 无对应口播脚本（form4.json）", entry.ID)
		}
		for _, seed := range seeds {
			art.Items = append(art.Items, runItem(ctx, opts, entry, m, s, seed, root))
		}
	}
	art.Yield = eval.ComputeYield(evalItems(art.Items))
	if d, err := art.ComputeDigest(); err == nil {
		art.Digest = d
	}
	return art, nil
}

// runItem 执行一条：TTS → lipsync → 转写 → plan 构造 → 渲染 → 断言。
func runItem(ctx context.Context, opts Options, entry eval.Entry,
	m *form1.Merchant, s *Script, seed int64, root string) ItemResult {
	total0 := time.Now()
	it := ItemResult{ItemResult: eval.ItemResult{EntryID: entry.ID, Seed: seed}}
	it.ScriptRef = s.Path
	workdir := filepath.Join(opts.WorkdirRoot, entry.ID, fmt.Sprintf("seed-%d", seed))
	outPath := filepath.Join(workdir, "final.mp4")

	portrait, err := filepath.Abs(filepath.Join(root, entry.ImagePath))
	if err != nil {
		it.Status, it.Error = eval.ItemGenFail, err.Error()
		return it
	}

	// 1) 语音合成（gen_tts）——时间线时长由口播时长决定（视频跟着声音走）
	tts0 := time.Now()
	resp, err := opts.TTS.Run(ctx, operator.Request{
		ContractVersion: contracts.ContractOperator,
		Op:              "gen_tts",
		Inputs:          map[string]any{"text": entry.Prompt},
		Params:          withModel(opts.Suite.Params, opts.Suite.Model),
		Workdir:         filepath.Join(workdir, "tts"),
		Determinism:     operator.Determinism{Seed: &seed},
	})
	it.Timing.TTSMs = time.Since(tts0).Milliseconds()
	if err != nil {
		it.Status, it.Error = eval.ItemGenFail, err.Error()
		return it
	}
	if resp.Status != operator.StatusOK {
		it.Status, it.Error = eval.ItemGenFail, failMsg(resp)
		return it
	}
	audioPath, _ := resp.Outputs["audio_path"].(string)
	audioHash, _ := resp.Outputs["content_hash"].(string)
	durationSec, _ := resp.Outputs["duration_sec"].(float64)
	it.AudioHash = audioHash
	it.GpuSeconds, it.PeakMemMB = resp.Metrics.GpuSecond, resp.Metrics.PeakMemMB
	it.ModelVersions = resp.ModelVersions
	if audioPath == "" || audioHash == "" || durationSec <= 0 {
		it.Status, it.Error = eval.ItemGenFail,
			fmt.Sprintf("gen_tts OK 但缺 audio_path/content_hash/duration_sec（%v）", resp.Outputs)
		return it
	}
	frames := int(durationSec * CanvasFPS)
	if frames < 1 {
		it.Status, it.Error = eval.ItemGenFail,
			fmt.Sprintf("口播时长 %vs 不足 1 帧", durationSec)
		return it
	}

	// 2) 口型同步（gen_lipsync）：人像照 × 语音 → 口播视频
	lip0 := time.Now()
	resp, err = opts.Lipsync.Run(ctx, operator.Request{
		ContractVersion: contracts.ContractOperator,
		Op:              "gen_lipsync",
		Inputs: map[string]any{
			"image_path": portrait, "audio_path": audioPath, "fps": opts.Suite.FPS,
		},
		Params:      withModel(opts.Suite.Params, opts.Suite.Model),
		Workdir:     filepath.Join(workdir, "lipsync"),
		Determinism: operator.Determinism{Seed: &seed},
	})
	it.Timing.LipsyncMs = time.Since(lip0).Milliseconds()
	if err != nil {
		it.Status, it.Error = eval.ItemGenFail, err.Error()
		return it
	}
	if resp.Status != operator.StatusOK {
		it.Status, it.Error = eval.ItemGenFail, failMsg(resp)
		return it
	}
	lipPath, _ := resp.Outputs["video_path"].(string)
	lipHash, _ := resp.Outputs["content_hash"].(string)
	it.GpuSeconds += resp.Metrics.GpuSecond
	if resp.Metrics.PeakMemMB > it.PeakMemMB {
		it.PeakMemMB = resp.Metrics.PeakMemMB
	}
	for k, v := range resp.ModelVersions {
		if it.ModelVersions == nil {
			it.ModelVersions = map[string]string{}
		}
		it.ModelVersions["lipsync."+k] = v
	}
	if lipPath == "" || lipHash == "" {
		it.Status, it.Error = eval.ItemGenFail, "gen_lipsync OK 但缺 video_path/content_hash"
		return it
	}

	// 3) 转写三要素验证（transcribe 对口播语音独立转写；fake 透传 hint）
	trModel := opts.TranscribeModel
	if trModel == "" {
		trModel = "fake"
	}
	trResp, err := opts.Transcribe.Run(ctx, operator.Request{
		ContractVersion: contracts.ContractOperator,
		Op:              "transcribe",
		Inputs:          map[string]any{"audio_path": audioPath, "text_hint": entry.Prompt},
		Params:          map[string]any{"model": trModel},
		Workdir:         filepath.Join(workdir, "transcribe"),
		Determinism:     operator.Determinism{Seed: &seed},
	})
	if err != nil {
		it.Status, it.Error = eval.ItemGenFail, "转写执行失败: "+err.Error()
		return it
	}
	if trResp.Status != operator.StatusOK {
		it.Status, it.Error = eval.ItemGenFail, "转写失败: "+failMsg(trResp)
		return it
	}
	transcribed, _ := trResp.Outputs["text"].(string)
	it.TranscribedText = transcribed

	// 4) plan 构造（品牌名信息层逐字取自脚本——INV-5）
	plan, err := BuildPlan(planInput{
		Brand: s.Brand, AIGCText: m.AIGCDisclosure,
		GenHash: lipHash, VOHash: audioHash, Frames: frames,
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

	// 5) 渲染（真实解码 + 信息层 + AIGC 双轨 + 口播音轨保留）
	render0 := time.Now()
	req, err := compiler.Compile(plan,
		compiler.MediaIndex{"generated:lip-0001": {
			LocalPath: lipPath, ContentHash: lipHash, FPS: opts.Suite.FPS}},
		[]compiler.Font{opts.Font},
		compiler.Output{Path: outPath, Codec: "h264", CRF: 28, Preset: "veryfast"},
		compiler.Modes{Deterministic: true}, opts.RendererExpect)
	if err == nil {
		err = renderer.RenderVO(req, nil, audioPath)
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

	// 6) 断言：探针断言（qc 引擎，重试 ≤1）+ 转写三要素/信息层构造断言
	assert0 := time.Now()
	subj := &qc.Subject{
		MediaURI: outPath, MediaHash: od,
		Spec: map[string]any{"ref_media_path": lipPath},
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
	it.Assertions = append(rep.Results, transcribeResults(transcribed, s, audioPath)...)
	it.Assertions = append(it.Assertions, infoLayerResults(plan, s, m)...)
	it.Timing.AssertMs = time.Since(assert0).Milliseconds()
	it.Timing.TotalMs = time.Since(total0).Milliseconds()
	if rep.Pass() && constructPass(it.Assertions) {
		it.Status, it.Usable = eval.ItemOK, true
	} else {
		it.Status = eval.ItemAssertFail
	}
	return it
}

// constructPass 判定构造级断言（转写三要素 + 信息层）是否全过
// （探针断言已由 rep.Pass() 覆盖）。
func constructPass(results []qc.Result) bool {
	for _, r := range results {
		if !r.Pass && r.Skipped == "" &&
			(strings.HasPrefix(r.AssertionID, "L1.transcribe.") ||
				strings.HasPrefix(r.AssertionID, "L1.info_layer.")) {
			return false
		}
	}
	return true
}

// assertionPack 是形态4 成品的机器可判定断言包（evals/suites/
// form4_digital_human.json 的成品口径）：时长上限/画幅走 L0 探针；
// 口型同步走 L2 探针（SyncNet LSE-C/LSE-D）；AIGC 双轨走 L3 探针。
// aigc_overlay_present 的区域与 plan.go aigcBox 钉死同值；生成源路径经
// {{ref_media_path}} 从被检对象 Spec 注入（引擎把 Spec 展平为模板变量域）。
func assertionPack() []qc.Assertion {
	remedy := func(action, tpl string) qc.Remedy {
		return qc.Remedy{Action: action, InstructionTemplate: tpl}
	}
	return []qc.Assertion{
		{
			AssertionID: "L0.duration_upper_bound", Level: qc.L0, Severity: qc.SeverityBlocker,
			Probe:  qc.Probe{Op: "ffprobe_field", Args: map[string]any{"field": "duration_sec"}},
			Expect: qc.Expect{Op: "lte", Value: 6.0},
			Remedy: remedy("TRIM", "时长 {{measured}}s 超制式上限 6.0s，缩口播或加速"),
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
			AssertionID: "L2.lipsync_confidence", Level: qc.L2, Severity: qc.SeverityMajor,
			Probe:  qc.Probe{Op: "lipsync_lse_c", Args: map[string]any{}},
			Expect: qc.Expect{Op: "gte", Value: 6.0},
			Remedy: remedy("REGENERATE", "口型同步置信度 {{measured}} 低于阈值 6.0（SyncNet LSE-C），重抽或换后端"),
		},
		{
			AssertionID: "L2.lipsync_distance", Level: qc.L2, Severity: qc.SeverityMajor,
			Probe:  qc.Probe{Op: "lipsync_lse_d", Args: map[string]any{}},
			Expect: qc.Expect{Op: "lte", Value: 8.4},
			Remedy: remedy("REGENERATE", "口型同步距离 {{measured}} 超过阈值 8.4（SyncNet LSE-D），重抽或换后端"),
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

// transcribeResults 转写三要素断言（AC-7 判定题）：转写文本必须包含
// 品牌名/卖点/行动号召（期望值钉死在口播脚本，不从文案倒推）。
// 证据 = VO 产物路径（转写对象的内容寻址锚）。
func transcribeResults(text string, s *Script, audioPath string) []qc.Result {
	checks := []struct{ id, want string }{
		{"L1.transcribe.brand", s.Brand},
		{"L1.transcribe.selling_point", s.SellingPoint},
		{"L1.transcribe.cta", s.CTA},
	}
	out := make([]qc.Result, 0, len(checks))
	for _, c := range checks {
		out = append(out, qc.Result{
			AssertionID: c.id, Level: qc.L1, Severity: qc.SeverityBlocker,
			Pass: strings.Contains(text, c.want), Measured: text, Expected: c.want,
			EvidenceURI: audioPath,
		})
	}
	return out
}

// infoLayerResults 信息层构造断言（INV-5 构造级判定）：品牌名与 AIGC
// 披露的 overlay 文本必须与脚本/信息表逐字相等。证据 = 数据文件路径；
// 「文本真的进了画面」由 R-1 bitexact 与 aigc_overlay_present 探针背书。
func infoLayerResults(p videoplan.Plan, s *Script, m *form1.Merchant) []qc.Result {
	checks := []struct {
		id, want, oid, evidence string
	}{
		{"L1.info_layer.brand", s.Brand, "ov_brand", s.Path},
		{"L1.info_layer.aigc_disclosure", m.AIGCDisclosure, "ov_aigc", m.Path},
	}
	byID := map[string]string{}
	for _, o := range p.Overlays {
		if t, ok := o.Props["text"].(string); ok {
			byID[o.OverlayID] = t
		}
	}
	out := make([]qc.Result, 0, len(checks))
	for _, c := range checks {
		got := byID[c.oid]
		out = append(out, qc.Result{
			AssertionID: c.id, Level: qc.L1, Severity: qc.SeverityBlocker,
			Pass: got == c.want, Measured: got, Expected: c.want,
			EvidenceURI: c.evidence,
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
		return "", fmt.Errorf("form4: artifact 未计算 digest")
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

// failMsg 把算子四态失败压成单行信息。
func failMsg(resp operator.Response) string {
	msg := "算子故障"
	if resp.Error != nil {
		msg = resp.Error.Message
	}
	return fmt.Sprintf("%s: %s", resp.Status, msg)
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

// fileDigest 是文件的 sha256。
func fileDigest(path string) (string, error) {
	bs, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(bs)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}
