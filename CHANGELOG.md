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

### Removed
- 模板占位：src/index.ts 的 greet 与 tests/smoke.test.ts。

### Changed
- 仓库定位从 template-service 模板切换为 Shorts_Director（README 重写、包名更新）。
- 工程设计文档移至 [docs/Engineering_plan.md](docs/Engineering_plan.md)。
- docs/ARCHITECTURE.md 对齐工程设计：四层分层心智模型、仓库布局、契约纪律与语言栈硬约束。
