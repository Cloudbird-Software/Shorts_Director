// shorts-form1 —— 形态1 端到端管线入口（IR-0007 AC-6 / BEH-5，E5 切片）：
//
//	shorts-form1 -suite evals/suites/form1_ambience.json \
//	  -merchants evals/merchants -runner local -bin operators/gen_i2v/run.sh \
//	  -font-path /usr/share/fonts/.../NotoSansCJK-Regular.ttf \
//	  -font-family Noto_Sans_CJK -probe-bin bin/shorts-operator -out out/form1
//
// 流程：mock 商家（种子图+信息表）→ gen_i2v → 信息层叠加 → AIGC 双轨 →
// mp4 → 形态1 断言包 → run artifact（全链耗时分解 + 出片率）。
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
	"github.com/Cloudbird-Software/Shorts_Director/internal/operator"
	"github.com/Cloudbird-Software/Shorts_Director/internal/qc"
)

func main() {
	suitePath := flag.String("suite", "evals/suites/form1_ambience.json", "形态1 套件（条目=商家×prompt×时长）")
	merchantsDir := flag.String("merchants", "evals/merchants", "mock 商家数据集目录")
	outDir := flag.String("out", "out/form1", "run artifact 落盘目录")
	runnerMode := flag.String("runner", "local", "生成算子执行器：fake|local")
	bin := flag.String("bin", "operators/gen_i2v/run.sh", "local 模式生成算子入口")
	model := flag.String("model", "", "覆盖套件 model（如 fake——冒烟联调）")
	probeBin := flag.String("probe-bin", "bin/shorts-operator", "探针算子入口（先 go build -o bin/shorts-operator ./cmd/shorts-operator）")
	fontPath := flag.String("font-path", "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"信息层字体文件（中文信息须 CJK 字体；哈希核验 R-4）")
	fontFamily := flag.String("font-family", "DejaVu_Sans", "字体族名（与字体文件对应）")
	workdir := flag.String("workdir", "", "逐条目工作目录根（默认 <out>/work）")
	profilePath := flag.String("profile", "", "capability profile JSON（内容寻址引用来源）")
	date := flag.String("date", time.Now().UTC().Format("2006-01-02"), "确定性锚日期 YYYY-MM-DD")
	flag.Parse()

	suite, err := eval.LoadSuite(*suitePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "shorts-form1: %v\n", err)
		os.Exit(1)
	}
	if *model != "" {
		suite.Model = *model
	}
	merchants, err := form1.LoadMerchantsDir(*merchantsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "shorts-form1: %v\n", err)
		os.Exit(1)
	}
	font, err := loadFont(*fontPath, *fontFamily)
	if err != nil {
		fmt.Fprintf(os.Stderr, "shorts-form1: %v\n", err)
		os.Exit(1)
	}

	var gen operator.Runner
	switch *runnerMode {
	case "fake":
		gen = &operator.LocalRunner{Bin: *bin} // 套件/参数须指向 fake 后端（-model fake）
	case "local":
		gen = &operator.LocalRunner{Bin: *bin}
	default:
		fmt.Fprintf(os.Stderr, "shorts-form1: 未知 runner %q（fake|local）\n", *runnerMode)
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
			fmt.Fprintf(os.Stderr, "shorts-form1: %v\n", err)
			os.Exit(1)
		}
	}
	art, err := form1.Run(context.Background(), form1.Options{
		Suite: suite, Merchants: merchants, Gen: gen,
		Engine: mustEngine(*probeBin),
		Font:   font, RunnerMode: *runnerMode, ProfileRef: profileRef,
		WorkdirRoot: root, Date: *date,
		RendererExpect: compiler.RendererExpect{
			FFmpeg: ffmpegVersion(), Remotion: "4.0.230", Node: "22.11.0"},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "shorts-form1: %v\n", err)
		os.Exit(1)
	}
	path, err := art.Save(*outDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "shorts-form1: 落盘失败: %v\n", err)
		os.Exit(1)
	}
	summary, _ := json.Marshal(map[string]any{
		"artifact": path, "digest": art.Digest,
		"yield_ratio":   art.Yield.YieldRatio,
		"entries":       []int{art.Yield.EntriesWithUsable, art.Yield.EntriesTotal},
		"items_usable":  art.Yield.ItemsUsable, "items_total": art.Yield.ItemsTotal,
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

// mustEngine 注册形态1 断言包所需探针（经 RunnerProbeAdapter 走 C2 契约）。
func mustEngine(probeBin string) *qc.Engine {
	tiers := map[string]qc.CostTier{
		"ffprobe_field":         qc.CostFree,
		"resolution":            qc.CostFree,
		"aigc_metadata_present": qc.CostFree,
		"blackdetect_ratio":     qc.CostLight,
		"aigc_overlay_present":  qc.CostLight,
	}
	probes := qc.Operators(&operator.LocalRunner{Bin: probeBin},
		filepath.Join(os.TempDir(), "shorts-form1-probes"), tiers)
	e, err := qc.NewEngine(probes...)
	if err != nil {
		panic(err)
	}
	return e
}
