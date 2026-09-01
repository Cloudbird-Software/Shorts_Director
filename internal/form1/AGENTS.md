# internal/form1/ —— 形态1 端到端管线（IR-0007 AC-6 / BEH-5，实验 E5 切片）

mock 商家（种子图 + 信息表）→ gen_i2v（C2 算子）→ VideoPlan 构造 →
renderer 渲染（真实解码 + 信息层叠加 + AIGC 双轨）→ 形态1 断言包 →
内容寻址 run artifact（含全链耗时分解）。

## 不变量

- INV-5 信息层零失败：店名/招牌项/价格/地址/电话只经 overlay 模板进入
  成品（plan 构造期逐字取自 merchant.json），禁止交给生成域；
  「逐字一致」按构造断言落 artifact（渲染确定性由 R-1 保证）。
- INV-3 非确定性显式落盘：gen 产物 content_hash、plan digest、断言
  判定全部进 artifact；artifact 自身 JCS 内容寻址。
- 出片率口径复用 internal/eval（IFACE-5 唯一口径）。
- plan 构造确定性：同 merchant + 同 gen 产物 + 同日期 → 恒同 plan
  （PlanID/Provenance 从输入派生，不读时钟）。

## 禁止

- 禁止在本包内做语义质量判断（VLM 裁判属卡 #121 vlm_boolean）。
- 禁止绕过 qc 引擎自行发明判定格式——探针断言走 Engine，
  构造断言复用 qc.Result 形态。

## 验证

- `make go-check`（pipeline_test.go：fake 后端 3 商家端到端）。
