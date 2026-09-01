// shorts-form4 —— 形态4（数字人口播）端到端管线入口（IR-0007 AC-7 / BEH-6）：
//
//	shorts-form4 -suite evals/suites/form4_digital_human.json \
//	  -merchants evals/merchants -runner local -model fake \
//	  -tts-bin operators/gen_tts/run.sh -lipsync-bin operators/gen_lipsync/run.sh \
//	  -transcribe-bin operators/transcribe/run.sh -transcribe-model fake \
//	  -font-path /usr/share/fonts/.../NotoSansCJK-Regular.ttf \
//	  -font-family Noto_Sans_CJK -probe-bin bin/shorts-operator \
//	  -syncnet-bin operators/syncnet_metric/run.sh -out out/form4
//
// 流程：mock 商家（人像照+口播文案+三要素脚本）→ gen_tts → gen_lipsync →
// transcribe 三要素 → 信息层+AIGC 双轨+口播音轨保留渲染 → 形态4 断言包 →
// run artifact（全链耗时分解 tts/lipsync/render/assert + 出片率）。
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Cloudbird-Software/Shorts_Director/internal/compiler"
	"github.com/Cloudbird-Software/Shorts_Director/internal/eval"
	"github.com/Cloudbird-Software/Shorts_Director/internal/form1"
	"github.com/Cloudbird-Software/Shorts_Director/internal/form4"
	"github.com/Cloudbird-Software/Shorts_Director/internal/operator"
	"github.com/Cloudbird-Software/Shorts_Director/internal/qc"
)

// probeRouter 把断言探针按 op 路由到 Go 探针二进制或 Python 指标算子
// （lipsync_lse_c/d 是 SyncNet 探针，独立于 shorts-operator）。
type probeRouter map[string]operator.Runner

func (r probeRouter) Run(ctx context.Context, req operator.Request) (operator.Response, error) {
	runner, ok := r[req.Op]
	if !ok {
		return operator.Response{}, fmt.Errorf("shorts-form4: 未注册探针 %q", req.Op)
	}
	return runner.Run(ctx, req)
}

func main() {
	suitePath := flag.String("suite", "evals/suites/form4_digital_human.json", "形态4 套件（条目=商家×口播文案）")
	merchantsDir := flag.String("merchants", "evals/merchants", "mock 商家数据集目录（含 form4.json 三要素脚本）")
	outDir := flag.String("out", "out/form4", "run artifact 落盘目录")
	runnerMode := flag.String("runner", "local", "生成算子执行器：fake|local")
	model := flag.String("model", "", "覆盖套件 model（如 fake——冒烟联调；fake 时 TTS/lipsync 均走假后端）")
	ttsBin := flag.String("tts-bin", "operators/gen_tts/run.sh", "gen_tts 算子入口")
	lipsyncBin := flag.String("lipsync-bin", "operators/gen_lipsync/run.sh", "gen_lipsync 算子入口")
	transcribeBin := flag.String("transcribe-bin", "operators/transcribe/run.sh", "transcribe 算子入口")
	transcribeModel := flag.String("transcribe-model", "fake", "转写后端：fake（透传 hint，联调）|whisper（独立转写，V100 实测）")
	probeBin := flag.String("probe-bin", "bin/shorts-operator", "Go 探针算子入口（先 go build -o bin/shorts-operator ./cmd/shorts-operator）")
	syncnetBin := flag.String("syncnet-bin", "operators/syncnet_metric/run.sh", "SyncNet 口型指标探针入口")
	fontPath := flag.String("font-path", "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"信息层字体文件（中文口播须 CJK 字体；哈希核验 R-4）")
	fontFamily := flag.String("font-family", "DejaVu_Sans", "字体族名（与字体文件对应）")
	workdir := flag.String("workdir", "", "逐条目工作目录根（默认 <out>/work）")
	profilePath := flag.String("profile", "", "capability profile JSON（内容寻址引用来源）")
	date := flag.String("date", time.Now().UTC().Format("2006-01-02"), "确定性锚日期 YYYY-MM-DD")
	seedsCSV := flag.String("seeds", "", "覆盖套件 seed 集（逗号分隔；默认取套件首个 seed——AC-7 每商家 ≥1 条）")
	flag.Parse()

	suite, err := eval.LoadSuite(*suitePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "shorts-form4: %v\n", err)
		os.Exit(1)
	}
	if *model != "" {
		suite.Model = *model
	}
	merchants, err := form1.LoadMerchantsDir(*merchantsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "shorts-form4: %v\n", err)
		os.Exit(1)
	}
	scripts, err := form4.LoadScriptsDir(*merchantsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "shorts-form4: %v\n", err)
		os.Exit(1)
	}
	font, err := loadFont(*fontPath, *fontFamily)
	if err != nil {
		fmt.Fprintf(os.Stderr, "shorts-form4: %v\n", err)
		os.Exit(1)
	}

	switch *runnerMode {
	case "fake", "local":
	default:
		fmt.Fprintf(os.Stderr, "shorts-form4: 未知 runner %q（fake|local）\n", *runnerMode)
		os.Exit(1)
	}

	root := *workdir
	if root == "" {
		root = filepath.Join(*outDir, "work")
	}
	// 探针算子要求绝对路径（inputs.media_path 硬校验）——CLI 边界统一转绝对。
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	profileRef := ""
	if *profilePath != "" {
		if profileRef, err = profileDigest(*profilePath); err != nil {
			fmt.Fprintf(os.Stderr, "shorts-form4: %v\n", err)
			os.Exit(1)
		}
	}
	var seeds []int64
	if *seedsCSV != "" {
		for _, s := range strings.Split(*seedsCSV, ",") {
			var v int64
			if _, err := fmt.Sscanf(strings.TrimSpace(s), "%d", &v); err != nil {
				fmt.Fprintf(os.Stderr, "shorts-form4: seed %q 不是整数\n", s)
				os.Exit(1)
			}
			seeds = append(seeds, v)
		}
	}
	art, err := form4.Run(context.Background(), form4.Options{
		Suite: suite, Merchants: merchants, Scripts: scripts,
		TTS:             &operator.LocalRunner{Bin: *ttsBin},
		Lipsync:         &operator.LocalRunner{Bin: *lipsyncBin},
		Transcribe:      &operator.LocalRunner{Bin: *transcribeBin},
		TranscribeModel: *transcribeModel,
		Engine:          mustEngine(*probeBin, *syncnetBin, root),
		Font:            font, RunnerMode: *runnerMode, ProfileRef: profileRef,
		WorkdirRoot: root, Root: ".", Date: *date, Seeds: seeds,
		RendererExpect: compiler.RendererExpect{
			FFmpeg: ffmpegVersion(), Remotion: "4.0.230", Node: "22.11.0"},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "shorts-form4: %v\n", err)
		os.Exit(1)
	}
	path, err := art.Save(*outDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "shorts-form4: 落盘失败: %v\n", err)
		os.Exit(1)
	}
	summary, _ := json.Marshal(map[string]any{
		"artifact": path, "digest": art.Digest,
		"yield_ratio":  art.Yield.YieldRatio,
		"entries":      []int{art.Yield.EntriesWithUsable, art.Yield.EntriesTotal},
		"items_usable": art.Yield.ItemsUsable, "items_total": art.Yield.ItemsTotal,
	})
	fmt.Println(string(summary))
}

