// shorts-calibrate —— 裁判校准入口（IR-0007 AC-8 / BEH-7，E6）：
//
//	shorts-calibrate -labels evals/human_labels/labels.json \
//	  -bin operators/vlm_boolean/run.sh -model fake -out out/calibrate
//
// 流程：vlm_boolean 评审探针逐条判定标注集媒体 → 与人工标注比对 →
// 混淆矩阵 + 一致率 → 内容寻址报告落盘（结论登记进 README 假设看板 E6）。
// fake 后端是无语义负对照（一致率应显著低于真实 VLM——仪器 sanity check）；
// 真实校准：-model qwen-vl -seed 7（V100）。
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Cloudbird-Software/Shorts_Director/internal/calibrate"
	"github.com/Cloudbird-Software/Shorts_Director/internal/operator"
)

func main() {
	labelsPath := flag.String("labels", "evals/human_labels/labels.json", "人工标注集（含媒体相对路径）")
	bin := flag.String("bin", "operators/vlm_boolean/run.sh", "vlm_boolean 算子入口")
	model := flag.String("model", "fake", "评审后端：fake（负对照）|qwen-vl（真实校准）")
	seedFlag := flag.Int64("seed", 0, "determinism.seed（真实后端必填，AC-3 重放条款）")
	outDir := flag.String("out", "out/calibrate", "校准报告落盘目录")
	workdir := flag.String("workdir", "", "逐条目工作目录根（默认 <out>/work）")
	flag.Parse()

	labels, err := calibrate.LoadLabels(*labelsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "shorts-calibrate: %v\n", err)
		os.Exit(1)
	}
	root := *workdir
	if root == "" {
		root = filepath.Join(*outDir, "work")
	}
	// 算子要求绝对路径（inputs.media_path 硬校验）——CLI 边界统一转绝对。
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	var seed *int64
	if *model != "fake" {
		seed = new(int64)
		*seed = *seedFlag
	}
	rep, err := calibrate.Run(context.Background(), calibrate.Options{
		Labels: labels, LabelsPath: *labelsPath,
		Runner: &operator.LocalRunner{Bin: *bin}, Model: *model,
		WorkdirRoot: root, Seed: seed,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "shorts-calibrate: %v\n", err)
		os.Exit(1)
	}
	path, err := rep.Save(*outDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "shorts-calibrate: 落盘失败: %v\n", err)
		os.Exit(1)
	}
	summary, _ := json.Marshal(map[string]any{
		"artifact": path, "digest": rep.Digest,
		"matrix":    rep.Matrix,
		"agreement": rep.Agreement,
		"judged":    rep.Matrix.Total(),
		"errors":    rep.Errors,
		"labels":    len(rep.Items),
		"date":      time.Now().UTC().Format("2006-01-02"),
	})
	fmt.Println(string(summary))
}
