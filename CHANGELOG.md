# Changelog

本文件记录对外可见的变更。格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)。

## [Unreleased]

### Added
- 初始模板工程（CI gate / hygiene / dependabot / automerge 全套护栏）。
- schema/ 单一真源目录与模块规约（schema/AGENTS.md）。
- 受控词表 v1 第一批（结构类）：beat_role（冻结）、audio_role、shot_type、camera_motion、proof_type。
- 受控词表 v1 第二批（语义类·餐饮垂类）：scene.food（14 值）、subject.food（24 值）。
- 受控词表 v1：action（38 值，跨垂类通用、餐饮校准，等价类 PREP/COOK/PRESENT/SERVICE/SUPPLY/REACTION/SOCIAL）。
- 受控词表 v1：copy_function（18 值）、overlay_intent（14 值，含合规强制的 AIGC_DISCLOSURE）。
- 受控词表 v1：defect_type（39 值，按 L0/L1/L2/L3 分组）、remedy_action（14 值）。
- 受控词表 v1 收尾：compliance_risk（16 值）、season（6 值）、ttl_class（4 值）。14 张词表全部冻结。
- schema 公共块：provenance（溯源）/ versioned_ref（版本化引用）/ licensed_ref（授权引用）。
- ShotSlotQuery v1：谓词 AST（字段白名单 + 受控操作符 + 降级链 + 消耗策略），IV-SQ-1/SQ-2。
- VideoPlan IR v1：整数帧 timebase、content_hash 版本钉死、constraints_report、diversity_signature、预算与 AIGC 双轨标识位（IV-VP-1..5）。
- Asset v1 / Shot v1：五组元数据（identity/semantic/affordance/technical/compliance）+ 生命周期，状态机与 IV-SH-1..3。
- BrandKernel v1：根契约（IV-BK-1..3），事实字段带 verified_at 防漂移；Event v1：14 种事件的不可变日志，COPY_EDITED 强制 JSON Patch。
- QCAssertion v1：判定题断言 DSL（29 个 probe 算子 + remedy 返修模板 + applies_when 条件谓词）；ProductionOrder v1：制作令合同界面（intent/spec/验收断言/handles 余量）。
- C2 算子协议 v1（request/response：纯函数 CLI、四态 status、model_versions 回填）；C3 渲染契约 v1（R-1 确定性 / R-2 帧数守恒 / R-3 安全区拒绝 / R-4 无隐式回退 / R-5 版本含字体哈希）。
- src/contracts 契约常量模块（schema 版本/词表清单/冻结枚举/工艺参数），替换模板占位代码；补全 depcruise 边界规则。
- codegen 流水线（`make gen`）：schema/vocab/v1/*.yaml → codegen/ts/vocab.ts（词表清单 + 全部枚举值与类型，prettier 确定性输出）；CI 新鲜度测试守住"改 schema 必须重生成"；src/contracts 词表锚点切换为生成代码再导出。
- 词表元数据生成：zh/def/等价类/废弃链全量进 codegen，附 VOCAB_IDS/VOCAB_META 注册表；src/contracts/vocab.ts 运行期助手（isVocabId/assertVocabId/isDeprecated/replacedBy/equivalenceClassOf/zhOf）——ShotSlotQuery 按类取材与 QC 断言的公共底座；生成器新增废弃值必须 replaced_by 同表合法 id 的结构断言。
- Freeze Gate G1 校验 harness（ajv 2020-12 + ajv-formats）：schema/testdata/<entity>/{valid,invalid} 自动纳管，valid 全过 / invalid 全拒 / 规模门槛（≥5+≥15）进 CI；Asset 首批样本 6 valid + 19 invalid，content_hash 钉死 pattern `^sha256:[0-9a-f]{64}$`。
- Shot G1 样本：5 valid（minimal/typical 打标态/available 全量/pillarbox 声明/单帧边界）+ 25 invalid（必填缺失×11、状态机枚举、fps 枚举、in/out_frame 边界、camera_motion_dir、subjects 超 maxItems=5、safe_crop 缺 ok、quality_tier 越界、provenance 结构×3、additionalProperties×2）。
- Event G1 样本：5 valid（四类 actor × 五种 kind 含 COPY_EDITED 的 RFC 6902 Patch 与 PLAN_REJECTED 的 reason_code）+ 15 invalid（必填缺失×6、kind/actor 枚举、uuid/date-time 格式、payload 类型、additionalProperties）。
- BrandKernel G1 样本：5 valid（minimal/typical 全字段带溯源与 human_edits/high_completeness/6 支柱边界/digital_human 真人授权）+ 23 invalid（必填缺失×9、schema_version const、pillars 与 proof_types 下限、differentiators/segments 下限、one_liner 长度、target_ratio/completeness/interview_turns 范围、persona/decision_trigger/digital_human.source 枚举、additionalProperties）；schema 修正 digital_human.source 允许 null（enabled=false 时）。
- ShotSlotQuery G1 样本：5 valid（等价类谓词/复合 and·or·not/数值区间 between·lte·gt/终端图形兜底/空 must 兜底）+ 21 invalid（字段白名单外、op 非受控、数值 op 传字符串、between 三元与非数、semantic top_k 越界、逻辑空 operands、fallback 结构×4、consumption 边界、should 权重越界等）。
- QCAssertion G1 样本：5 valid（L0 黑帧/L1 条件断言 applies_when/L2 口型 between/L3 AIGC 标识 BLOCKER/违禁词 contains_none + 采样策略）+ 17 invalid（必填缺失×6、level/severity/probe op/expect op/sampling 枚举、remedy 缺模板、applies_when 字段白名单外）。
- ProductionOrder G1 样本：5 valid（minimal/摄影实拍全量/生成供应商/商家自拍/返修条款）+ 20 invalid（必填缺失×8、kind/vendor_type 枚举、duration_sec 二元组、framing headroom、min_resolution 常量、auto_gate_level、budget 负值、deadline 格式）。
- VideoPlan G1 样本：5 valid（minimal 音乐计划/口播加变速/纯生成素材/商用音乐带凭证/静音模式）+ 29 invalid（必填缺失×14、canvas w 常量与 fps 枚举、timebase 秒制、clip speed 上限与 src_out 下限、track kind、overlay anchor 与帧区间、LicensedRef 凭证、VersionedRef 版本、caption 长度）；valid 样本 diversity_signature 与实际 tracks/audio 对齐。
- Freeze Gate G3（跨语言 JCS 一致）：src/digest TS 侧 RFC 8785 实现（零依赖：String(n)=ES 规范数字、默认 sort=UTF-16 码元序、JSON.stringify 转义），附 contentDigestJsonText；共享向量集 testdata/digest/jcs_vectors.json（RFC 锚点值 + 转义/UTF-16 排序/C2 请求形态）由 Go 与 TS 双侧对照 canonical 与 sha256，逐字节一致。
- C2/C3 契约 G1 样本 + harness 纳管 contracts：operator request/response（5+17 / 5+21）、render request/response（5+22 / 5+22，request 内嵌合法 VideoPlan 且 resolved_media 全量预解析）；G1 harness 改为按 $id 递归发现（支持嵌套 key），新增孤儿样本目录守卫（拼错/$id 不匹配即测试失败，禁止静默跳过）。
- Go 控制面骨架（#27）：go.mod 工作区、Makefile test 接入、契约版本锚点包 internal/contracts（C2/C3 版本常量与词表清单单一来源）。
- digest 包（#29）：RFC 8785 JCS 规范化与内容寻址摘要——跨语言（Go/TS/BAML）digest 一致性的 Go 侧实现。
- slotquery 包（#30/#31）：ShotSlotQuery 谓词 AST 的 Validate/内存求值/字段求值，IV-SQ 校验；降级链取材 Resolve（候选池过滤 → 消耗策略 → 逐级放宽）与排序。
- entity 包（#30 附带）：Shot 实体生命周期状态机与 IV-SH-1..3 跨字段校验。
- planner 包（#32）：PlanDay 阶段 B 确定性时长求解与节拍吸附（整数帧，禁 float seconds）。
- videoplan 包（#33）：VideoPlan IR v1 Go 实体层（结构体与 schema 一一对应，round-trip 防漂移）+ IV-VP-1..4 校验。
- qc 包（#34）：断言引擎——applies_when 过滤、probe 分组去重、成本排序、BLOCKER 短路、remedy 模板渲染（变量缺失显式报错）。
- operator 包（#35/#36/#37）：C2 契约执行器 Local/Docker/Fake 三实现（stdin/stdout JSON、结构化错误、golden 驱动）；shorts-operator CLI 与 probe 算子（ffprobe 元信息提取）；RunnerProbeAdapter 桥接 qc↔C2。
- compiler 包（#38）：VideoPlan → C3 RenderRequest 编译器——媒体引用预解析（R-4 无隐式回退：缺媒体/hash 漂移即报错）、字体完备性校验、确定性输出（排序后逐字节稳定）；Validate 镜像 C3 request schema，5 个契约 valid 样本驱动防漂移。
- compliance 包（#39）：S8 ComplianceGate 八门串行链（类目准入/违禁词/必需声明/三方权利/音乐授权/声音授权/AIGC 标识/肖像授权），任一 BLOCKER 即停、REVIEW 压低整体结论；外部事实全量注入（可重放审计）；AIGC 双轨——显式 overlay 强制注入（幂等/安全区/post_text 追加）+ 隐式元数据字段映射版本化；videoplan 增加 compliance 回写块与 IV-VP-5。
- BAML C1 契约（#40）：gen-vocab 新增 BAML 渲染（17 张词表 → baml_src/vocab.baml，AUTO-GENERATED + 新鲜度入 CI）；TagShot（B-2 确定性字段作可信输入 / B-3 uncertain_fields 强制）与 NextInterviewQuestion（苏格拉底追问 + 停止条件）各 6 test block 含 2 对抗样本；Go 结构守卫（B-1 类型可解析 / B-4 测试规模 / 素材引用存在性）；新增词表 mood（8 值）/negative_space（5 值）。

### Removed
- 模板占位：src/index.ts 的 greet 与 tests/smoke.test.ts。

### Changed
- 仓库定位从 template-service 模板切换为 Shorts_Director（README 重写、包名更新）。
- 工程设计文档移至 [docs/Engineering_plan.md](docs/Engineering_plan.md)。
- docs/ARCHITECTURE.md 对齐工程设计：四层分层心智模型、仓库布局、契约纪律与语言栈硬约束。
