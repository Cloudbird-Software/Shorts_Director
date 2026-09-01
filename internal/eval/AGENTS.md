# internal/eval —— 制式评估编排（E2 出片率仪器）

specs/IR-0007 AC-4/AC-5 / BEH-3/BEH-4 / IFACE-2/IFACE-5。套件定义 →
逐条生成（C2 Runner）→ 逐条断言（复用 qc 引擎）→ 聚合出片率 →
内容寻址 run artifact。CLI：cmd/shorts-eval。

## 不变量

- **出片率口径全系统唯一（IFACE-5）**：可用 = 通过断言包全部断言；
  聚合 = K 次抽卡中至少 1 条可用的条目（entry）比例。任何报告不得
  使用第二套口径——新增聚合指标须经 spec 变更。
- **artifact 自包含（IFACE-2）**：内嵌套件定义全文 + capability profile
  引用；聚合可自 artifact 复算（ComputeYield 是唯一口径入口，
  property test 守护）。
- **预算截断（BUDGET-3）**：wall/gpu 超限即中止，剩余条目落盘标注
  SKIPPED_BUDGET；断言基础设施故障重试 ≤1 次（BUDGET-2）。
- 套件定义硬件无关：不含机器名/路径外的执行环境；执行环境签名
  经 profile ref 进入 artifact。

## 禁止

- 在本包引入第二套"可用性"判定（打分/加权/人工印象）。
- 聚合逻辑散落多处——只能调 ComputeYield。
- 套件定义里写 GPU 专属参数（显存/设备号）——那些属于 capability profile。

## 验证

`make go-check`（含 -race 干净的并发测试 + golden 报告样本 digest 钉死）。
