# internal/entity —— 冻结实体的 Go 类型与跨字段不变式

## 职责

- 结构体与 schema/entities/*.json 字段一一对应（json tag 对齐，可 round-trip）
- 跨字段不变式（IV-*）的运行期校验：Shot 的 IV-SH-1/2/3、Provenance/VersionedRef 语义
- 生命周期 FSM 的唯一迁移表（禁止布尔字段拼装状态）

## 边界（重要）

- JSON Schema **结构**校验（required/additionalProperties/enum…）由 TS 侧 G1
  harness 负责；本包只做"结构合法之后"的**业务不变式**——两侧共享
  schema/testdata 样本，Go 测试直接消费同一批 G1 样本防漂移。
- 时间字段：identity 用整数帧；lifecycle 用 YYYY-MM-DD 字符串（schema 如此）。

## 不变量

- `Shot.Validate`：帧边界、fps∈{25,30}、duration_frames 生成列一致、
  subjects≤5、IV-SH-2（risk_flags ⇒ 非 AVAILABLE）
- `Shot.EligibleForVertical`：IV-SH-1（safe_crop 不可裁且未声明 PILLARBOX ⇒ 不可入 9:16 候选池）
- `Shot.IsConsumable(now)`：状态 + 合规 + IV-SH-3（ttl 过期出池）的合取
- `CanTransition`：唯一合法迁移表，终态 EXPIRED 不可逆

## 验证

`go test ./internal/entity/`（G1 valid 样本全过 + FSM 正反例 + IV 逐条触发）。
