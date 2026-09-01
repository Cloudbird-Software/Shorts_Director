// cli_test.go —— shorts-doctor CLI 端到端：构建二进制、真实环境探测、
// 校验落盘 artifact 文件名 = 内容寻址 digest（IFACE-2 可回查）。
// 无 go 工具链时 skip（CI ubuntu 有 go）。
package doctor

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctorCLIArtifactContentAddressed(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go 工具链缺失")
	}
	bin := filepath.Join(t.TempDir(), "shorts-doctor")
	build := exec.Command(goBin, "build", "-o", bin,
		"github.com/Cloudbird-Software/Shorts_Director/cmd/shorts-doctor")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("构建失败: %v\n%s", err, out)
	}
	outDir := filepath.Join(t.TempDir(), "profiles")
	run := exec.Command(bin, "-out", outDir)
	out, err := run.Output()
	if err != nil {
		t.Fatalf("运行失败: %v\nstderr 详见下方输出: %s", err, out)
	}
	var summary struct {
		Artifact string `json:"artifact"`
		Digest   string `json:"digest"`
	}
	if err := json.Unmarshal(out, &summary); err != nil {
		t.Fatalf("stdout 非机器可读 JSON: %v\n%s", err, out)
	}
	if !strings.HasPrefix(summary.Digest, "sha256:") {
		t.Fatalf("digest 形态异常: %q", summary.Digest)
	}
	raw, err := os.ReadFile(summary.Artifact)
	if err != nil {
		t.Fatalf("artifact 不可读: %v", err)
	}
	// 回查：文件内容重新求摘要必须与 digest 一致（内容寻址）。
	var p Profile
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("artifact 非合法 profile: %v", err)
	}
	if d, err := p.ComputeDigest(); err != nil || d != summary.Digest {
		t.Fatalf("回查摘要不一致: %s vs %s (err=%v)", d, summary.Digest, err)
	}
	if filepath.Base(summary.Artifact) != strings.TrimPrefix(summary.Digest, "sha256:")+".json" {
		t.Fatalf("artifact 文件名必须是 digest hex: %s", summary.Artifact)
	}
	if p.SchemaVersion != SchemaVersion {
		t.Fatalf("schema_version 异常: %q", p.SchemaVersion)
	}
}
