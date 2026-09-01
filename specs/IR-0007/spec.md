---
taskId: IR-0007
specVersion: 1
title: 生成一等公民的营销视频实验平台——V100 实测认知（退役重构 + 实验骨架 + 出片率实测）
irRef: "Cloudbird-Software/Shorts_Director#107"
acceptanceCriteria:
  - id: AC-1
    given: 一台装配 NVIDIA GPU（V100-32G）、驱动、ffmpeg 与 docker 的实验机
    when: 运行环境探测入口并完成探测
    then: 输出机器可读的 capability profile——GPU 算力版本、显存、驱动与 CUDA 版本、ffmpeg、docker 逐项在列，候选生成模型逐项给出 feasible 或 infeasible 判定与原因，且该 profile 以内容寻址 artifact 落盘可回查
  - id: AC-2
    given: 退役工作卡已合并
    when: 检查仓库内容与流水线
    then: 谓词检索、时序求解、商家访谈内核、实拍镜头实体治理、前向兼容层五类模块及其关联 schema 样本与测试从代码库消失，退役前既有测试与 fixture 已归档到可恢复的 git tag，且仓库检查命令（make check）保持全绿
  - id: AC-3
    given: 一张种子静态图与一条含 seed 的图生视频生成请求
    when: 生成算子按算子契约执行该请求
    then: 返回契约四态响应——成功态携带视频产物路径、内容哈希、耗时与 GPU 资源计量及模型版本清单；失败态携带可执行的错误信息；同一输入与同一 seed 重放时产物内容哈希一致或差异来源被显式记录
  - id: AC-4
    given: 一个镜头制式评估套件定义（制式、模型、seed 集、断言包、预算上限）
    when: 评估编排入口执行该套件
    then: 产出内容寻址 run artifact——逐条生成物的 pass 或 fail 判定与证据 URI、聚合出片率（K 次抽卡至少 1 条可用的比例）、耗时与资源计量，全部由该 artifact 自身可复算
  - id: AC-5
    given: 形态1（图生视频氛围展示）与形态4（数字人口播）两套制式套件已在实验机执行完成
    when: 查看两份评估报告
    then: 每份报告含逐条判定明细、聚合出片率、单条平均耗时与资源计量，出片率数字与 run artifact 明细一致，且报告显式标注实验机的 capability profile 引用
  - id: AC-6
    given: 不少于 3 个 mock 商家各自的种子图与信息表（公开图库或合成数据）
    when: 形态1 端到端管线对每个商家执行
    then: 每个 mock 商家产出至少 1 条完整营销视频——时长落在制式区间、信息层含店名/招牌项/价格/地址/电话且与信息表逐字一致、携带 AIGC 标识、并通过该制式断言包
  - id: AC-7
    given: mock 商家人像照与含品牌名、卖点、行动号召三要素的口播文案
    when: 形态4 端到端管线执行（语音合成、口型同步、信息层叠加）
    then: 产出数字人口播视频——时长不超制式上限、口播内容经转写后三要素齐全、口型同步通过断言、携带 AIGC 标识与至少品牌名信息层
  - id: AC-8
    given: 一份约百条生成物的人工可用性标注集（公开或合成素材的生成产物）
    when: 评审探针判定结果与人工标注执行比对
    then: 产出混淆矩阵与一致率报告，且该报告结论登记进假设看板对应条目
  - id: AC-9
    given: 实验战役（环境探测、单发冒烟、两制式出片率、信息层零失败、端到端切片、裁判校准、产能外推）全部执行完毕
    when: 查看仓库 README
    then: 原 Freeze Gate 看板被假设看板替换——每条假设登记状态、证据链接与结论，结论可回查对应 run artifact 或报告
  - id: AC-10
    given: 月租机器含电价的成本前提
    when: 产能与成本结论在报告中表述
    then: 以实测耗时与资源计量外推日产能区间并显式标注为估算，报告不引入电价变量，且给出向 A100 对照实验迁移的口径说明
  - id: AC-11
    given: 本 spec 与套件已合并
    when: 红队审计运行并产出判定
    then: 存在 verdict 为 survived 的审计记录，且后续工作卡的验收标准逐条派生自本 spec 的 AC-1 至 AC-10
