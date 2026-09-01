// shorts-eval —— 制式评估套件执行入口（IR-0007 AC-4/AC-5，E2 仪器 CLI）。
//
//	shorts-eval -suite evals/…/suite.json -profile out/doctor/<digest>.json \
//	            -runner local -bin operators/gen_i2v/run.sh -out out/eval
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Cloudbird-Software/Shorts_Director/internal/eval"
	"github.com/Cloudbird-Software/Shorts_Director/internal/operator"
	"github.com/Cloudbird-Software/Shorts_Director/internal/qc"
)

func main() {
	suitePath := flag.String("suite", "", "套件定义 JSON（必填）")
	profilePath := flag.String("profile", "", "capability profile JSON（内容寻址引用来源，必填）")
	outDir := flag.String("out", "out/eval", "run artifact 落盘目录")
	runnerMode := flag.String("runner", "fake", "生成算子执行器：fake|local")
	bin := flag.String("bin", "operators/gen_i2v/run.sh", "local 模式算子入口")
	probeBin := flag.String("probe-bin", "bin/shorts-operator",
		"探针算子入口（先 go build -o bin/shorts-operator ./cmd/shorts-operator）")
	workdir := flag.String("workdir", "", "逐条目工作目录根（默认 <out>/work）")
	flag.Parse()

	if *suitePath == "" || *profilePath == "" {
		fmt.Fprintln(os.Stderr, "shorts-eval: -suite 与 -profile 必填")
		os.Exit(1)
	}
	suite, err := eval.LoadSuite(*suitePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "shorts-eval: %v\n", err)
		os.Exit(1)
	}
	profileRef, err := profileDigest(*profilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "shorts-eval: %v\n", err)
		os.Exit(1)
	}

	var gen operator.Runner
	switch *runnerMode {
	case "fake":
		gen = &operator.FakeRunner{Dir: "testdata/golden"}
	case "local":
		gen = &operator.LocalRunner{Bin: *bin}
	default:
		fmt.Fprintf(os.Stderr, "shorts-eval: 未知 runner %q（fake|local）\n", *runnerMode)
		os.Exit(1)
	}

	root := *workdir
	if root == "" {
		root = filepath.Join(*outDir, "work")
	}
	art, err := eval.Run(context.Background(), eval.RunOptions{
		Suite: suite, Gen: gen, Engine: mustEngine(suite, *probeBin),
		ProfileRef: profileRef, RunnerMode: *runnerMode,
		WorkdirRoot: root, Now: time.Now,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "shorts-eval: %v\n", err)
		os.Exit(1)
	}
	path, err := art.Save(*outDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "shorts-eval: 落盘失败: %v\n", err)
		os.Exit(1)
	}
	summary, _ := json.Marshal(map[string]any{
		"artifact": path, "digest": art.Digest,
		"yield_ratio":  art.Yield.YieldRatio,
		"entries":      []int{art.Yield.EntriesWithUsable, art.Yield.EntriesTotal},
		"items_usable": art.Yield.ItemsUsable, "items_total": art.Yield.ItemsTotal,
		"budget_truncated": art.BudgetTruncated,
	})
	fmt.Println(string(summary))
}

// profileDigest 提取 capability profile 的内容寻址 digest 作为引用。
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

// mustEngine 按套件断言包用到的 probe op 注册探针（经 RunnerProbeAdapter
// 走 C2 契约；成本档位手工标注默认 Light，重量模型类 op 标 Heavy）。
func mustEngine(s *eval.Suite, probeBin string) *qc.Engine {
	heavy := map[string]bool{"vlm_boolean": true}
	ops := map[string]bool{}
	for _, a := range s.AssertionPack {
		ops[a.Probe.Op] = true
	}
	specs := map[string]qc.CostTier{}
	for op := range ops {
		specs[op] = qc.CostLight
		if heavy[op] {
			specs[op] = qc.CostHeavy
		}
	}
	probes := qc.Operators(&operator.LocalRunner{Bin: probeBin},
		filepath.Join(os.TempDir(), "shorts-eval-probes"), specs)
	e, err := qc.NewEngine(probes...)
	if err != nil {
		panic(err)
	}
	return e
}
