# internal/ —— Go 控制面（modular monolith）

本目录承载 Engineering_plan.md §3.1 的控制面服务（S1–S11）与公共底座。
语言：Go（标准库优先，新增第三方依赖须先报批）；LLM 提示契约一律 BAML。

## 模块规约

- 每个服务一个子包：`internal/<service>/`，对外 API 只从包级符号进入
  （Go 包本身即边界，禁止跨包 import 内部文件——Go 编译器天然强制）。
- 每个子包落位时必须附 `AGENTS.md`：负责什么、不变量、禁止什么、如何验证。
- 模块大小上限 3000 行（含测试），超过就拆。
- 契约（C2/C3）数据结构以 `schema/` JSON Schema 为唯一真源；
  Go 结构体手写并保持字段 tag 与 schema 一致，漂移由 `make go-check`
  中的 round-trip 测试发现。

## 硬约束（继承 docs/ARCHITECTURE.md）

1. Planner / 求值器必须是纯函数：同输入同输出，禁止读取时钟、随机源、网络。
2. 一切非确定性（LLM、模型推理）只允许出现在 BAML 层与算子边界，
   产物以内容寻址 digest 落盘（RFC 8785 JCS + sha256，见 internal/digest）。
3. 时间一律整数帧（`*_frame` 字段），禁止 float seconds 进入任何结构体。
4. 跨实体引用一律 `VersionedRef{ID, Version}`，禁止裸 id 引用可变实体。
5. 实体 ID 用 UUIDv7 字符串；范式/词表用可读 slug。

## 验证

- `make go-check` = gofmt 检查 + go vet + go test ./...（本地门槛，
  CI 接线待人类批准后挂入 .github/workflows）。
