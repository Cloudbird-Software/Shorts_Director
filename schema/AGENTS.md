# schema/ —— 单一真源（Single Source of Truth）

## 职责

全系统唯一的契约真源：受控词表、实体 Schema、服务间契约、测试样本。
Go / TS / BAML / SQL 的类型一律由此生成（`make gen`，产物进 `codegen/`，禁止手改）。

## vocab YAML 格式（v1）

```yaml
version: 1
name: shot_type
kind: enum
values:
  - id: CLOSEUP # 大写蛇形，DB enum / 代码常量直接采用
    zh: 特写 # 中文展示名（给商家/外包看）
    def: 主体占画面 30%–70% # 一句话定义，VLM 打标与 QC 断言共用
    equivalence_class: [DETAIL, FACE] # 等价类：slot query 按类取材的依据
    deprecated: false # 废弃时 true
    replaced_by: null # 废弃时的继任者 id
```

- 词表是 VLM 打标的输出约束、ShotSlotQuery 的合法取值域、QC 断言的谓词域、
  制作令的规格语言——**四处共用一套词表，改一处必须想四处**。
- 按垂类分表的词表命名 `<name>.<vertical>.yaml`（如 `scene.food.yaml`）。

## 演进规则（违反 = 破坏性变更）

1. enum **只允许追加**，禁止删除或改 id 语义；废弃用 `deprecated: true` +
   `replaced_by`，DB 侧走 `ALTER TYPE ... ADD VALUE`。
2. `def` / `zh` 的措辞变更视为语义变更，需在 PR 里说明影响面。
3. 等价类只允许扩容（一个 id 可以加入新等价类），禁止移出已有等价类。

## entities 规则

- JSON Schema draft 2020-12；`$id` 统一 `https://shorts.director/schemas/v1/<entity>.json`。
- 跨字段不变式在 schema `description` 中以 `IV-<实体缩写>-<序号>` 编号，
  实现侧（Go invariants 包）必须逐条对应并有 invalid 样本。
- 时间一律整数帧（字段名后缀 `_frame`），禁止 float seconds。
- 跨实体引用用 `common/versioned_ref.json`，禁止裸 id 引用可变实体。

## testdata 规则（Freeze Gate G1）

```
testdata/<entity>/
├── valid/      ≥5 个：minimal / typical / maximal / 边界
├── invalid/    ≥15 个：文件名即断言（missing_pillars.json、enum_bad_shot_type.json…）
└── evolution/  上一 major 版本的真实样本，验证向后兼容
```

## 禁止

- 手改 `codegen/` 下任何文件
- schema/testdata 中出现真实租户名、密钥、连接串
- 在 L3（范式层）schema 中引用 L2 具体实例（asset_id/shot_id）

## 验证

- 当前：`make check`（格式 + lint + 测试）
- 契约工具链就位后：`make gen && make contract-test`（跨语言一致性 + testdata 全量跑）
