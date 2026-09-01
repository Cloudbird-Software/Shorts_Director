# videoplan —— VideoPlan IR v1 实体层

schema/entities/video_plan.schema.json 的 Go 映射：生成编排（internal/eval，
IR-0007）的时间线产物，Compiler（C3）的输入。结构体字段与 schema 一一对应
（json tag 对齐），漂移由 round-trip 测试发现（G1 valid 样本必须能反序列化
并通过 Validate）。通用溯源块与版本化引用（provenance.go）随 IR-0007 退役-2/3
自 internal/entity 迁入本包；G2 双语言一致性 Go 侧锚点见 consistency_test.go。

## 不变量（Validate 强制）

- IV-VP-1 帧数守恒：VIDEO_MAIN 轨 clips 无缝铺满 [0, 总时长]；其余轨不得超出
- IV-VP-2 clip 不越界：src_out-src_in ≥ 1 且 tl_end > tl_start，起点非负
- IV-VP-3 字幕/overlay 帧区间落在总时长内；overlay layout_box 不越 safe_area
  （按 anchor 语义换算左上角后判界；渲染器 R-3 复检）
- IV-VP-4 overlay.component 必须在 overlayComponentWhitelist（渲染组件注册表）
- IV-VP-5 依赖的 aigc_disclosure 字段尚未进入 schema v1——字段落地时补
- speed ∈ [1, 1.15]（工艺上限，Engineering_plan §S5 钉死）
- timebase.unit=frame 且 rate == canvas.fps（禁 float seconds）

## 禁止

- 给 Plan 增加任何 schema 之外的字段（漂移 = 缺陷）
- 浮点秒进入任何时间字段
- 白名单外组件 id

## 验证

`make go-check`（gofmt + go vet + go test ./internal/videoplan/...）
