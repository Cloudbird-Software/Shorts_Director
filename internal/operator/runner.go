// runner.go 实现 C2 调用协议的三种执行器：
//
//	operator <op_name> --contract-version 1 < request.json > response.json
//
// LocalRunner（开发）、DockerRunner（生产，每算子独立镜像）、
// FakeRunner（golden fixtures，测试——见 fake.go）。
package operator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

// Runner 执行一次 C2 算子调用。实现负责把 Request 序列化进算子、
// 把 stdout 解码成 Response；业务四态（OK/INPUT_ERROR/…）经
// Response.Status 表达，只有系统级故障才返回 error。
type Runner interface {
	Run(ctx context.Context, req Request) (Response, error)
}

// LocalRunner 直接 exec 本地算子可执行文件（开发用）。
type LocalRunner struct {
	// Bin 是算子入口（脚本或二进制），接收 <op> --contract-version 1 参数。
	Bin string
	// Envs 附加环境变量（如 CUDA_VISIBLE_DEVICES）。
	Envs []string
}

// Run 执行算子：request 进 stdin，response 从 stdout 解码。
// 算子输出合法四态响应时，即便进程非零退出也以 Response 返回
// （算子被要求在失败时输出结构化错误）。
func (r *LocalRunner) Run(ctx context.Context, req Request) (Response, error) {
	if err := req.Validate(); err != nil {
		return Response{}, err
	}
	args, stdin, err := encodeCall(req)
	if err != nil {
		return Response{}, err
	}
	cmd := exec.CommandContext(ctx, r.Bin, args...)
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.Env = append(cmd.Environ(), r.Envs...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err == nil {
		return DecodeResponse(out)
	}
	// 进程失败但 stdout 有合法结构化响应：算子已履约
	if resp, derr := DecodeResponse(out); derr == nil {
		return resp, nil
	}
	if ctx.Err() != nil {
		return Response{}, fmt.Errorf("operator: %s 被取消/超时: %w", req.Op, ctx.Err())
	}
	return Response{}, fmt.Errorf("operator: %s 执行失败: %v; stderr: %s",
		req.Op, err, stderr.String())
}

// DockerRunner 经 docker run 调用每算子独立镜像（生产用）。
// 网络禁用（算子不许联网），workdir 挂载进容器。
type DockerRunner struct {
	// Images 是 op → 镜像名（含 tag），如 "shot_segment": "shot_segment:1.2.0"。
	Images map[string]string
	// ExtraArgs 追加 docker 参数（--gpus 等）。
	ExtraArgs []string
}

// Run 构造 docker 命令执行算子。
func (r *DockerRunner) Run(ctx context.Context, req Request) (Response, error) {
	if err := req.Validate(); err != nil {
		return Response{}, err
	}
	image, ok := r.Images[req.Op]
	if !ok {
		return Response{}, fmt.Errorf("operator: 算子 %q 无镜像注册", req.Op)
	}
	args := dockerArgs(image, req.Workdir, r.ExtraArgs, req.Op)
	stdin, err := json.Marshal(req)
	if err != nil {
		return Response{}, err
	}
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdin = bytes.NewReader(stdin)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if resp, derr := DecodeResponse(out); derr == nil {
			return resp, nil
		}
		if ctx.Err() != nil {
			return Response{}, fmt.Errorf("operator: %s 被取消/超时: %w", req.Op, ctx.Err())
		}
		return Response{}, fmt.Errorf("operator: docker %s: %v; stderr: %s", req.Op, err, stderr.String())
	}
	return DecodeResponse(out)
}

// dockerArgs 构造 docker 命令行（独立函数便于表驱动测试）：
// run --rm -i --network none -v <workdir>:<workdir> <image> <op> --contract-version 1
func dockerArgs(image, workdir string, extra []string, op string) []string {
	args := []string{"run", "--rm", "-i", "--network", "none"}
	args = append(args, extra...)
	args = append(args, "-v", workdir+":"+workdir, image,
		op, "--contract-version", "1")
	return args
}

// encodeCall 序列化请求并返回算子 CLI 参数。
func encodeCall(req Request) ([]string, []byte, error) {
	raw, err := json.Marshal(req)
	if err != nil {
		return nil, nil, fmt.Errorf("operator: 请求序列化失败: %w", err)
	}
	return []string{req.Op, "--contract-version", "1"}, raw, nil
}
