// Package operators 是 C2 算子协议的服务端骨架与纯 Go 可实现的算子。
// 每个算子是无状态纯函数：Request → Response，不知道数据库/租户/业务。
// Python 重模型算子（SyncNet/VLM 等）独立成镜像，不经本包——
// 本包只收编"外部二进制即可完成"的算子（如 probe 走 ffprobe）。
package operators

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/Cloudbird-Software/Shorts_Director/internal/contracts"
	"github.com/Cloudbird-Software/Shorts_Director/internal/operator"
)

// operatorVersion 是本包 Go 算子的版本指纹（C2 response.operator_version）。
const operatorVersion = "shorts-operator/go"

// Handler 是一个算子的实现：返回 Response 的 status/outputs/artifacts，
// 或系统级 error（由 Serve 折叠为 RUNTIME_ERROR——CLI 永远输出合法响应）。
type Handler func(ctx context.Context, req operator.Request) (operator.Response, error)

// Serve 处理一次 C2 调用：解码请求 → 校验 → 执行 → 写出响应。
// 响应写不出（管道断裂）才是本函数的 error。
func Serve(ctx context.Context, r io.Reader, w io.Writer, h Handler) error {
	var req operator.Request
	if err := json.NewDecoder(r).Decode(&req); err != nil {
		return writeResponse(w, inputErrorResponse("unknown", "BAD_REQUEST", "请求不是合法 JSON: "+err.Error()))
	}
	if err := req.Validate(); err != nil {
		return writeResponse(w, inputErrorResponse(req.Op, "BAD_REQUEST", err.Error()))
	}
	resp, err := h(ctx, req)
	if err != nil {
		resp = runtimeErrorResponse(req.Op, err)
	}
	if resp.Op == "" {
		resp.Op = req.Op
	}
	return writeResponse(w, resp)
}

func writeResponse(w io.Writer, resp operator.Response) error {
	if err := resp.Validate(); err != nil {
		// 兜底：算子产物不合法本身按 RUNTIME_ERROR 报，不输出坏 JSON
		resp = runtimeErrorResponse(resp.Op, fmt.Errorf("算子响应违反契约: %w", err))
	}
	enc := json.NewEncoder(w)
	return enc.Encode(resp)
}

// baseResponse 组装带公共字段的响应骨架（handler/错误构造器只补业务字段）。
func baseResponse(op string, status operator.Status) operator.Response {
	return operator.Response{
		ContractVersion: contracts.ContractOperator,
		Op:              op,
		Status:          status,
		Outputs:         map[string]any{},
		OperatorVersion: operatorVersion,
	}
}

func inputErrorResponse(op, code, msg string) operator.Response {
	resp := baseResponse(op, operator.StatusInputError)
	resp.Metrics = operator.Metrics{WallMs: 0}
	resp.Error = &operator.OpError{Code: code, Message: msg, Retryable: false}
	return resp
}

func runtimeErrorResponse(op string, err error) operator.Response {
	resp := baseResponse(op, operator.StatusRuntimeError)
	resp.Metrics = operator.Metrics{WallMs: 0}
	resp.Error = &operator.OpError{Code: "INTERNAL", Message: err.Error()}
	return resp
}

// okResponse 组装 OK 响应（handler 只填业务字段）。
func okResponse(op string, outputs map[string]any, wall time.Duration, models map[string]string) operator.Response {
	resp := baseResponse(op, operator.StatusOK)
	resp.Outputs = outputs
	resp.Metrics = operator.Metrics{WallMs: wall.Milliseconds()}
	resp.ModelVersions = models
	return resp
}
