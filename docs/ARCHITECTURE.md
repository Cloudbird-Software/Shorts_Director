# 架构纪律（每个模块都必须遵守）

> 从 AGENTS.md 拆出以省上下文。新建模块、动模块边界、review 时读。
> 完整工程设计见 [Engineering_plan.md](Engineering_plan.md)。

## 分层心智模型（全系统唯一心智模型）

```
L4 意图层  BrandKernel / CampaignGoal / ContentPillar      —— 这家店是谁，要说什么
L3 范式层  BeatSchema / ShotSlotQuery / CopyFunction / StyleTheme —— 叙事骨架与风格
L2 物料层  Asset / Shot / AudioTrack / VoiceProfile / Overlay      —— 手上有什么
L1 产物层  MonthlySchedule / VideoPlan(IR) / RenderArtifact / QCReport / Delivery
```

**最高不变式：L3 永远不引用 L2 的具体实例，只引用"等价类谓词"。**
BeatSchema 中出现 `asset_id` 即架构违规（写成 CI 检查）。
依赖方向只允许上层引用下层；L1 产物通过 `VersionedRef` 引用 L3/L2。

## 仓库布局

```
schema/            唯一真源，人手写（规则见 schema/AGENTS.md）
├── vocab/v1/      受控词表（YAML，enum 只允许追加）
├── entities/      实体 JSON Schema（结构 + 跨字段不变式编号 IV-*）
├── contracts/     C2 算子协议 / C3 渲染契约
└── testdata/      valid / invalid / evolution 三类样本
codegen/           make gen 产物（Go/TS/BAML/SQL），禁止手改
src/               TS 侧（渲染层 / schema 工具），受"渲染确定性三禁令"约束
operators/         Python 无状态算子（每算子独立镜像，只走 C2 契约）
```

后续控制面（Go）落地时按 `internal/<service>` 组织，服务清单见
Engineering_plan.md §3.1（S1–S11）与契约清单 C0–C6。

## 模块边界规则

1. **每个模块一个 public entry**（`index.ts` / `index.go`）。跨模块只能 import
   entry，禁止深入内部实现文件。`make arch` 会检查。
2. **entry 文件不 export 内部实现类型**——接口必须真正收敛在边界上。
3. **每个模块目录一份 `AGENTS.md`**：写清该模块负责什么、不变量是什么、
   禁止做什么、如何独立验证。
4. **契约测试在模块边界**，实现细节内部自由。这样模块内可以大改而外部测试不动。
5. **模块大小上限 3000 行**。超过就拆——一个模块必须能被 agent 一次性完整读完。
6. **生成代码进独立目录**（`codegen/`、`*.gen.ts`、`baml_client/` 等），禁止手改。
7. **接口设计标准**：一个 LLM 能否仅凭函数签名 + 一行 docstring 就零样本正确使用？
   答案是否 => 接口太浅，重做。
8. **测试优先级**：行为不变量用 property-based test（vitest + fast-check），
   关键输出用 golden test。先写不变量，再写实现。

## 契约纪律（Phase 0 核心）

- **契约先冻结，实现后填充**（A6）。冻结准入条件 = Freeze Gate 12 项
  （Engineering_plan.md §4.8），未过 Gate 的契约不得被实现依赖。
- Schema 版本 `<entity>/<major>`，破坏性变更才升 major；词表 enum 只允许追加，
  废弃用 `deprecated: true` + `replaced_by`，不删。
- 跨实体引用一律 `VersionedRef {id, version}`，禁止裸 id 引用可变实体。
- 时间全系统用**整数帧**（DB 存 `_frame` 后缀），禁止 float seconds 进任何 schema。
- 实体 ID 用 UUIDv7；范式/词表用可读稳定 slug（如 `bs.food.origin_story`）。
- 内容寻址 digest 一律 **RFC 8785（JCS）规范化**后再 sha256，跨语言必须一致。

## 语言栈约束

| 区域   | 语言/栈                | 硬约束                                                         |
| ------ | ---------------------- | -------------------------------------------------------------- |
| 控制面 | Go（modular monolith） | Planner 纯函数；一切非确定性显式落盘为内容寻址 artifact        |
| 算子层 | Python（无状态 CLI）   | 只走 C2：JSON stdin/stdout + 文件路径，不连库、不知租户        |
| 渲染层 | TS + Remotion          | 三禁令：禁 `Date`、禁 `Math.random`、禁网络请求（eslint 强制） |
| LLM 层 | BAML                   | 返回类型必须引用 codegen enum，禁止裸 string（B-1）            |

## 依赖规则

新增依赖前先列出"依赖名 / 用途 / 许可证 / 是否能用标准库替代"等人确认；
禁止引入 AGPL / GPL-3.0 / SSPL 的库。
规则引擎在 `.dependency-cruiser.cjs`，新模块落地时必须同步补全其中的 TODO 边界规则。
