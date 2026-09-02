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

| 实验            | 假设 / 问题                                                                                | 状态      | 证据链接                                                                                                                  | 结论                                                                                                                                                                   |
| --------------- | ------------------------------------------------------------------------------------------ | --------- | ------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| E0 环境探测     | V100-32G（sm_70，fp16 可用，无 bf16/FA2）上候选开源生成模型逐项 feasible/infeasible 与原因 | running   | [doctor profile](evals/runs/2026-09-02/doctor/6068867f40e271919e287f71c8949b3495f260c887d3188b88cb05ed6d43dea6.json)      | 探测仪器就绪并产出内容寻址 profile；开发沙箱无 GPU（profile 如实登记 infeasible）；V100 逐项可行性判定待实验机执行 `make doctor`                                       |
| E1 单发冒烟     | 可行模型能产出 5–8s 图生视频片段并计量 latency/peak_mem                                    | running   | [form1 run](evals/runs/2026-09-02/form1/fe2b36b0e23f511c94b72fd202826442197c42114f0cbc4aa03bb4dd4b4abb9d.json)            | 管道冒烟通过（fake 后端 5–8s 片段 + wall/mem 计量落盘）；真实模型 latency/peak_mem 待 V100                                                                             |
| E2 形态1 出片率 | 形态1（I2V_AMBIENCE）× 模型 × seed[1..5] 的抽卡出片率 P(≥1 条可用)                         | running   | [form1 run](evals/runs/2026-09-02/form1/fe2b36b0e23f511c94b72fd202826442197c42114f0cbc4aa03bb4dd4b4abb9d.json)            | 仪器全链路就绪：3 商家 × 5 seed = 15/15 可用（fake 口径，验证断言与聚合）；真实模型抽卡出片率待 V100 同套件重跑                                                        |
| E3 形态4 出片率 | 形态4（DIGITAL_HUMAN）全链路（TTS+口型同步+信息层）出片率                                  | running   | [form4 run](evals/runs/2026-09-02/form4/5f5997f7b550622af76d6d0c5ac4f80e6b19963730610df2b35b4574faeef4c7.json)            | 全链路（gen_tts→gen_lipsync→transcribe 三要素→渲染→断言）fake 演练 3/3 商家出片；真实后端出片率待 V100                                                                 |
| E4 信息层零失败 | 确定性信息模板叠加与信息表逐字一致、同输入双跑字节一致                                     | confirmed | [form1 run](evals/runs/2026-09-02/form1/fe2b36b0e23f511c94b72fd202826442197c42114f0cbc4aa03bb4dd4b4abb9d.json)            | 信息层是确定性模板叠加（不经生成模型）：15 条 fake 渲染逐字一致断言全过；`TestRenderDeterministic`/`TestForm1Deterministic` 执法双跑字节一致——与后端无关，判 confirmed |
| E5 端到端切片   | ≥3 个 mock 商家 × 形态1/形态4 组合出完整营销视频                                           | running   | [report](evals/runs/2026-09-02/report/fb79b5a04b96dfe7ad66eadec96905e237dea80ea31abaf61e32991b4e3162e1.json)              | 3 个 mock 商家 × 两形态 fake 口径 6/6 条目出完整营销视频（含 AIGC 双轨）；真实生成口径待 V100                                                                          |
| E6 裁判校准     | vlm_boolean 评审探针 vs 人工标注（约百条）一致率与混淆矩阵                                 | running   | [calibrate report](evals/runs/2026-09-02/calibrate/be6c9c44ba6e6695432a6e27aeb8fba573218bf816643a99d051056ddf57be2c.json) | fake 负对照完成：100 条判定、零探针错误、一致率 0.59（无语义基线≈随机，符合预期）；qwen-vl 真实校准待 V100                                                             |
| E7 产能外推     | 以实测耗时与资源计量外推日产能区间（显式标注估算，不引入电价变量）                         | running   | [report](evals/runs/2026-09-02/report/fb79b5a04b96dfe7ad66eadec96905e237dea80ea31abaf61e32991b4e3162e1.json)              | 外推仪器就绪（shorts-report：出片率复算执法 + 估算标注 + 无电价变量 + A100 迁移口径）；fake 口径演练区间 4105–6245 条/日（仅仪器演练）；真实外推待 V100 实测           |

> 注：截至 2026-09-02，开发沙箱无 GPU——上表 fake 口径 run 验证的是**仪器**
> （套件→算子→断言→聚合→报告全链路的正确性与确定性），不代表真实模型出片率
> 或产能；V100 实验机以同套件重跑后，本看板各项升级为实测结论（DECISION-5）。
> 证据均为内容寻址 artifact（文件名 = digest hex），出片率可由明细复算（AC-5）。

出片率口径全系统唯一（IFACE-5）：「可用」= 通过该制式断言包全部断言；
聚合口径 = K 次抽卡中至少 1 条可用的条目比例。

## 仓库布局

```
schema/            唯一真源：受控词表（含 gen_form 形态词表）/ 实体契约 / C2、C3 契约
internal/          Go 控制面（videoplan / qc / compliance / compiler / renderer / operator / eval）
operators/         Python 无状态生成/评审算子（C2 契约：stdin/stdout JSON、四态语义）
cmd/               CLI 入口（shorts-doctor 环境探测 / shorts-form1 / shorts-form4 管线 / shorts-eval 评估编排 / shorts-calibrate 裁判校准 / shorts-report 出片率报告 / shorts-render / shorts-operator）
evals/             制式评估套件定义 + mock 商家数据集 + 人工标注集 + runs/ 内容寻址 run artifact（看板证据）
specs/             IR 与条款级规格（specs/IR-0007/spec.md）
```

## Makefile 接口（所有语言统一，CI 只认这个）

| 目标                         | 作用                                                                                     |
| ---------------------------- | ---------------------------------------------------------------------------------------- |
| `make setup`                 | 安装依赖（`npm ci`）                                                                     |
| `make gen`                   | 词表 codegen（schema/vocab → codegen/）                                                  |
| `make check`                 | lint + test，**提交前必须全绿**                                                          |
| `make go-check`              | Go 控制面执法（gofmt + vet + test）                                                      |
| `make doctor`                | 环境探测：GPU/ffmpeg/docker + 候选模型可行性，落盘内容寻址 capability profile（卡 #113） |
| `go run ./cmd/shorts-form1`  | 形态1 端到端管线（gen_i2v→信息层→AIGC 双轨→断言→run artifact）                           |
| `go run ./cmd/shorts-form4`  | 形态4 端到端管线（TTS→口型同步→转写三要素→渲染→断言→run artifact）                       |
| `go run ./cmd/shorts-report` | 出片率实测报告 + 产能外推（聚合 run artifact，复算执法，AC-5/AC-10）                     |

## CI 结构

- `hygiene`：密钥扫描（gitleaks）、大文件/凭据文件拦截、zizmor Actions 审计
- `check`：`make setup && make check`；`go-check`：Go 控制面执法
- `contract`：契约兼容性检测门（breaking 须 ADR 留痕，ADR-0038/0105）
- `deps`：依赖漏洞 + 许可证审查（PR 时）
- `gate`：聚合门（组织 ruleset 的唯一必需 check）

工作流实现在 [CI-Workflows](https://github.com/Cloudbird-Software/CI-Workflows)，本仓只引用钉扎 SHA。
