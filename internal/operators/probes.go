// probes.go 实现 QC 断言包消费的纯 Go 探针算子（IR-0007 AC-6 形态1
// 断言包的执行端）。输出契约遵守 qc/bridge 约定：
//
//	outputs = { "value": <测量值>, "evidence_uri": "<证据，可选>" }
//
// 全部确定性：同媒体同参数同输出（黑帧检测阈值/像素差分阈值钉死，
// 不读时钟不取随机）。
package operators

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Cloudbird-Software/Shorts_Director/internal/compliance"
	"github.com/Cloudbird-Software/Shorts_Director/internal/operator"
)

// ── ffprobe_field ──────────────────────────────────────────────────────────

// FFProbeFieldOp 读媒体的单一 ffprobe 字段（L0 确定性断言的通用探针）。
// args.field ∈ parseFFProbe 输出键（duration_sec/width/height/fps/vcodec/…）。
type FFProbeFieldOp struct{ FFProbeBin string }

// Handle 执行 ffprobe 并取出单字段。
func (p *FFProbeFieldOp) Handle(ctx context.Context, req operator.Request) (operator.Response, error) {
	bin := p.FFProbeBin
	if bin == "" {
		bin = "ffprobe"
	}
	start := time.Now()
	path, field, err := mediaPathAndArg(req, "field")
	if err != nil {
		return inputErrorResponse(req.Op, "BAD_INPUTS", err.Error()), nil
	}
	raw, err := exec.CommandContext(ctx, bin, "-v", "error", "-print_format", "json",
		"-show_format", "-show_streams", path).Output()
	if err != nil {
		return inputErrorResponse(req.Op, "BAD_MEDIA",
			fmt.Sprintf("ffprobe 无法读取该文件: %v", err)), nil
	}
	meta, err := parseFFProbe(raw)
	if err != nil {
		return inputErrorResponse(req.Op, "BAD_MEDIA", err.Error()), nil
	}
	v, ok := meta[field]
	if !ok {
		return inputErrorResponse(req.Op, "BAD_INPUTS",
			fmt.Sprintf("field %q 不是 probe 输出键（可用: %v）", field, keys(meta))), nil
	}
	return okResponse(req.Op, map[string]any{"value": v}, time.Since(start), nil), nil
}

// ── resolution ─────────────────────────────────────────────────────────────

// ResolutionOp 读画幅宽/高（args.dim ∈ width|height）。
type ResolutionOp struct{ FFProbeBin string }

// Handle 执行 ffprobe 并取 width/height。
func (p *ResolutionOp) Handle(ctx context.Context, req operator.Request) (operator.Response, error) {
	bin := p.FFProbeBin
	if bin == "" {
		bin = "ffprobe"
	}
	start := time.Now()
	path, dim, err := mediaPathAndArg(req, "dim")
	if err != nil {
		return inputErrorResponse(req.Op, "BAD_INPUTS", err.Error()), nil
	}
	if dim != "width" && dim != "height" {
		return inputErrorResponse(req.Op, "BAD_INPUTS",
			fmt.Sprintf("dim 必须是 width|height，得到 %q", dim)), nil
	}
	raw, err := exec.CommandContext(ctx, bin, "-v", "error", "-print_format", "json",
		"-show_format", "-show_streams", path).Output()
	if err != nil {
		return inputErrorResponse(req.Op, "BAD_MEDIA",
			fmt.Sprintf("ffprobe 无法读取该文件: %v", err)), nil
	}
	meta, err := parseFFProbe(raw)
	if err != nil {
		return inputErrorResponse(req.Op, "BAD_MEDIA", err.Error()), nil
	}
	return okResponse(req.Op, map[string]any{"value": meta[dim]}, time.Since(start), nil), nil
}

// ── blackdetect_ratio ──────────────────────────────────────────────────────

// blackdetect 阈值钉死（确定性）：最短黑段 2 帧（25fps）、亮度阈值 10%。
const (
	blackMinDuration = 0.08
	blackPixTh       = 0.10
)

// BlackdetectRatioOp 测黑帧时长占比：ffmpeg blackdetect 检测器的时长加总
// ÷ 总时长。证据（检测器原始日志）落 workdir，evidence_uri 可回查。
type BlackdetectRatioOp struct{ FFmpegBin string }

