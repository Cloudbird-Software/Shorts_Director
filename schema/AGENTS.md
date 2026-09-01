# schema/ —— 单一真源（Single Source of Truth）

## 职责

全系统唯一的契约真源：受控词表、实体 Schema、服务间契约、测试样本。
Go / TS / BAML 的类型一律由此生成（`make gen`，产物进 `codegen/`，禁止手改）。
范式依据：specs/IR-0007（生成一等公民）；旧意图的契约冻结路线（Freeze Gate
12 项）已随退役归档，见 docs/history/Engineering_plan.md。

## vocab YAML 格式（v1）

```yaml
version: 1
name: shot_type
kind: enum
values:
  - id: CLOSEUP # 大写蛇形，DB enum / 代码常量直接采用
    zh: 特写 # 中文展示名（给商家/外包看）
    def: 主体占画面 30%–70% # 一句话定义，QC 断言与评审探针共用
    equivalence_class: [DETAIL, FACE] # 等价类
    deprecated: false # 废弃时 true
    replaced_by: null # 废弃时的继任者 id
```

- 词表是 QC 断言的谓词域、生成形态的规格语言、评审探针的输出约束——
  **多处共用一套词表，改一处必须想多处**。
- 按垂类分表的词表命名 `<name>.<vertical>.yaml`（如 `scene.food.yaml`）。
- **gen_form（生成形态）词表携带 bindings 扩展元数据**（duration_s / canvas /
  info_layer / assertion_pack，IFACE-3）：形态=数据而非代码分支；bindings
  数值变更视为语义变更，须同步断言包与套件定义。现行 codegen 不消费
  bindings，评估编排（internal/eval）直接读表。

## 演进规则（违反 = 破坏性变更）

1. enum **只允许追加**，禁止删除或改 id 语义；废弃用 `deprecated: true` +
   `replaced_by`，不删值。
2. `def` / `zh` / `bindings` 的措辞或数值变更视为语义变更，需在 PR 里说明影响面。
3. 等价类只允许扩容（一个 id 可以加入新等价类），禁止移出已有等价类。

## entities 规则

- JSON Schema draft 2020-12；`$id` 统一 `https://shorts.director/schemas/v1/<entity>.json`。
- 现存实体：asset / video_plan / qc_assertion / event（IR-0007 退役后）；
  新增实体须同步 org policy 契约声明（ADR-0105 显式枚举通道）。
- 跨字段不变式在 schema `description` 中以 `IV-<实体缩写>-<序号>` 编号，
  实现侧必须逐条对应并有 invalid 样本。
- 时间一律整数帧（字段名后缀 `_frame`），禁止 float seconds。
- 跨实体引用用 `common/versioned_ref.json`，禁止裸 id 引用可变实体。

## testdata 规则（G1 harness）

```
testdata/<entity>/
├── valid/      ≥5 个：minimal / typical / maximal / 边界
├── invalid/    ≥15 个：文件名即断言（missing_overlays.json、enum_bad_kind.json…）
└── evolution/  上一 major 版本的真实样本，验证向后兼容
```

evolution/ 语义：

- 文件名 `v<major>_<形态>.json`（如 `v1_minimal.json`）标注样本来自哪个 major。
- 样本一旦落盘即**钉死**：未来 major 的破坏性变更不得改写这些文件，
  新版本的 schema/实现必须仍能消费它们（harness 自动纳管，双侧消费测试）。

## 禁止

- 手改 `codegen/` 下任何文件
- schema/testdata 中出现真实商家/个人身份信息、密钥、连接串（INV-6）
- 把确定性信息（文字/价格/地址/电话/Logo）交进生成域——它们只属于信息层模板（INV-5）

## 验证

- `make check`（格式 + lint + 测试，含 codegen 新鲜度）
- `make go-check`（Go 侧 G2 一致性锚点，见 internal/videoplan/consistency_test.go）
