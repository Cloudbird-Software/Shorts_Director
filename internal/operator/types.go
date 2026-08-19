// Package operator 实现 C2 契约（schema/contracts/operator/*.json）：
// 控制面 ↔ Python 算子。算子是纯函数 CLI（stdin/stdout JSON + 文件路径），
// 不知道数据库/租户/业务。同输入（含 seed）必须同输出——算子作者
// 交付义务包括 golden fixtures。
package operator

import (
	"encoding/json"
	"fmt"

	"github.com/Cloudbird-Software/Shorts_Director/internal/contracts"
)

// Status 是算子执行的四态。
type Status string

const (
	StatusOK           Status = "OK"
	StatusInputError   Status = "INPUT_ERROR"   // 坏输入，可重传修复
	StatusRuntimeError Status = "RUNTIME_ERROR" // 算子内部故障
	StatusTimeout      Status = "TIMEOUT"
)

// Determinism 是确定性要求：seed 钉死后同输入同输出。
type Determinism struct {
	Seed       *int64 `json:"seed"`        // null = 无随机性
	CPUThreads *int   `json:"cpu_threads"` // 数值可复现性要求时钉死
}

// Request 是 C2 OperatorRequest v1。
type Request struct {
	ContractVersion int            `json:"contract_version"`
	Op              string         `json:"op"`
	Inputs          map[string]any `json:"inputs"` // 媒体一律绝对路径，禁止 URL
	Params          map[string]any `json:"params,omitempty"`
	Workdir         string         `json:"workdir"` // 内容寻址工作目录
	Determinism     Determinism    `json:"determinism"`
}

// Artifact 是中间产物声明（关键帧等，A2 落盘可追溯）。
type Artifact struct {
	Role  string   `json:"role"`
	Paths []string `json:"paths"`
}

// Metrics 是成本治理（CostGovernor）的计费依据。
type Metrics struct {
	WallMs    int64   `json:"wall_ms"`
	GpuSecond float64 `json:"gpu_seconds,omitempty"`
	PeakMemMB float64 `json:"peak_mem_mb,omitempty"`
}

// OpError 是算子错误：人话、可执行——外包返修指令的上游。
type OpError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable,omitempty"`
}

// Response 是 C2 OperatorResponse v1。
type Response struct {
	ContractVersion int               `json:"contract_version"`
	Op              string            `json:"op"`
	Status          Status            `json:"status"`
	Outputs         map[string]any    `json:"outputs"`
	Artifacts       []Artifact        `json:"artifacts,omitempty"`
	Metrics         Metrics           `json:"metrics"`
	OperatorVersion string            `json:"operator_version"`
	ModelVersions   map[string]string `json:"model_versions,omitempty"`
	Error           *OpError          `json:"error,omitempty"`
}

// Validate 校验请求的契约形态（C2 request schema 的 Go 侧镜像）。
func (r Request) Validate() error {
	if r.ContractVersion != contracts.ContractOperator {
		return fmt.Errorf("operator: contract_version 必须 %d", contracts.ContractOperator)
	}
	if r.Op == "" {
		return fmt.Errorf("operator: op 必填")
	}
	if r.Inputs == nil {
		return fmt.Errorf("operator: inputs 必填（可为空对象）")
	}
	if r.Workdir == "" {
		return fmt.Errorf("operator: workdir 必填（内容寻址）")
	}
	return nil
}

// Validate 校验响应的契约形态与状态语义：INPUT_ERROR 必须带可执行的
// 错误描述；OK 时不得携带 error。
func (r Response) Validate() error {
	if r.ContractVersion != contracts.ContractOperator {
		return fmt.Errorf("operator: response contract_version 必须 %d", contracts.ContractOperator)
	}
	if r.Op == "" {
		return fmt.Errorf("operator: response op 必填")
	}
	switch r.Status {
	case StatusOK:
		if r.Error != nil {
			return fmt.Errorf("operator: OK 响应不得携带 error")
		}
	case StatusInputError, StatusRuntimeError, StatusTimeout:
		if r.Error == nil || r.Error.Message == "" {
			return fmt.Errorf("operator: %s 必须带可执行的 error.message", r.Status)
		}
	default:
		return fmt.Errorf("operator: status 非法 %q", r.Status)
	}
	if r.Outputs == nil {
		return fmt.Errorf("operator: outputs 必填（可为空对象）")
	}
	if r.Metrics.WallMs < 0 {
		return fmt.Errorf("operator: metrics.wall_ms 不得为负")
	}
	if r.OperatorVersion == "" {
		return fmt.Errorf("operator: operator_version 必填（如 shot_segment@1.2.0）")
	}
	for i, a := range r.Artifacts {
		if a.Role == "" || len(a.Paths) == 0 {
			return fmt.Errorf("operator: artifacts[%d] 需要 role 与 ≥1 paths", i)
		}
	}
	return nil
}

// DecodeResponse 从算子 stdout 解码并校验响应。
func DecodeResponse(raw []byte) (Response, error) {
	var resp Response
	if err := json.Unmarshal(raw, &resp); err != nil {
		return Response{}, fmt.Errorf("operator: 响应不是合法 JSON: %w", err)
	}
	if err := resp.Validate(); err != nil {
		return Response{}, err
	}
	return resp, nil
}