// Handle 执行 blackdetect 并汇总占比。
func (p *BlackdetectRatioOp) Handle(ctx context.Context, req operator.Request) (operator.Response, error) {
	ffmpeg := p.FFmpegBin
	if ffmpeg == "" {
		ffmpeg = "ffmpeg"
	}
	start := time.Now()
	path := mediaPath(req)
	if path == "" {
		return inputErrorResponse(req.Op, "BAD_INPUTS", "inputs.media_path 必填（绝对路径）"), nil
	}
	if err := ctxErr(ctx); err != nil {
		return operator.Response{}, err
	}
	// 总时长（占比分母）
	probeBin := strings.TrimSuffix(ffmpeg, "ffmpeg") + "ffprobe"
	raw, err := exec.CommandContext(ctx, probeBin, "-v", "error",
		"-show_entries", "format=duration", "-of", "csv=p=0", path).Output()
	if err != nil {
		return inputErrorResponse(req.Op, "BAD_MEDIA",
			fmt.Sprintf("ffprobe 无法读取该文件: %v", err)), nil
	}
	total, err := strconv.ParseFloat(strings.TrimSpace(string(raw)), 64)
	if err != nil || total <= 0 {
		return inputErrorResponse(req.Op, "BAD_MEDIA", "总时长不可解析"), nil
	}
	// blackdetect 检测器
	cmd := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-i", path,
		"-vf", fmt.Sprintf("blackdetect=d=%g:pix_th=%g", blackMinDuration, blackPixTh),
		"-an", "-f", "null", "-")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return inputErrorResponse(req.Op, "BAD_MEDIA",
			fmt.Sprintf("blackdetect 执行失败: %v", err)), nil
	}
	black := parseBlackdetect(string(out))
	// 证据落盘（A2：非确定性产物显式落盘——这里是检测器原始日志）
	var evidence string
	if req.Workdir != "" {
		if err := os.MkdirAll(req.Workdir, 0o755); err == nil {
			ep := filepath.Join(req.Workdir, "blackdetect.log")
			if err := os.WriteFile(ep, out, 0o644); err == nil {
				evidence = ep
			}
		}
	}
	return okResponse(req.Op, map[string]any{
		"value": black / total, "evidence_uri": evidence,
	}, time.Since(start), map[string]string{"detector": "ffmpeg-blackdetect"}), nil
}

var blackLine = regexp.MustCompile(`black_start:(\S+) black_end:(\S+) black_duration:(\S+)`)

// parseBlackdetect 汇总黑段时长。
func parseBlackdetect(log string) float64 {
	sum := 0.0
	for _, m := range blackLine.FindAllStringSubmatch(log, -1) {
		if d, err := strconv.ParseFloat(m[3], 64); err == nil {
			sum += d
		}
	}
	return sum
}

// ── aigc_metadata_present ──────────────────────────────────────────────────

// AIGCMetadataOp 判定容器元数据是否携带完整 AIGC 隐式标识块
// （GB 45438 双轨的元数据轨；判定逻辑复用 internal/compliance）。
type AIGCMetadataOp struct{ FFProbeBin string }

