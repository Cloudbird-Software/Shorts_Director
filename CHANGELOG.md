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

### Removed
- 模板占位：src/index.ts 的 greet 与 tests/smoke.test.ts。

### Changed
- 仓库定位从 template-service 模板切换为 Shorts_Director（README 重写、包名更新）。
- 工程设计文档移至 [docs/Engineering_plan.md](docs/Engineering_plan.md)。
- docs/ARCHITECTURE.md 对齐工程设计：四层分层心智模型、仓库布局、契约纪律与语言栈硬约束。
