// fake.go 实现 FakeRunner：读 golden fixtures 的测试替身。
// "每个算子都必须提供一套 golden fixtures"是 C2 契约的交付义务——
// Go 业务逻辑因此可以完全单元测试，不依赖 Python 环境。
package operator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Cloudbird-Software/Shorts_Director/internal/digest"
)

// FakeRunner 按"影响输出的请求字段"内容寻址查 golden 响应：
//
//	testdata/golden/<op>/<digest>.json
//
// digest 覆盖 op + inputs + params + determinism.seed（workdir 是
// 路径不影响输出）。同请求恒得同响应——与算子的确定性义务同构。
type FakeRunner struct {
	// Dir 是 golden 根目录（如 testdata/golden）。
	Dir string
}

// Run 查表返回 golden 响应；缺 fixture 是测试数据缺口，报错指路。
func (f *FakeRunner) Run(ctx context.Context, req Request) (Response, error) {
	if err := req.Validate(); err != nil {
		return Response{}, err
	}
	key, err := GoldenKey(req)
	if err != nil {
		return Response{}, err
	}
	path := filepath.Join(f.Dir, req.Op, key+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return Response{}, fmt.Errorf(
			"operator/fake: 缺 golden fixture %s（算子作者交付义务）；请求键 %s: %w",
			path, key, err)
	}
	resp, err := DecodeResponse(raw)
	if err != nil {
		return Response{}, fmt.Errorf("operator/fake: %s: %w", path, err)
	}
	if resp.Op != req.Op {
		return Response{}, fmt.Errorf("operator/fake: %s 的 op=%q 与请求 %q 不符",
			path, resp.Op, req.Op)
	}
	return resp, nil
}

// GoldenKey 对影响输出的请求字段（op+inputs+params+determinism）做
// JCS 摘要。导出供 fixture 生成工具与跨包测试使用。
func GoldenKey(req Request) (string, error) {
	material := map[string]any{
		"op":          req.Op,
		"inputs":      req.Inputs,
		"params":      req.Params,
		"determinism": req.Determinism,
	}
	h, err := digest.ValueDigest(material)
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(h, "sha256:"), nil
}