// Handle 读 format tags 并判定。
func (p *AIGCMetadataOp) Handle(ctx context.Context, req operator.Request) (operator.Response, error) {
	bin := p.FFProbeBin
	if bin == "" {
		bin = "ffprobe"
	}
	start := time.Now()
	path := mediaPath(req)
	if path == "" {
		return inputErrorResponse(req.Op, "BAD_INPUTS", "inputs.media_path 必填（绝对路径）"), nil
	}
	raw, err := exec.CommandContext(ctx, bin, "-v", "error", "-show_format",
		"-print_format", "json", path).Output()
	if err != nil {
		return inputErrorResponse(req.Op, "BAD_MEDIA",
			fmt.Sprintf("ffprobe 无法读取该文件: %v", err)), nil
	}
	var probe struct {
		Format struct {
			Tags map[string]string `json:"tags"`
		} `json:"format"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return inputErrorResponse(req.Op, "BAD_MEDIA", "ffprobe 输出不可解析: "+err.Error()), nil
	}
	return okResponse(req.Op, map[string]any{
		"value": compliance.HasImplicitLabel(probe.Format.Tags),
	}, time.Since(start), nil), nil
}

// ── aigc_overlay_present ───────────────────────────────────────────────────

// AIGCOverlayOp 测「显式 AIGC 角标是否真的画进了画面」：
// 成品第 N 帧的指定区域 vs 生成源片段经同一归一化链（fps/scale/pad/format，
// 与 renderer.prepareMedia 一致）抽出第 N 帧——区域内容差异像素占比。
// 有文字（或任何叠加痕迹）→ 占比显著非零；纯重编码噪声在容差内不计数。
//
// 已知边界（诚实声明）：该探针证明「该区域被叠加过内容」，不证明内容
// 是 AIGC 文案本身——语义级判定留给 vlm_boolean（卡 #121）。缺
// ref_media_path 时 INPUT_ERROR（无参照即无法判定，宁失败不静默放行）。
type AIGCOverlayOp struct{ FFmpegBin string }

// Handle 抽帧比对并返回差异像素占比。
func (p *AIGCOverlayOp) Handle(ctx context.Context, req operator.Request) (operator.Response, error) {
	ffmpeg := p.FFmpegBin
	if ffmpeg == "" {
		ffmpeg = "ffmpeg"
	}
	start := time.Now()
	path := mediaPath(req)
	if path == "" {
		return inputErrorResponse(req.Op, "BAD_INPUTS", "inputs.media_path 必填（绝对路径）"), nil
	}
	ref, _ := req.Inputs["ref_media_path"].(string)
	if ref == "" {
		return inputErrorResponse(req.Op, "BAD_INPUTS",
			"inputs.ref_media_path 必填（生成源片段路径——无参照无法判定叠加存在性）"), nil
	}
	if _, err := os.Stat(ref); err != nil {
		return inputErrorResponse(req.Op, "BAD_INPUTS",
			"inputs.ref_media_path 不可读: "+err.Error()), nil
	}
	region, err := parseRegion(req.Inputs)
	if err != nil {
		return inputErrorResponse(req.Op, "BAD_INPUTS", err.Error()), nil
	}
	frame := intArg(req.Inputs, "frame", 12)
	tolerance := intArg(req.Inputs, "tolerance", 20)

	// 成品画幅 = 归一化目标（源片段按 renderer 同链归一到成品画幅）
	w, h, err := videoDims(ffmpeg, path)
	if err != nil {
		return inputErrorResponse(req.Op, "BAD_MEDIA", "读成品画幅失败: "+err.Error()), nil
	}
	wd := req.Workdir
	if wd == "" {
		wd = os.TempDir()
	}
	if err := os.MkdirAll(wd, 0o755); err != nil {
		return operator.Response{}, err
	}
	finalDir, refDir := filepath.Join(wd, "ov_final"), filepath.Join(wd, "ov_ref")

	// 成品第 N 帧：format=rgba 解码（无重采样）
	if err := extractNth(ffmpeg, path, "", w, h, frame, finalDir); err != nil {
		return inputErrorResponse(req.Op, "BAD_MEDIA", "抽成品帧失败: "+err.Error()), nil
	}
	// 源第 N 帧：renderer.prepareMedia 同一归一化链（fps/scale/pad/format）
	norm := fmt.Sprintf("fps=25,scale=%d:%d:force_original_aspect_ratio=decrease,"+
		"pad=%d:%d:(ow-iw)/2:(oh-ih)/2:black,format=rgba", w, h, w, h)
	if err := extractNth(ffmpeg, ref, norm, w, h, frame, refDir); err != nil {
		return inputErrorResponse(req.Op, "BAD_INPUTS", "抽源帧失败: "+err.Error()), nil
	}
	fin, err := readPNG(filepath.Join(finalDir, fmt.Sprintf("f_%06d.png", frame)))
	if err != nil {
		return inputErrorResponse(req.Op, "BAD_MEDIA", "读成品帧失败: "+err.Error()), nil
	}
	rf, err := readPNG(filepath.Join(refDir, fmt.Sprintf("f_%06d.png", frame)))
	if err != nil {
		return inputErrorResponse(req.Op, "BAD_INPUTS", "读源帧失败: "+err.Error()), nil
	}
	ratio, err := regionDiff(fin, rf, region, tolerance)
	if err != nil {
		return inputErrorResponse(req.Op, "BAD_INPUTS", err.Error()), nil
	}
	// 证据：两张被比对的帧留在 workdir 可回查
	return okResponse(req.Op, map[string]any{
		"value":       ratio,
		"evidence_uri": filepath.Join(finalDir, fmt.Sprintf("f_%06d.png", frame)),
	}, time.Since(start), map[string]string{"method": "pixel-diff-vs-ref"}), nil
}

// extractNth 抽第 frame 帧到 dir/f_%06d.png（先落 0..frame 全序列再取）。
func extractNth(ffmpeg, path, vf string, w, h, frame int, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	args := []string{"-y", "-hide_banner", "-loglevel", "error", "-i", path}
	if vf != "" {
		args = append(args, "-vf", vf)
	}
	args = append(args, "-frames:v", strconv.Itoa(frame+1),
		"-start_number", "0", filepath.Join(dir, "f_%06d.png"))
	if out, err := exec.Command(ffmpeg, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("%v: %s", err, out)
	}
	return nil
}

func readPNG(path string) (image.Image, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	return png.Decode(fh)
}

// region 是比对区域（像素，成品坐标系）。
type region struct{ X, Y, W, H int }

// parseRegion 解析 args 的 x/y/w/h。
func parseRegion(inputs map[string]any) (region, error) {
	r := region{
		X: intArg(inputs, "x", 0), Y: intArg(inputs, "y", 0),
		W: intArg(inputs, "w", 0), H: intArg(inputs, "h", 0),
	}
	if r.W <= 0 || r.H <= 0 {
		return r, fmt.Errorf("args w/h 必须为正（比对区域）")
	}
	return r, nil
}

// regionDiff 比对两图区域：max 通道差 > tolerance 的像素占比。
func regionDiff(a, b image.Image, r region, tolerance int) (float64, error) {
	ab, bb := a.Bounds(), b.Bounds()
	if ab.Dx() != bb.Dx() || ab.Dy() != bb.Dy() {
		return 0, fmt.Errorf("两帧画幅不一致 %dv%d vs %dv%d", ab.Dx(), ab.Dy(), bb.Dx(), bb.Dy())
	}
	if r.X < 0 || r.Y < 0 || r.X+r.W > ab.Dx() || r.Y+r.H > ab.Dy() {
		return 0, fmt.Errorf("区域 (%d,%d,%d,%d) 越出画幅 %dx%d", r.X, r.Y, r.W, r.H, ab.Dx(), ab.Dy())
	}
	ai, bi := a.(image.RGBA64Image), b.(image.RGBA64Image)
	if ai == nil || bi == nil {
		return 0, fmt.Errorf("帧非 RGBA 可解码")
	}
	changed, total := 0, r.W*r.H
	for y := r.Y; y < r.Y+r.H; y++ {
		for x := r.X; x < r.X+r.W; x++ {
			pa, pb := ai.RGBA64At(x, y), bi.RGBA64At(x, y)
			dr := abs64(int(pa.R) - int(pb.R))
			dg := abs64(int(pa.G) - int(pb.G))
			db := abs64(int(pa.B) - int(pb.B))
			if max3(dr, dg, db) > tolerance {
				changed++
			}
		}
	}
	return float64(changed) / float64(total), nil
}

func abs64(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func max3(a, b, c int) int {
	m := a
	if b > m {
		m = b
	}
	if c > m {
		m = c
	}
	return m
}

// videoDims 读视频画幅（ffprobe）。
func videoDims(ffmpeg, path string) (int, int, error) {
	probeBin := strings.TrimSuffix(ffmpeg, "ffmpeg") + "ffprobe"
	raw, err := exec.Command(probeBin, "-v", "error", "-select_streams", "v:0",
		"-show_entries", "stream=width,height", "-of", "csv=p=0", path).Output()
	if err != nil {
		return 0, 0, err
	}
	parts := strings.Split(strings.TrimSpace(string(raw)), ",")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("画幅不可解析: %q", string(raw))
	}
	w, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	h, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil {
		return 0, 0, fmt.Errorf("画幅不可解析: %q", string(raw))
	}
	return w, h, nil
}

// ── 公共小工具 ─────────────────────────────────────────────────────────────

// mediaPath 取 inputs.media_path（绝对路径校验）。
func mediaPath(req operator.Request) string {
	p, _ := req.Inputs["media_path"].(string)
	if p != "" && strings.HasPrefix(p, "/") {
		return p
	}
	return ""
}

// mediaPathAndArg 取 media_path + 一个字符串参数（缺一报错）。
func mediaPathAndArg(req operator.Request, argKey string) (string, string, error) {
	path := mediaPath(req)
	if path == "" {
		return "", "", fmt.Errorf("inputs.media_path 必填（绝对路径）")
	}
	arg, _ := req.Inputs[argKey].(string)
	if arg == "" {
		return "", "", fmt.Errorf("inputs.%s 必填", argKey)
	}
	return path, arg, nil
}

// intArg 取整型参数（JSON 数值默认 float64；带默认值）。
func intArg(inputs map[string]any, key string, def int) int {
	switch v := inputs[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case json.Number:
		if n, err := v.Int64(); err == nil {
			return int(n)
		}
	}
	return def
}

// keys 取 map 键排序清单（报错指引用）。
func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func ctxErr(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("operator: 被取消: %w", err)
	}
	return nil
}

// HandlerRunner 把 Handler 适配成 operator.Runner（进程内执行，无子进程）：
// 单元测试与 CLI 共用同一 Handler 实现，避免测试替身与生产路径漂移。
type HandlerRunner struct {
	Op string
	H  Handler
}

// Run 进程内执行 Handler（先过请求校验，与 Serve 同前置）。
func (r HandlerRunner) Run(ctx context.Context, req operator.Request) (operator.Response, error) {
	if err := req.Validate(); err != nil {
		return operator.Response{}, err
	}
	resp, err := r.H(ctx, req)
	if err != nil {
		return operator.Response{}, err
	}
	if resp.Op == "" {
		resp.Op = r.Op
	}
	return resp, nil
}