nonGoals:
  - 不做商家输入端（小程序/表单）、不做数据库/队列/对象存储/编排调度平台、不做多租户
  - 不做 6 形态全覆盖——只实测形态1 与形态4（光谱两端），其余 4 形态留待数据配置
  - 不做素材打标与访谈的 LLM 运行时（服务于旧意图的素材治理）
  - 不在 V100 阶段优化产能/成本到业务文档目标（日 60-100 条、单条 5 元以内）——本阶段只建立测量仪器并产出第一批实测数字；A100 对照实验一次性完成
  - 不引入商业渲染订阅、剪辑时间线交换格式、微服务拆分
blastRadius:
  - "Cloudbird-Software/Shorts_Director: specs/IR-0007/**"
  - "Cloudbird-Software/Shorts_Director: internal/**（退役五类模块；新增评估编排与实验骨架）"
  - "Cloudbird-Software/Shorts_Director: operators/**（Python 生成/评审算子）"
  - "Cloudbird-Software/Shorts_Director: cmd/**（探测与评估入口）"
  - "Cloudbird-Software/Shorts_Director: schema/**（受控词表与样本随退役/新增同步）"
  - "Cloudbird-Software/Shorts_Director: evals/**（套件定义与 mock 商家数据集）"
  - "Cloudbird-Software/Shorts_Director: README.md（假设看板替换 Freeze Gate 看板）"
---

# IR-0007 生成一等公民的营销视频实验平台（条款级规格）

## INV 不变量

- INV-1: 生成是一等公民——仓库的一切控制面、契约与数据结构必须服务于「以生成能力反推视频形态」的范式；任何把实拍素材库治理（谓词检索、镜头生命周期、月度排期）置于生成之上的结构都不得回流。
- INV-2: 评估判定题化——出片率、可用性、口型同步等一切质量判断必须表达为断言（probe → 期望 → bool + 证据），禁止以 1-10 打分或自由文本评价作为门禁；报告中的聚合数字必须能从 run artifact 明细复算。
- INV-3: 非确定性显式落盘——模型推理、seed、评审判定等一切非确定性产物必须落为内容寻址 artifact（哈希覆盖输入、模型版本与 seed），不落盘的产物不得进入任何报告。
- INV-4: 生成算子走既有算子契约——stdin/stdout JSON、四态语义、确定性字段与资源计量字段一个不少；算子无状态、不知租户、不连库。
- INV-5: 信息层零失败——确定性信息（文字/价格/地址/电话/Logo）只能经模板叠加进入成品，禁止交给生成模型产生；叠加层的渲染必须是确定性的（同输入同版本恒同输出）。
- INV-6: 合规底线——一切成品视频必须携带 AIGC 标识；实验数据只用公开数据集或合成数据，不得使用真实商家或真实个人的身份信息与生物特征。
- INV-7: 退役不可悄悄发生——退役模块必须先归档（golden/测试落 git tag）再删除，退役后仓库检查命令必须保持全绿；「先抓 golden 再删」是不可跳过的顺序。

## BEH 行为

