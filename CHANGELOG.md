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

### Changed
- 仓库定位从 template-service 模板切换为 Shorts_Director（README 重写、包名更新）。
- 工程设计文档移至 [docs/Engineering_plan.md](docs/Engineering_plan.md)。
- docs/ARCHITECTURE.md 对齐工程设计：四层分层心智模型、仓库布局、契约纪律与语言栈硬约束。
