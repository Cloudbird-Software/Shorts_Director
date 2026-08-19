package operator

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRequestValidate(t *testing.T) {
	cases := []struct {
		name    string
		req     Request
		wantErr bool
	}{
		{"合法", Request{ContractVersion: 1, Op: "probe", Inputs: map[string]any{}, Workdir: "/w"}, false},
		{"版本错", Request{ContractVersion: 2, Op: "probe", Inputs: map[string]any{}, Workdir: "/w"}, true},
		{"缺 op", Request{ContractVersion: 1, Inputs: map[string]any{}, Workdir: "/w"}, true},
		{"缺 inputs", Request{ContractVersion: 1, Op: "probe", Workdir: "/w"}, true},
		{"缺 workdir", Request{ContractVersion: 1, Op: "probe", Inputs: map[string]any{}}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.req.Validate()
			if (err != nil) != c.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, c.wantErr)
			}
		})
	}
}

func TestResponseValidate(t *testing.T) {
	ok := Response{
		ContractVersion: 1, Op: "probe", Status: StatusOK,
		Outputs: map[string]any{}, Metrics: Metrics{WallMs: 10},
		OperatorVersion: "probe@1.0.0",
	}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	inputErr := ok
	inputErr.Status = StatusInputError
	if err := inputErr.Validate(); err == nil {
		t.Fatal("INPUT_ERROR 必须带 error")
	}
	inputErr.Error = &OpError{Code: "BAD_MEDIA", Message: "文件不存在：/x.mp4"}
	if err := inputErr.Validate(); err != nil {
		t.Fatal(err)
	}
	withErr := ok
	withErr.Error = &OpError{Message: "x"}
	if err := withErr.Validate(); err == nil {
		t.Fatal("OK 不得携带 error")
	}
	badStatus := ok
	badStatus.Status = "MAYBE"
	if err := badStatus.Validate(); err == nil {
		t.Fatal("status 非法应报错")
	}
}

// okResponse 是 fixture 算子的合法响应。
const okResponse = `{"contract_version":1,"op":"probe","status":"OK",
 "outputs":{"width":1080,"fps":30},"metrics":{"wall_ms":12},
 "operator_version":"probe@1.0.0","model_versions":{"ffprobe":"7.1"}}`

func writeScript(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func sampleReq() Request {
	seed := int64(42)
	return Request{
		ContractVersion: 1, Op: "probe",
		Inputs:      map[string]any{"media_path": "/mnt/work/abc.mp4"},
		Workdir:     "/mnt/work/job-1",
		Determinism: Determinism{Seed: &seed},
	}
}

func TestLocalRunnerOK(t *testing.T) {
	dir := t.TempDir()
	bin := writeScript(t, dir, "ok.sh", "#!/bin/sh\ncat <<'EOF'\n"+okResponse+"\nEOF\n")
	r := &LocalRunner{Bin: bin}
	resp, err := r.Run(context.Background(), sampleReq())
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != StatusOK || resp.Outputs["fps"] != float64(30) {
		t.Fatalf("resp=%+v", resp)
	}
	if resp.ModelVersions["ffprobe"] != "7.1" {
		t.Fatalf("model_versions 未回填（A2 provenance 上游）")
	}
}

// 算子失败但已履约输出结构化错误：作为业务结果返回，不是系统错误。
func TestLocalRunnerStructuredError(t *testing.T) {
	dir := t.TempDir()
	body := "#!/bin/sh\ncat <<'EOF'\n" +
		`{"contract_version":1,"op":"probe","status":"INPUT_ERROR","outputs":{},` +
		`"metrics":{"wall_ms":3},"operator_version":"probe@1.0.0",` +
		`"error":{"code":"BAD_MEDIA","message":"文件不存在","retryable":true}}` +
		"\nEOF\nexit 1\n"
	bin := writeScript(t, dir, "err.sh", body)
	r := &LocalRunner{Bin: bin}
	resp, err := r.Run(context.Background(), sampleReq())
	if err != nil {
		t.Fatalf("结构化失败应作为 Response 返回: %v", err)
	}
	if resp.Status != StatusInputError || resp.Error.Retryable != true {
		t.Fatalf("resp=%+v", resp)
	}
}

func TestLocalRunnerGarbage(t *testing.T) {
	dir := t.TempDir()
	bin := writeScript(t, dir, "garbage.sh", "#!/bin/sh\necho not-json; exit 2\n")
	r := &LocalRunner{Bin: bin}
	if _, err := r.Run(context.Background(), sampleReq()); err == nil {
		t.Fatal("垃圾输出应报系统错误")
	}
}

func TestLocalRunnerTimeout(t *testing.T) {
	dir := t.TempDir()
	bin := writeScript(t, dir, "slow.sh", "#!/bin/sh\nsleep 5\n")
	r := &LocalRunner{Bin: bin}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := r.Run(ctx, sampleReq()); err == nil {
		t.Fatal("超时应报错")
	}
}

func TestDockerArgs(t *testing.T) {
	got := dockerArgs("probe:1.0", "/mnt/w", []string{"--gpus", "all"}, "probe")
	want := []string{"run", "--rm", "-i", "--network", "none",
		"--gpus", "all", "-v", "/mnt/w:/mnt/w", "probe:1.0",
		"probe", "--contract-version", "1"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("docker args:\n got %v\nwant %v", got, want)
	}
}

func TestDockerRunnerUnregistered(t *testing.T) {
	r := &DockerRunner{Images: map[string]string{}}
	if _, err := r.Run(context.Background(), sampleReq()); err == nil {
		t.Fatal("未注册镜像应报错")
	}
}

func TestFakeRunnerGolden(t *testing.T) {
	dir := t.TempDir()
	req := sampleReq()
	// 生成 golden：键 = 影响输出字段的 JCS 摘要
	key, err := GoldenKey(req)
	if err != nil {
		t.Fatal(err)
	}
	opDir := filepath.Join(dir, req.Op)
	if err := os.MkdirAll(opDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// golden 里嵌一段换行/缩进——FakeRunner 必须容忍任意合法 JSON 排版
	var pretty map[string]any
	if err := json.Unmarshal([]byte(okResponse), &pretty); err != nil {
		t.Fatal(err)
	}
	prettyJSON, _ := json.MarshalIndent(pretty, "", "  ")
	if err := os.WriteFile(filepath.Join(opDir, key+".json"), prettyJSON, 0o644); err != nil {
		t.Fatal(err)
	}
	f := &FakeRunner{Dir: dir}
	resp, err := f.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != StatusOK || resp.Outputs["width"] != float64(1080) {
		t.Fatalf("resp=%+v", resp)
	}
	// 同请求 → 同响应（确定性同构）
	again, err := f.Run(context.Background(), sampleReq())
	if err != nil {
		t.Fatal(err)
	}
	if again.OperatorVersion != resp.OperatorVersion {
		t.Fatal("同请求必须恒得同响应")
	}
}

func TestFakeRunnerMissingFixture(t *testing.T) {
	f := &FakeRunner{Dir: t.TempDir()}
	_, err := f.Run(context.Background(), sampleReq())
	if err == nil || !strings.Contains(err.Error(), "golden fixture") {
		t.Fatalf("缺 fixture 应指路: %v", err)
	}
}

func TestFakeRunnerWorkdirNotInKey(t *testing.T) {
	// workdir 不影响输出：换目录必须命中同一 golden
	a, err := GoldenKey(sampleReq())
	if err != nil {
		t.Fatal(err)
	}
	req := sampleReq()
	req.Workdir = "/mnt/other"
	b, err := GoldenKey(req)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatal("workdir 不应参与 golden 键")
	}
}
