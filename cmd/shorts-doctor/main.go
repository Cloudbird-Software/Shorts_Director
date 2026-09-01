// shorts-doctor —— 环境探测入口（IR-0007 AC-1，make doctor）：
// 探测 GPU/ffmpeg/docker 与候选模型可行性，产出内容寻址 capability
// profile 落盘，stdout 输出 {artifact, digest} 摘要（机器可读）。
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Cloudbird-Software/Shorts_Director/internal/doctor"
)

func main() {
	outDir := flag.String("out", "out/doctor", "capability profile 落盘目录")
	flag.Parse()

	p := doctor.Collect(context.Background(), doctor.ExecRunner{}, time.Now())
	raw, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "shorts-doctor: profile 序列化失败: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "shorts-doctor: 建目录失败: %v\n", err)
		os.Exit(1)
	}
	// 文件名 = 内容寻址 digest（可回查：重算文件内容摘要须一致）。
	name := strings.TrimPrefix(p.Digest, "sha256:") + ".json"
	path := filepath.Join(*outDir, name)
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "shorts-doctor: 落盘失败: %v\n", err)
		os.Exit(1)
	}
	summary, _ := json.Marshal(map[string]string{
		"artifact": path,
		"digest":   p.Digest,
	})
	fmt.Println(string(summary))
}
