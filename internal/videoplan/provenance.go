// provenance.go 承载通用溯源块与版本化引用（schema/common/provenance、
// versioned_ref 的 Go 侧实体）。原属 internal/entity，IR-0007 退役-2/3
// 迁入 videoplan（唯一存留消费方）。
package videoplan

import (
	"errors"
	"fmt"
)

// GeneratedBy 表示实体产出方式（A2：非确定性显式化）。
type GeneratedBy string

const (
	GeneratedByLLM           GeneratedBy = "LLM"
	GeneratedByHuman         GeneratedBy = "HUMAN"
	GeneratedByDeterministic GeneratedBy = "DETERMINISTIC"
	GeneratedByHybrid        GeneratedBy = "HYBRID"
)

// HumanEdit 是人类编辑轨迹（JSON Patch 级修正记录，飞轮高密度信号源）。
type HumanEdit struct {
	Path   string `json:"path"`             // RFC 6901 JSON Pointer
	Before any    `json:"before,omitempty"` // 修改前值；新增时省略
	After  any    `json:"after"`            // 修改后值；删除时为 null
	Editor string `json:"editor"`
	At     string `json:"at"` // RFC 3339 date-time
}

// Provenance 是通用溯源块（schema/common/provenance.json）。
// 每个 LLM 产出的实体必须携带；Planner 等确定性代码 generated_by=DETERMINISTIC。
type Provenance struct {
	GeneratedBy   GeneratedBy `json:"generated_by"`
	ModelID       string      `json:"model_id"` // DETERMINISTIC 时填代码模块版本
	PromptVersion string      `json:"prompt_version"`
	InputDigest   string      `json:"input_digest"` // sha256(JCS(input))，RFC 8785
	Seed          *int64      `json:"seed"`         // 未用随机性时为 null
	HumanEdits    []HumanEdit `json:"human_edits,omitempty"`
	CreatedAt     string      `json:"created_at"` // RFC 3339 date-time
}

// Validate 校验溯源块的必填语义。
func (p Provenance) Validate() error {
	switch p.GeneratedBy {
	case GeneratedByLLM, GeneratedByHuman, GeneratedByDeterministic, GeneratedByHybrid:
	default:
		return fmt.Errorf("videoplan/provenance: generated_by 非法 %q", p.GeneratedBy)
	}
	if p.ModelID == "" || p.PromptVersion == "" || p.InputDigest == "" || p.CreatedAt == "" {
		return errors.New("videoplan/provenance: model_id/prompt_version/input_digest/created_at 均必填")
	}
	for i, e := range p.HumanEdits {
		if e.Path == "" || e.After == nil && e.Before == nil || e.Editor == "" || e.At == "" {
			return fmt.Errorf("videoplan/provenance: human_edits[%d] 字段不完整", i)
		}
	}
	return nil
}

// VersionedRef 是跨实体引用的唯一合法形式（禁止裸 id 引用可变实体）。
type VersionedRef struct {
	ID      string `json:"id"`      // 可读 slug 或 UUIDv7
	Version int    `json:"version"` // ≥1
}

// Validate 校验引用不退化成裸 id。
func (r VersionedRef) Validate() error {
	if r.ID == "" {
		return errors.New("videoplan/versioned_ref: id 必填")
	}
	if r.Version < 1 {
		return fmt.Errorf("videoplan/versioned_ref: version 必须 ≥1，得到 %d", r.Version)
	}
	return nil
}
