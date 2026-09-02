// shorts-report —— 出片率实测报告 + 产能外推入口（IR-0007 AC-5/AC-9/AC-10，
// 卡 #122 / 实验 E7）：
//
//	shorts-report -profile evals/runs/doctor-*.json \
//	  evals/runs/form1_fake_*.json evals/runs/form4_fake_*.json -out out/report
//
// 聚合形态1/形态4 run artifact：逐条判定明细、复算出片率（与 artifact
// 不一致即失败）、单条平均耗时与资源计量、日产能区间（显式标注估算，
// 不含电价变量，附 A100 对照迁移口径）——内容寻址落盘。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Cloudbird-Software/Shorts_Director/internal/report"
)

func main() {
	profilePath := flag.String("profile", "", "capability profile JSON（内容寻址引用来源，必填）")
	outDir := flag.String("out", "out/report", "报告落盘目录")
	date := flag.String("date", time.Now().UTC().Format("2006-01-02"), "确定性锚日期 YYYY-MM-DD")
	flag.Parse()
	paths := flag.Args()

	if *profilePath == "" || len(paths) == 0 {
		fmt.Fprintln(os.Stderr, "shorts-report: -profile 与至少一个 run artifact 路径必填")
		os.Exit(1)
	}
	rep, err := report.Build(report.Options{
		ArtifactPaths: paths, ProfilePath: *profilePath, Date: *date,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "shorts-report: %v\n", err)
		os.Exit(1)
	}
	path, err := rep.Save(*outDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "shorts-report: 落盘失败: %v\n", err)
		os.Exit(1)
	}
	summary, _ := json.Marshal(map[string]any{
		"artifact": path, "digest": rep.Digest,
		"suites":         rep.Totals.Suites,
		"entries_usable": rep.Totals.EntriesUsable,
		"entries_total":  rep.Totals.EntriesTotal,
		"items_usable":   rep.Totals.ItemsUsable,
		"items_total":    rep.Totals.ItemsTotal,
		"capacity_daily": []int{rep.Capacity.DailyLow, rep.Capacity.DailyHigh},
		"estimated":      rep.Capacity.Estimated,
		"date":           rep.Date,
	})
	fmt.Println(string(summary))
}
