// Command shorts-render 是 Phase 0 最小渲染入口（#43 DoD）：
//
//	make render-demo
//	  ≡ go run ./cmd/shorts-render
//	      -plan schema/testdata/video_plan/valid/minimal_music_plan.json \
//	      -out out/demo.mp4
//
// 媒体索引与字体为内置占位（minimal plan 的 DoD 语义就是素材可为假的）；
// 真实控制面的索引装配是 Phase 1 S6 的职责。输出 JSON 摘要到 stdout
// （C3 response 的最小形态：R-1/R-2/R-5 的可判定证据）。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Cloudbird-Software/Shorts_Director/internal/compiler"
	"github.com/Cloudbird-Software/Shorts_Director/internal/renderer"
	"github.com/Cloudbird-Software/Shorts_Director/internal/videoplan"
)

// placeholderEntry 生成占位媒体文件并返回索引条目（hash 与 plan 钉死值一致）。
func placeholderEntry(dir, name, hash string) compiler.MediaEntry {
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("placeholder:"+name), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "shorts-render: 写占位素材失败: %v\n", err)
		os.Exit(1)
	}
	return compiler.MediaEntry{LocalPath: p, ContentHash: hash, FPS: 25}
}

func main() {
	planPath := flag.String("plan", "schema/testdata/video_plan/valid/minimal_music_plan.json", "手写 plan.json 路径")
	outPath := flag.String("out", "out/demo.mp4", "输出视频路径")
	flag.Parse()

	raw, err := os.ReadFile(*planPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "shorts-render: 读 plan 失败: %v\n", err)
		os.Exit(1)
	}
	var p videoplan.Plan
	if err := json.Unmarshal(raw, &p); err != nil {
		fmt.Fprintf(os.Stderr, "shorts-render: 解析 plan 失败: %v\n", err)
		os.Exit(1)
	}

	// 占位索引：按 plan 实际引用的 SHOT/GENERATED clip 生成，
	// content_hash 直接采用 plan 钉死值（版本校验恒通过——占位的意义）。
	tmp, err := os.MkdirTemp("", "shorts-render-media-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "shorts-render: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmp)
	idx := compiler.MediaIndex{}
	phSeq := 0
	for _, t := range p.Tracks {
		if t.Kind != videoplan.TrackVideoMain && t.Kind != videoplan.TrackVideoInsert {
			continue
		}
		for _, c := range t.Clips {
			if c.Source.Kind == "SHOT" || c.Source.Kind == "GENERATED" {
				ref := "shot:" + c.Source.Ref
				if c.Source.Kind == "GENERATED" {
					ref = "generated:" + c.Source.Ref
				}
				phSeq++
				idx[ref] = placeholderEntry(tmp,
					fmt.Sprintf("ph-%d-%s.placeholder", phSeq, c.Source.Ref),
					c.Source.ContentHash)
			}
		}
	}

	resp, err := renderer.RenderPlanTo(p, idx,
		[]compiler.Font{{Family: "HarmonyOS_Sans_Bold", Path: tmp + "/font.placeholder", Hash: "sha256:aaaa"}},
		compiler.Output{Path: *outPath, Codec: "h264", CRF: 28, Preset: "veryfast"},
		compiler.Modes{Deterministic: true},
		compiler.RendererExpect{FFmpeg: "7.1", Remotion: "4.0.230", Node: "22.11.0"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "shorts-render: 渲染失败: %v\n", err)
		os.Exit(1)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(resp)
}
