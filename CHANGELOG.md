# Changelog

本文件记录对外可见的变更。格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)。

## [Unreleased]

### Added
- 初始模板工程（CI gate / hygiene / dependabot / automerge 全套护栏）。
- schema/ 单一真源目录与模块规约（schema/AGENTS.md）。
- 受控词表 v1 第一批（结构类）：beat_role（冻结）、audio_role、shot_type、camera_motion、proof_type。
- 受控词表 v1 第二批（语义类·餐饮垂类）：scene.food（14 值）、subject.food（24 值）。
- 受控词表 v1：action（38 值，跨垂类通用、餐饮校准，等价类 PREP/COOK/PRESENT/SERVICE/SUPPLY/REACTION/SOCIAL）。

### Changed
- 仓库定位从 template-service 模板切换为 Shorts_Director（README 重写、包名更新）。
- 工程设计文档移至 [docs/Engineering_plan.md](docs/Engineering_plan.md)。
- docs/ARCHITECTURE.md 对齐工程设计：四层分层心智模型、仓库布局、契约纪律与语言栈硬约束。
