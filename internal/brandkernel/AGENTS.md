# brandkernel —— BrandKernel 根契约的 Go 实体层

schema/entities/brand_kernel.schema.json 的 Go 映射：Onboarding 苏格拉底
问答的唯一产物、全系统根契约（L4 意图层）。结构体与 schema 一一对应
（json tag 对齐），漂移由 round-trip + DisallowUnknownFields 测试发现
（G1 valid 样本必须能反序列化并通过 Validate）。

## 不变量（Validate 强制）

- IV-BK-1：pillars ≥3 且每个 pillar 绑定 ≥2 种 proof_type
- IV-BK-2：`ReadyForL3Matching()`——completeness.score ≥ 0.75 才允许进入
  L3 匹配（停止条件=槽位覆盖度而非访谈轮次）；后续 S1 InterviewService /
  S4 ParadigmLibrary 以此为准入 API
- IV-BK-3：category 必须来自受控枚举（`ValidCategories()`，**临时 Go 侧
  集合**——business_category 词表冻结后切换 vocab.IsVocabID）且必须决定
  compliance_profile_id，禁止 LLM 自由生成
- 可视觉验证的 differentiator 必须绑定 ≥1 种 proof_type

## 禁止

- 给 BrandKernel 增加 schema 之外的字段（漂移 = 缺陷）
- proof_types 做词表强校验（G1 冻结样本含词表外值，词表对齐是独立议题）
- category 枚举绕过 ValidCategories() 散落多处

## 验证

`make go-check`（gofmt + go vet + go test ./internal/brandkernel/...）。