// loadFont 计算字体文件 sha256（R-4：渲染时按哈希核验，无隐式换字体）。
func loadFont(path, family string) (compiler.Font, error) {
	bs, err := os.ReadFile(path)
	if err != nil {
		return compiler.Font{}, fmt.Errorf("读字体失败: %w", err)
	}
	sum := sha256.Sum256(bs)
	return compiler.Font{
		Family: family, Path: path,
		Hash: "sha256:" + hex.EncodeToString(sum[:]),
	}, nil
}

// ffmpegVersion 取本机 ffmpeg 版本指纹（R-5 版本钉死）。
func ffmpegVersion() string {
	out, err := exec.Command("ffmpeg", "-version").Output()
	if err != nil {
		return "unknown"
	}
	fields := strings.Fields(strings.SplitN(string(out), "\n", 2)[0])
	if len(fields) >= 3 {
		return fields[2]
	}
	return "unknown"
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

// mustEngine 注册形态4 断言包所需探针：Go 探针走 shorts-operator，
// SyncNet 口型指标走 Python 算子（探针统一经 C2 契约）。
func mustEngine(probeBin, syncnetBin, workdir string) *qc.Engine {
	router := probeRouter{
		"ffprobe_field":         &operator.LocalRunner{Bin: probeBin},
		"resolution":            &operator.LocalRunner{Bin: probeBin},
		"aigc_metadata_present": &operator.LocalRunner{Bin: probeBin},
		"aigc_overlay_present":  &operator.LocalRunner{Bin: probeBin},
		"lipsync_lse_c":         &operator.LocalRunner{Bin: syncnetBin},
		"lipsync_lse_d":         &operator.LocalRunner{Bin: syncnetBin},
	}
	tiers := map[string]qc.CostTier{
		"ffprobe_field":         qc.CostFree,
		"resolution":            qc.CostFree,
		"aigc_metadata_present": qc.CostFree,
		"aigc_overlay_present":  qc.CostLight,
		"lipsync_lse_c":         qc.CostHeavy,
		"lipsync_lse_d":         qc.CostHeavy,
	}
	e, err := qc.NewEngine(qc.Operators(router,
		filepath.Join(workdir, "probes"), tiers)...)
	if err != nil {
		panic(err)
	}
	return e
}
