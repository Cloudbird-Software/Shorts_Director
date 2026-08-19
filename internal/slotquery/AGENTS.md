# internal/slotquery —— ShotSlotQuery 谓词求值器（语义真源）

## 职责

- 谓词 AST（eq/in/neq/nin/gte/lte/gt/lt/between/semantic/and/or/not）对 Shot 的内存求值
- `Validate`：IV-SQ-1（降级链末端必须可解：terminal graphic 或空 must）与
  IV-SQ-2（字段白名单 + 词表受控取值，分表前缀匹配）
- `Match`（must ∧ ¬forbid 硬匹配）与 `Score`（should 加权打分）

## 不变量

- 求值是纯函数；未打标字段按"不匹配"处理，类型错配按错误处理（不静默）。
- semantic 仅 should（硬匹配直接报 ErrSemanticNotRankable，需要注入向量排序器）。
- 多值字段（subjects/actions/mood/…）比较语义 = 成员关系
  （eq⇔包含，in⇔交集，neq/nin⇔否定）。

## 禁止

- 本包不得引用具体 shot/asset 实例（L3 只引用等价类谓词——最高不变式）。
- SQL/pgvector 编译属 AssetQueryEngine（S3）；其输出必须与本包结果一致，
  以 schema/testdata G1 样本为对拍基准。

## 验证

`go test ./internal/slotquery/`（G1 valid 样本 Validate 全过 + IV 负例 +
全算子求值 + 组合逻辑）。
