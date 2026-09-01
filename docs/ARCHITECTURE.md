# 架构纪律（每个模块都必须遵守）

> 从 AGENTS.md 拆出以省上下文。新建模块、动模块边界、review 时读。
> 范式依据：specs/IR-0007/spec.md（生成一等公民）；旧意图（实拍素材治理）
> 的完整工程设计见 [history/Engineering_plan.md](history/Engineering_plan.md)。

## 分层心智模型（全系统唯一心智模型）

```
L4 实验层  EvalSuite / RunArtifact / 报告与假设看板        —— 实测认知的载体
L3 编排层  Eval 编排 / VideoPlan IR / ComplianceGate       —— 把形态变成时间线
L2 算子层  gen_i2v / gen_tts / gen_lipsync / vlm_boolean   —— 生成与评审能力（C2）
L1 产物层  RenderArtifact（C3 确定性渲染） / 信息层叠加 / 成品 mp4
```

**最高不变式：生成是一等公民（INV-1）**——上层不得以「实拍素材库治理」
为前提约束下层形态；形态（gen_form）是数据不是代码分支（IFACE-3）。
确定性信息（文字/价格/地址/电话/Logo）只经 L1 模板叠加进入成品（INV-5），
禁止交给生成模型产生。

## 仓库布局

```
schema/            唯一真源，人手写（规则见 schema/AGENTS.md）
├── vocab/v1/      受控词表（YAML，enum 只允许追加；gen_form 钉形态绑定）
├── entities/      实体 JSON Schema（asset / video_plan / qc_assertion / event）
├── contracts/     C2 算子协议 / C3 渲染契约
└── testdata/      valid / invalid / evolution 三类样本
internal/          Go 控制面（模块目录各带 AGENTS.md）
operators/         Python 无状态生成/评审算子（每算子独立镜像，只走 C2 契约）
cmd/               CLI 入口（shorts-doctor / shorts-eval / shorts-render / shorts-operator）
evals/             制式评估套件定义 + mock 商家数据集（版本化目录，禁运行时抓网）
codegen/           make gen 产物（Go/TS/BAML），禁止手改
specs/             IR 与条款级规格
```

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

## 契约纪律

- 生成算子走 **C2 契约**：stdin/stdout JSON、四态语义（INV-4）、确定性字段
  与资源计量字段一个不少；算子无状态、不知租户、不连库。
- 生成算子 op 命名为受控枚举（IFACE-1，首批：图生视频/语音合成/口型同步/
  布尔评审）；新增 op 走清单变更，禁散落命名。
- **capability profile 与 run artifact 必须内容寻址**（IFACE-2）：run artifact
  内嵌套件定义全文与执行环境签名，不依赖外部状态即可复算。
- Schema 版本 `<entity>/<major>`，破坏性变更才升 major；词表 enum 只允许追加，
  废弃用 `deprecated: true` + `replaced_by`，不删。
- 跨实体引用一律 `VersionedRef {id, version}`，禁止裸 id 引用可变实体。
- 时间全系统用**整数帧**（字段名后缀 `_frame`），禁止 float seconds 进任何 schema。
- 内容寻址 digest 一律 **RFC 8785（JCS）规范化**后再 sha256，跨语言必须一致。
- mock 商家数据集版本化落仓（IFACE-4），禁止运行时从网络抓取。

## 语言栈约束

| 区域   | 语言/栈                 | 硬约束                                                       |
| ------ | ----------------------- | ------------------------------------------------------------ |
| 控制面 | Go（modular monolith）  | 评估编排纯函数；一切非确定性显式落盘为内容寻址 artifact      |
| 算子层 | Python（无状态 CLI）    | 只走 C2：JSON stdin/stdout + 文件路径，不连库、不知租户      |
| 渲染层 | TS                      | 确定性渲染（R-1 同输入同版本恒同输出）；信息层叠加 bit-exact |
| 评审   | 布尔探针（vlm_boolean） | 判定=断言（bool+证据），禁止打分做门禁（INV-2）              |

## 依赖规则

新增依赖前先列出"依赖名 / 用途 / 许可证 / 是否能用标准库替代"等人确认；
禁止引入 AGPL / GPL-3.0 / SSPL 的库。Python 侧新依赖仅限算子镜像内。
规则引擎在 `.dependency-cruiser.cjs`，新模块落地时必须同步补全其中的 TODO 边界规则。
