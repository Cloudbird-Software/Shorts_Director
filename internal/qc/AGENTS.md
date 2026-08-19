# qc —— S7 QCService 断言引擎

QCAssertion DSL 的求值器（Engineering_plan §S7）：applies_when 过滤 →
probe 分组去重 → 成本排序 → BLOCKER 短路 → remedy 指令渲染。
评估判定题化（A5）：assertion → bool + 证据，禁止 1–10 打分作门禁。

## 边界

- probe 算子本体属 C2 边界：引擎只依赖 `ProbeOperator` 接口，实现注入。
- 结构级校验（schema enum/required）由 TS 侧 G1 harness 负责；
  `Assertion.Validate` 只做"结构合法之后"的受控域校验
  （assertion_id 前缀、词表绑定、expect 形态、auto_fix 语义）。
- applies_when 复用 slotquery 谓词语义（`EvaluateFields`），
  字段域仍是 IV-SQ-2 白名单；关联字段（source_kind 等）由编排层
  展平进 `Subject.Fields`。

## 不变量

- 去重键 = probe op + args 的 JCS 摘要：同对象同 op+args 只测一次，
  多断言共享测量（成本控制）。
- 成本排序 + BLOCKER 短路：BLOCKER 失败后剩余组全部跳过——
  一个 L0 BLOCKER 失败不该再跑 L2 的 SyncNet。
- probe.Measure 出错 ⇒ Run 整体报错（基础设施故障不产生假报告）。
- 模板变量未定义 ⇒ 报错，不静默渲染空串（错误指令比失败更糟）。

## 阈值策略（L0/L1/L2 vs L3 相反）

素材 QC 宁可漏检（错杀触发无谓返修）；L3 合规宁可错杀
（合规事故不可逆）。阈值校准走 assertion_set_version 版本化。

## 禁止

- 在引擎内做子串/模糊匹配——命中语义归 probe，expect 只做集合判定。
- 引擎内读时钟/随机源/网络（纯编排，可测）。

## 验证

`make go-check`（gofmt + go vet + go test ./internal/qc/...）