- BEH-1: 当实验机上运行环境探测入口时，系统必须探测 GPU 算力版本/显存/驱动/CUDA、ffmpeg、docker，并对候选生成模型逐项给出 feasible 或 infeasible 判定与原因，产出内容寻址的 capability profile。
- BEH-2: 当一条含 seed 的图生视频请求进入生成算子时，算子必须按四态契约响应，成功态必须携带产物路径、内容哈希、耗时与 GPU 资源计量及模型版本清单。
- BEH-3: 当一个制式套件（制式×模型×seed 集×断言包×预算上限）交给评估编排入口时，系统必须逐条执行生成、逐条跑断言、聚合出片率（K 次抽卡至少 1 条可用的比例），并落盘内容寻址 run artifact。
- BEH-4: 当评估报告生成时，报告必须含逐条判定明细、聚合出片率、耗时与资源计量，且引用实验机的 capability profile。
- BEH-5: 当 mock 商家的种子图与信息表进入形态1 端到端管线时，系统必须产出时长在制式区间内、信息层与信息表逐字一致、携带 AIGC 标识并通过断言包的成品视频。
- BEH-6: 当人像照与口播文案进入形态4 管线时，系统必须经语音合成与口型同步产出数字人口播视频，且口播内容可经转写验证三要素齐全。
- BEH-7: 当评审探针对生成物可用性做出判定时，系统必须支持与人工标注集比对并产出混淆矩阵与一致率。
- BEH-8: 当实验战役任何一环产出结论时，该结论必须登记进 README 假设看板（状态/证据链接/结论），证据必须回查到 run artifact 或报告。
- BEH-9: 当退役卡执行时，系统必须先归档既有测试与 fixture 到 git tag 再删除模块，并保持检查命令全绿。

## IFACE 契约

- IFACE-1: 生成算子 op 命名必须收敛为受控枚举——首批：图生视频、语音合成、口型同步、布尔评审四类（布尔评审沿用既有 QC 断言白名单中的评审探针 vlm_boolean 语义）；op 名以受控清单落盘于仓库内，新增 op 必须走清单变更而非散落命名。
- IFACE-2: capability profile 与 run artifact 必须是机器可解析的结构化 JSON，携带各自的内容寻址哈希；run artifact 必须内嵌套件定义全文与执行环境签名（capability profile 引用），保证不依赖外部状态即可复算。
- IFACE-3: 镜头制式（形态）必须以受控词表落盘（enum 只允许追加），每形态钉死时长区间、画幅（统一 1080x1920，9:16）、信息层要素清单与断言包绑定；形态=数据而非代码分支。
- IFACE-4: mock 商家数据集必须以版本化目录存在于仓库内（种子图来源清单 + 信息表 + 授权占位记录），禁止运行时从网络抓取。
- IFACE-5: 出片率口径全系统唯一——「可用」= 通过该制式断言包全部断言；聚合口径 = K 次抽卡中至少 1 条可用的条目比例；任何报告不得使用第二套口径。

## BUDGET 预算

- BUDGET-1: 实现卡按一个 PR 一件事且 diff 低于 400 行的纪律逐卡拆分落地。
- BUDGET-2: 单条生成物评估的单断言失败重试不超过 1 次（仅限基础设施类瞬时故障），超限即记该条为 fail 并保留证据。
- BUDGET-3: 套件执行的预算上限（wall time 与 GPU 秒数）由套件定义钉死，超限即中止并将已完成条目的部分结果落盘标注「预算截断」。

## DECISION 决策

- DECISION-1: 重构采用「退役优先」顺序——先归档并删除旧意图模块，再落实验骨架，避免新旧范式并存造成结构歧义；退役范围以 IR-0007 清单为准。
- DECISION-2: 评估仪器自建而非引入外部 eval 框架——组织测试政策 L-01 的 eval_harness 语义由本仓评估编排承载，断言复用既有 QC 断言引擎，golden 机制复用既有算子 golden 纪律。
- DECISION-3: 首批实测模型池以公开开源、可本地推理为准入（候选由环境探测与单发冒烟裁决排序，不在 spec 内预置排名）；模型接入必须可插拔（换模型不改算子契约）。
- DECISION-4: 语音与口型同步走成熟开源链路；口型同步的可用性以 SyncNet 类探针指标为判定锚，不引入人工逐帧评审为门禁。
- DECISION-5: A100 对照实验的承接机制——套件定义硬件无关，run artifact 携带环境签名，A100 到位后同套件全量重跑并自动产出双机 diff 报告；V100 阶段不为 A100 预做任何代码特化。
- DECISION-6: 成本口径——月租含电价，产能与成本结论只以实测耗时与资源计量外推，显式标注估算属性。

