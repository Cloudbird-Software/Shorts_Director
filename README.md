# Shorts_Director

**生成一等公民的营销视频实验平台**（IR-0007）：不以实拍视频标准要求 AI，
而以 AI 稳定生成能力为一等公民反推视频形态；确定性信息（文字/价格/地址/
电话/Logo）经模板叠加实现零失败。当前以一块 V100-32G（月租含电价）为
实验资源，通过实测推进对可行性的认知；后续 A100 2×40GB 到位后一次性
完成对照实验。

架构纪律见 [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)；旧意图
（实拍素材治理型生产系统）的完整工程设计已归档至
[docs/history/Engineering_plan.md](docs/history/Engineering_plan.md)。

## 核心范式（不变式摘录，全文见 specs/IR-0007/spec.md）

- **生成是一等公民**——一切控制面、契约与数据结构服务于「以生成能力反推视频形态」。
- **评估判定题化**——出片率等一切质量判断表达为断言（probe → 期望 → bool + 证据），禁止打分做门禁。
- **非确定性显式落盘**——模型推理、seed、评审判定一律落为内容寻址 artifact，不落盘的产物不进报告。
- **信息层零失败**——确定性信息只经模板叠加进入成品，禁止交给生成模型产生。
- **合规底线**——一切成品携带 AIGC 标识；实验数据只用公开或合成数据。

## 假设看板（Assumption Board，替代原 Freeze Gate 看板）

实验战役 E0–E7 的每条假设登记状态/证据链接/结论；结论必须可回查对应
run artifact 或报告（BEH-8）。实验未执行前一律 `pending`；状态取值
`pending / running / confirmed / refuted / inconclusive`。

| 实验            | 假设 / 问题                                                                                | 状态    | 证据链接 | 结论 |
| --------------- | ------------------------------------------------------------------------------------------ | ------- | -------- | ---- |
| E0 环境探测     | V100-32G（sm_70，fp16 可用，无 bf16/FA2）上候选开源生成模型逐项 feasible/infeasible 与原因 | pending | —        | —    |
| E1 单发冒烟     | 可行模型能产出 5–8s 图生视频片段并计量 latency/peak_mem                                    | pending | —        | —    |
| E2 形态1 出片率 | 形态1（I2V_AMBIENCE）× 模型 × seed[1..5] 的抽卡出片率 P(≥1 条可用)                         | pending | —        | —    |
| E3 形态4 出片率 | 形态4（DIGITAL_HUMAN）全链路（TTS+口型同步+信息层）出片率                                  | pending | —        | —    |
| E4 信息层零失败 | 确定性信息模板叠加与信息表逐字一致、同输入双跑字节一致                                     | pending | —        | —    |
| E5 端到端切片   | ≥3 个 mock 商家 × 形态1/形态4 组合出完整营销视频                                           | pending | —        | —    |
| E6 裁判校准     | vlm_boolean 评审探针 vs 人工标注（约百条）一致率与混淆矩阵                                 | pending | —        | —    |
| E7 产能外推     | 以实测耗时与资源计量外推日产能区间（显式标注估算，不引入电价变量）                         | pending | —        | —    |

出片率口径全系统唯一（IFACE-5）：「可用」= 通过该制式断言包全部断言；
聚合口径 = K 次抽卡中至少 1 条可用的条目比例。

## 仓库布局

```
schema/            唯一真源：受控词表（含 gen_form 形态词表）/ 实体契约 / C2、C3 契约
internal/          Go 控制面（videoplan / qc / compliance / compiler / renderer / operator / eval）
operators/         Python 无状态生成/评审算子（C2 契约：stdin/stdout JSON、四态语义）
cmd/               CLI 入口（shorts-doctor 环境探测 / shorts-eval 评估编排 / shorts-render / shorts-operator）
evals/             制式评估套件定义 + mock 商家数据集（公开/合成数据）
specs/             IR 与条款级规格（specs/IR-0007/spec.md）
```

## Makefile 接口（所有语言统一，CI 只认这个）

| 目标                           | 作用                                                                                     |
| ------------------------------ | ---------------------------------------------------------------------------------------- |
| `make setup`                   | 安装依赖（`npm ci`）                                                                     |
| `make gen`                     | 词表 codegen（schema/vocab → codegen/）                                                  |
| `make check`                   | lint + test，**提交前必须全绿**                                                          |
| `make go-check`                | Go 控制面执法（gofmt + vet + test）                                                      |
| `make doctor`                  | 环境探测：GPU/ffmpeg/docker + 候选模型可行性，落盘内容寻址 capability profile（卡 #113） |
| `make eval-suite SUITE=<形态>` | 制式评估（规划中，卡 #115）                                                              |

## CI 结构

- `hygiene`：密钥扫描（gitleaks）、大文件/凭据文件拦截、zizmor Actions 审计
- `check`：`make setup && make check`；`go-check`：Go 控制面执法
- `contract`：契约兼容性检测门（breaking 须 ADR 留痕，ADR-0038/0105）
- `deps`：依赖漏洞 + 许可证审查（PR 时）
- `gate`：聚合门（组织 ruleset 的唯一必需 check）

工作流实现在 [CI-Workflows](https://github.com/Cloudbird-Software/CI-Workflows)，本仓只引用钉扎 SHA。
