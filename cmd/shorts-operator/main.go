// shorts-operator 是 C2 协议的 Go 算子 CLI：
//
//	shorts-operator <op> --contract-version 1 < request.json > response.json
//
// 纯 Go 可实现的算子在此注册；Python 重模型算子独立成镜像，
// 与本入口共用同一契约（schema/contracts/operator/*.json）。
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Cloudbird-Software/Shorts_Director/internal/operators"
)

func main() {
	if len(os.Args) < 4 || os.Args[2] != "--contract-version" || os.Args[3] != "1" {
		fmt.Fprintln(os.Stderr, "用法: shorts-operator <op> --contract-version 1 < request.json")
		os.Exit(2)
	}
	op := os.Args[1]

	registry := map[string]operators.Handler{
		"probe": (&operators.ProbeOp{}).Handle,
		// QC 断言包探针（IR-0007 AC-6 形态1 断言包执行端）
		"ffprobe_field":         (&operators.FFProbeFieldOp{}).Handle,
		"resolution":            (&operators.ResolutionOp{}).Handle,
		"blackdetect_ratio":     (&operators.BlackdetectRatioOp{}).Handle,
		"aigc_metadata_present": (&operators.AIGCMetadataOp{}).Handle,
		"aigc_overlay_present":  (&operators.AIGCOverlayOp{}).Handle,
	}
	h, ok := registry[op]
	if !ok {
		fmt.Fprintf(os.Stderr, "shorts-operator: 未注册算子 %q\n", op)
		os.Exit(2)
	}
	if err := operators.Serve(context.Background(), os.Stdin, os.Stdout, h); err != nil {
		fmt.Fprintln(os.Stderr, "shorts-operator:", err)
		os.Exit(1)
	}
}