## ASSUMPTION 假设

- ASSUMPTION-1: V100-32G（sm_70，fp16 可用，无 bf16 与 FlashAttention-2）上至少存在一个可行开源图生视频模型——若环境探测与冒烟全部 infeasible，则形态1 出片率实验降级为「记录不可行证据 + 上浮 A100 前置条件」的结论，不构成 spec 违约。
- ASSUMPTION-2: 公开图库与合成数据可以构造出足够代表性的 mock 商家场景（餐饮/美业等）；代表性不足导致的出片率偏差在报告中显式标注，不构成 spec 违约。
- ASSUMPTION-3: 评审探针（布尔判定）与人工判定的一致率存在可校准空间；一致率不足时结论为「仪器不可信，出片率数字失真」，仍属有效实验结论。
- ASSUMPTION-4: 开源 TTS 与口型同步链路在 6 秒以内短时长下的可用性满足断言阈值；不满足时按实验结论记录，不构成 spec 违约。

## 测试设计（逐类讨论，testing.yaml 逐项过堂）

**风险映射总纲**：本 spec 三类风险敞口——customer_upgrade_failure 体现为「实验平台不可复现/结论不可信」（run artifact 复算失败、非确定性未落盘）；llm_behavior_drift 体现为「生成与评审质量漂移」（换模型/换 seed 后断言口径漂移、评审探针失准）；fake_tests 体现为「评估摆拍」（断言永真、出片率数字与明细脱钩、golden 与实现共谋）。

**active_now（每 PR gate）：**
- T-01 unit_property_golden：**adopt**。Go 控制面（评估编排、出片率聚合、capability profile 解析）单测 + golden；property test 覆盖聚合口径（任意判定明细序列 → 出片率可复算）。理由：fake_tests 主防线。
- T-02 race_detection：**adopt**（applies: go）。评估编排并发跑多条生成/断言时必须 -race 干净。理由：并发聚合是出片率数字的来源，data race 会直接污染结论。
- T-03 goroutine_leak：**reject**。本平台为 CLI 一次性执行形态，无长驻进程；进程退出即回收。理由：触发条件 long_running_process 不满足。
- T-04 fuzz_seed：**adopt**（parser/protocol code）。算子契约 JSON 解析、capability profile 解析入 fuzz 面（gate 短跑 + 周深跑）。
- T-05 doc_examples：**reject**。本 spec 交付物是 CLI 入口与实验报告，无库 API 消费者；输出结构以 T-01 golden 覆盖。理由：无公开函数签名可做 Example。
- T-06 license_scan：**adopt**（hygiene 既有 gitleaks/依赖审查）。Python 算子镜像的新依赖入仓报批时许可证扫描前置。
- T-07 sbom：**reject**。实验平台阶段无 release 附件分发；A100 对照实验前再评估。理由：无 release 触发点。
- T-08 flaky_governance：**adopt**。GPU 算子冒烟类测试可能 flaky，入 flaky 台账并按组织规则重试。
- T-09 differential：**adopt（本 IR 为重写项目，gate 必选）**。退役前抓旧模块 golden（归档 tag），保留面（算子契约/QC 断言/JCS digest）新旧实现对同一 fixture 集回放对比；评估口径变更（断言包调整）时旧 run artifact 重放 diff。
- T-10 mutation：**adopt（weekly）**。评估聚合与出片率计算模块为变异测试重点（score<60% 即 fake_tests 候选）。
- T-11 governance_canary：**reject**。org 级既有 canary 覆盖，本仓无新增治理面。理由：非治理仓变更。
- T-12 diff_coverage：**adopt**（gate 既有 80% diff 口径）。
- T-13 test_integrity：**adopt**（gate 既有，测试篡改四形态拦截）。
- T-14 card_bound_test_required：**adopt**。本 spec 的 suite/ 即卡绑定测试集锚点；实现 PR 走卡对应测试集与已注册 holdout。
- T-15 intent_backstop：**adopt**（gate 既有红队顺带意图回探）。

