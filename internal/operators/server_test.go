package operators

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Cloudbird-Software/Shorts_Director/internal/operator"
)

// buildOperatorCLI 现场构建 shorts-operator 二进制（CLI 端到端用）。
func buildOperatorCLI(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "shorts-operator")
	out, err := exec.Command("go", "build", "-o", bin,
		"github.com/Cloudbird-Software/Shorts_Director/cmd/shorts-operator").CombinedOutput()
	if err != nil {
		t.Fatalf("构建 CLI 失败: %v: %s", err, out)
	}
	return bin
}

func serve(t *testing.T, input string, h Handler) operator.Response {
	t.Helper()
	var out bytes.Buffer
	if err := Serve(context.Background(), strings.NewReader(input), &out, h); err != nil {
		t.Fatal(err)
	}
	resp, err := operator.DecodeResponse(out.Bytes())
	if err != nil {
		t.Fatalf("Serve 必须输出合法响应: %v; raw=%s", err, out.String())
	}
	return resp
}

func TestServeBadJSON(t *testing.T) {
	resp := serve(t, "not-json", nil)
	if resp.Status != operator.StatusInputError || resp.Error.Code != "BAD_REQUEST" {
		t.Fatalf("resp=%+v", resp)
	}
}

func TestServeInvalidRequest(t *testing.T) {
	// 缺 workdir/determinism 的非法请求
	resp := serve(t, `{"contract_version":1,"op":"probe","inputs":{}}`, nil)
	if resp.Status != operator.StatusInputError {
		t.Fatalf("resp=%+v", resp)
	}
}

func TestServeHandlerError(t *testing.T) {
	h := func(ctx context.Context, req operator.Request) (operator.Response, error) {
		return operator.Response{}, context.DeadlineExceeded
	}
	resp := serve(t, `{"contract_version":1,"op":"probe","inputs":{},"workdir":"/w","determinism":{"seed":null}}`, h)
	if resp.Status != operator.StatusRuntimeError {
		t.Fatalf("handler 系统错误应折叠 RUNTIME_ERROR: %+v", resp)
	}
	if resp.Op != "probe" {
		t.Fatalf("op 应回填请求值: %+v", resp)
	}
}

func TestServeOK(t *testing.T) {
	h := func(ctx context.Context, req operator.Request) (operator.Response, error) {
		return okResponse(req.Op, map[string]any{"value": true}, 0, nil), nil
	}
	resp := serve(t, `{"contract_version":1,"op":"probe","inputs":{},"workdir":"/w","determinism":{"seed":null}}`, h)
	if resp.Status != operator.StatusOK || resp.Outputs["value"] != true {
		t.Fatalf("resp=%+v", resp)
	}
	if resp.OperatorVersion != "shorts-operator/go" {
		t.Fatalf("operator_version=%q", resp.OperatorVersion)
	}
}

// 违反契约的 handler 产物（OK 带 error）被兜底为 RUNTIME_ERROR，不输出坏 JSON。
func TestServeContractViolationFallback(t *testing.T) {
	h := func(ctx context.Context, req operator.Request) (operator.Response, error) {
		r := okResponse(req.Op, map[string]any{}, 0, nil)
		r.Error = &operator.OpError{Message: "矛盾"}
		return r, nil
	}
	resp := serve(t, `{"contract_version":1,"op":"probe","inputs":{},"workdir":"/w","determinism":{"seed":null}}`, h)
	if resp.Status != operator.StatusRuntimeError {
		t.Fatalf("resp=%+v", resp)
	}
	if !strings.Contains(resp.Error.Message, "契约") {
		t.Fatalf("错误信息应指向契约违背: %+v", resp.Error)
	}
}

// CLI 端到端：go build 出的二进制经 LocalRunner 走完整 C2 协议。
func TestCLIEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("环境无 go 工具链")
	}
	bin := buildOperatorCLI(t)
	r := &operator.LocalRunner{Bin: bin}
	req := operator.Request{
		ContractVersion: 1, Op: "probe",
		Inputs:      map[string]any{"media_path": "/definitely/not/exist.mp4"},
		Workdir:     "/tmp/probe-cli",
		Determinism: operator.Determinism{},
	}
	resp, err := r.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != operator.StatusInputError || resp.Error.Code != "BAD_MEDIA" {
		t.Fatalf("resp=%+v", resp)
	}
	if resp.OperatorVersion != "shorts-operator/go" {
		t.Fatalf("operator_version=%q", resp.OperatorVersion)
	}
}

func TestCLIUnknownOp(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("环境无 go 工具链")
	}
	bin := buildOperatorCLI(t)
	r := &operator.LocalRunner{Bin: bin}
	_, err := r.Run(context.Background(), operator.Request{
		ContractVersion: 1, Op: "no_such_op",
		Inputs: map[string]any{}, Workdir: "/tmp/x", Determinism: operator.Determinism{},
	})
	if err == nil {
		t.Fatal("未注册算子应报错")
	}
}
