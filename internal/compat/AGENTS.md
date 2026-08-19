# compat —— 前向兼容消费边界（G8）

读侧降级的唯一实现点：上游（算子/事件/未来版本 schema 的产物）数据含
v1 不认识的内容时，消费端降级而不是崩溃。与写侧护栏（entity.Validate /
TS ajv additionalProperties:false）互补——写侧拒漂移，读侧容未来。

## 路径

1. **未知字段忽略**：`DecodeTolerant`——encoding/json 默认忽略未知字段，
   本入口锁定该语义。禁止在消费边界用 DisallowUnknownFields。
2. **未知枚举降级**：`DegradeEnum`（单值→UNKNOWN+Raw）与
   `ScanUnknownEnums`（JSON Pointer 批量扫描，供日志/审计/决策）。

## 不变量

- 降级必须 fail-safe：未知状态/枚举按最保守语义处理
  （如未知 ShotState ⇒ 不可消费，测试锁定）
- Raw 原值必须保留（审计与后续升级判断依据）
- 写侧校验行为不受本包影响：Validate/ajv 对未来枚举照拒

## golden 样本

`testdata/forward/shot_from_future.json`：假想未来版本生产者的 Shot
（未知顶层/嵌套字段 + 未来 FSM 状态）。模拟"新生产者 → 旧消费者"场景，
改 fixture 必须同步改测试期望。

## 验证

`make go-check`（gofmt + go vet + go test ./internal/compat/...）。