**on_llm_product（条件激活——本 IR 主体即 LLM/GPU 产物评估）：**
- L-01 eval_harness：**adopt（本 spec 的核心交付物之一）**。gate 跑小回归集（FakeRunner 化的套件回放），weekly/实验机跑全量真模型。
- L-02 semantic_golden：**adopt**。评审探针对锚定样本集的判定以语义锚（判定+证据结构）固化为 golden，禁精确文本 diff。
- L-03 metamorphic：**adopt**。不变关系三条——同 seed 重放产物哈希一致（或差异显式记录）；seed 顺序重排不改变聚合出片率；信息层叠加双跑字节一致（bitexact）。
- L-04 adversarial_corpus：**adopt**。裁判校准集内含对抗样本（崩坏生成物、模糊图、主体畸变），专攻评审探针漏判。
- L-05 cost_latency_budget：**adopt**。单条生成耗时/GPU 秒数进 run artifact，套件级预算截断（BUDGET-3）触发记录。
- L-06 model_upgrade_differential：**adopt**。换模型=换实现——同套件在新旧模型下各跑一遍，产出双模型 diff 报告（这正是 E2 实验设计本身）。

**on_rewrite_project（本 IR 含退役重构，激活）：**
- R-01 upgrade_path / R-02 rollback / R-03 migration_idempotent / R-04 config_compatibility / R-05 fresh_install_smoke：**reject**。无线上客户、无旧版本升级路径、无生产部署——退役面向 git 历史而非运行时系统。理由：触发条件（真机升级/回滚/迁移）不成立。
- R-06 capture_golden_now：**adopt（严格执行，不可逆操作）**。退役 PR 前抓全部将被删除模块的测试与 fixture 归档 tag——本 spec AC-2 的直接来源。

**triggered（触发式）：**
- G-01 contract_api_drift：**reject**。无对外公网 API。
- G-02 integration_real_deps：**adopt（弱）**。算子-控制面集成以真实子进程握手测试（真 ffmpeg/真算子二进制）在实验机跑，gate 用 FakeRunner。理由：sql_layer_landed 不满足，但子进程契约面同类风险成立。
- G-03 bench_regression：**adopt（weekly）**。数据密集路径（评估编排、聚合）基准趋势跟踪。
- G-04 dast / G-05 iac_scan：**reject**。无对外服务栈、无 IaC。
- G-06 load_test：**reject**。无生产性能投诉面（实验平台）。
- G-07/G-08（若有）：按同口径 reject——无对外攻击面。

**rejected 清单（X-01..X-06）**：不启用任何一项（全局覆盖率门槛等）——与组织政策一致，无豁免诉求。

## holdout 测试设计（封存验收场景）

- **HO-0009@18eda58f**：形态1（图生视频氛围展示）端到端验收场景——mock 商家输入规格 + 六条机器可判定验收断言（时长区间/信息层五要素逐字一致/AIGC 标识/画幅/断言包/复算性）。verdict 阶段揭封执行，防实现向已知验收过拟合。
- **HO-0010@123d89e4**：形态4（数字人口播）端到端验收场景——人像+口播三要素输入规格 + 六条断言（时长上限/转写三要素/口型同步/AIGC 标识/信息层/复算性）。
- 注册路径：holdout 仓 scripts/new_entry.py（e2e-scenario 类型，sealed_by=owner），条目已通过 validate_entries 全绿（见 holdout 仓 PR #10）。
- 引用纪律：spec/卡/PR 只引用 id@sha8；payload 内容不出现在本仓任何文件；揭封只在 verdict 阶段经 unseal gate 执行，PR check 只显示计数。
- holdout 失败处置：修实现不修试卷（ADR-0095）；quarantine 路径见 ROLE-IMPLEMENT。
