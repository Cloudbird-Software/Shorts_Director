// Package contracts 是 schema/ 单一真源在 Go 侧的锚点：
// 实体 schema 版本、服务间契约版本与画布/工艺常量。
// 与 src/contracts/versions.ts 逐字对应——修改任何一侧必须同步另一侧，
// 并保持与 schema/*.schema.json 的实际版本一致。
package contracts

// SchemaName 是已冻结实体 schema 的名字（schema/entities/<name>.schema.json）。
type SchemaName string

const (
	SchemaBrandKernel     SchemaName = "brand_kernel"     // v1
	SchemaShot            SchemaName = "shot"             // v1
	SchemaAsset           SchemaName = "asset"            // v1
	SchemaShotSlotQuery   SchemaName = "shot_slot_query"  // v1
	SchemaVideoPlan       SchemaName = "video_plan"       // v1
	SchemaQCAssertion     SchemaName = "qc_assertion"     // v1
	SchemaProductionOrder SchemaName = "production_order" // v1
	SchemaEvent           SchemaName = "event"            // v1
)

// SchemaVersion 返回 "<entity>/<major>" 形式的版本锚点。
func SchemaVersion(n SchemaName) string {
	return string(n) + "/1"
}

// 服务间契约版本（schema/contracts/ 下 request/response 的 contract_version）。
const (
	ContractOperator = 1 // C2 控制面 ↔ 算子（无状态 CLI）
	ContractRender   = 1 // C3 控制面 ↔ 渲染器
)

// Canvas 是 VideoPlan IR 的画布硬约束。
type Canvas struct {
	Width  int
	Height int
	FPS    []int
}

// DefaultCanvas 返回 1080×1920、fps ∈ {25,30} 的画布约束。
func DefaultCanvas() Canvas {
	return Canvas{Width: 1080, Height: 1920, FPS: []int{25, 30}}
}

// RenderCraft 集中渲染工艺上限，可调可测（Engineering_plan §5.5）。
type RenderCraft struct {
	MaxSpeed                float64 // 变速上限，超过有明显不自然感
	BeatSnapToleranceFrames int     // 卡点吸附容差（帧）
	TargetLufs              float64 // 目标响度（LUFS）
}

// DefaultRenderCraft 返回 v1 工艺参数。
func DefaultRenderCraft() RenderCraft {
	return RenderCraft{MaxSpeed: 1.15, BeatSnapToleranceFrames: 3, TargetLufs: -14}
}
