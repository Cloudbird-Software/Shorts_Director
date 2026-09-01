#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""IR-0007 套件——生成一等公民营销视频实验平台 spec 的结构+语义锚断言。

被审"实现" = impl-dir 下的 spec.md（文档对形态：本 IR 首件交付物是条款级
规格本身）。断言四层（对齐 specs 先例 Viral_Radar#IR-0001 / QW_Arena1#IR-0001 口径）：
  L1 结构：frontmatter 字段、AC-1..AC-11 完备、条款段齐备
  L2 语义锚：生成一等公民实验平台才含的机制短语 + 条款级锚绑定
  L3 负向锚：偷懒改写最易缺的深水位标志（数值/枚举/口径/弱化词镜像）
  L4 一致性：AC 数、holdout 引用格式、防模板句复用、与 IR-0007 期望对齐
防摆拍口径（先例红队五轮沉淀，S1'-S6）：
  S1' 摆拍式 AC、S2 义务降级、S3 义务转嫁、S4 时态后移、S5 逃生舱、S6 前置堆叠。
"""
import os
import re
import sys
import unittest

_cwd = os.path.abspath(os.getcwd())
IMPL = None
if os.environ.get("IMPL_DIR"):
    IMPL = os.path.normpath(os.environ["IMPL_DIR"])
elif os.path.isfile(os.path.join(_cwd, "spec.md")):
    IMPL = _cwd
elif os.path.isfile(os.path.join(_cwd, "..", "spec.md")):
    IMPL = os.path.normpath(os.path.join(_cwd, ".."))
if IMPL is None:
    raise AssertionError("无法定位 impl 目录（IMPL_DIR 未设且 cwd 上下文无 spec.md）")
SPEC = os.path.join(IMPL, "spec.md")


def read(path):
    if not os.path.isfile(path):
        raise AssertionError(f"缺文件: {path}")
    with open(path, encoding="utf-8") as f:
        return f.read()


def frontmatter(text):
    m = re.match(r"^---\n(.*?)\n---\n", text, re.S)
    if not m:
        raise AssertionError("缺 frontmatter（--- 包裹的元数据块）")
    return m.group(1)


# ── L2 语义锚：真实"生成一等公民实验平台"spec 才含的机制短语 ──────────────
SEMANTIC_ANCHORS = [
    # 范式与实验设计锚
    "生成一等公民", "出片率", "抽卡", "capability profile", "run artifact",
    "内容寻址", "假设看板", "实验战役", "形态1", "形态4", "V100",
    # 生成链路锚
    "图生视频", "语音合成", "口型同步", "评审探针", "vlm_boolean",
    "seed", "断言包", "AIGC 标识", "信息层",
    # 治理锚
    "四态", "算子契约", "退役", "golden", "mock 商家", "混淆矩阵",
    # 口径锚
    "K 次抽卡", "逐字一致", "复算", "预算截断",
]
# ── L3 负向锚：弱化改写最易丢失的深水位标志 ──────────────────────────────
NEGATIVE_ANCHORS = [
    # 数值深水位：数字被弱化即丢
    "1080x1920", "不少于 3 个 mock 商家", "AC-1 至 AC-10",
    # 口径深水位：唯一口径/显式标注，弱化后即出多口径漏洞
    "口径全系统唯一", "显式标注为估算", "禁止以 1-10 打分",
    # 顺序深水位：先抓后删不可倒置
    "先归档", "再删除", "退役前",
    # 时态锚：必须现在时义务
    "必须", "不得",
]


class L1Structure(unittest.TestCase):
    def test_frontmatter_keys(self):
        fm = frontmatter(read(SPEC))
        for k in ("taskId: IR-0007", "specVersion:", "title:", "irRef:",
                  "acceptanceCriteria:", "blastRadius:", "nonGoals:"):
            self.assertIn(k, fm, f"frontmatter 缺 {k}")

    def test_ac_complete(self):
        s = read(SPEC)
        for i in range(1, 12):
            self.assertIn(f"- id: AC-{i}", s, f"缺 AC-{i}")
        self.assertEqual(len(re.findall(r"^\s*- id: AC-\d+", s, re.M)), 11,
                         "AC 总数应为 11（编号连续且无影子条款）")

    def test_ac_gwt(self):
        s = read(SPEC)
        acs = re.findall(r"- id: AC-\d+\n\s+given:(.*?)\n\s+when:(.*?)\n\s+then:(.*?)\n", s, re.S)
        self.assertEqual(len(acs), 11, "given/when/then 三段必须逐条配对")
        for i, (g, w, t) in enumerate(acs, 1):
            self.assertTrue(g.strip(), f"AC-{i} given 为空")
            self.assertTrue(w.strip(), f"AC-{i} when 为空")
            self.assertTrue(t.strip(), f"AC-{i} then 为空")
            self.assertTrue(len(t.strip()) > 20, f"AC-{i} then 过短（摆拍式 AC）")

    def test_clause_sections(self):
        s = read(SPEC)
        for sec in ("## INV 不变量", "## BEH 行为", "## IFACE 契约", "## BUDGET 预算",
                    "## DECISION 决策", "## ASSUMPTION 假设",
                    "## 测试设计（逐类讨论，testing.yaml 逐项过堂）",
                    "## holdout 测试设计"):
            self.assertIn(sec, s, f"缺条款节: {sec}")

    def test_blast_radius_nonempty(self):
        fm = frontmatter(read(SPEC))
        self.assertRegex(fm, r"blastRadius:\s*\n\s+-", "blastRadius 为空")

    def test_nongoals_preserved(self):
        s = read(SPEC)
        for ng in ("不做商家输入端", "不引入商业渲染订阅"):
            self.assertIn(ng, s, f"非目标丢失: {ng}")


class L2SemanticAnchors(unittest.TestCase):
    def test_anchors_present(self):
        s = read(SPEC)
        missing = [a for a in SEMANTIC_ANCHORS if a not in s]
        self.assertFalse(missing, f"语义锚缺失（意图空心化）: {missing}")

    def test_clause_bound_anchors(self):
        """条款级锚绑定：锚必须出现在正确条款域内，不能堆在背景叙述里充数。"""
        s = read(SPEC)
        inv = s.split("## INV 不变量")[1].split("## BEH")[0]
        self.assertIn("判定题化", inv, "INV 域缺判定题化锚")
        self.assertIn("内容寻址", inv, "INV 域缺内容寻址锚")
        iface = s.split("## IFACE 契约")[1].split("## BUDGET")[0]
        self.assertIn("口径全系统唯一", iface, "IFACE 域缺唯一口径锚")
        beh = s.split("## BEH 行为")[1].split("## IFACE")[0]
        self.assertIn("出片率", beh, "BEH 域缺出片率锚")
        self.assertIn("capability profile", beh, "BEH 域缺环境探测锚")


class L3NegativeAnchors(unittest.TestCase):
    def test_deep_markers(self):
        s = read(SPEC)
        missing = [a for a in NEGATIVE_ANCHORS if a not in s]
        self.assertFalse(missing, f"深水位标志缺失（弱化改写）: {missing}")

    def test_no_score_gate(self):
        s = read(SPEC)
        self.assertNotIn("1-10 打分或自由文本评价作为门禁以外的", s, "打分门禁表述自相矛盾")

    def test_no_escape_hatch_on_aigc(self):
        """S5 逃生舱：AIGC 标识/合规底线不得携带豁免后门。"""
        s = read(SPEC)
        inv = s.split("## INV 不变量")[1].split("## BEH")[0]
        aigc = [ln for ln in inv.splitlines() if "AIGC" in ln]
        self.assertTrue(aigc, "INV 域缺 AIGC 标识条款")
        for ln in aigc:
            self.assertNotIn("可以省略", ln, f"AIGC 条款含逃生舱: {ln}")
            self.assertNotIn("尽量", ln, f"AIGC 条款含弱化词: {ln}")

    def test_retire_order_enforced(self):
        """S6/AC-2：先抓 golden 再删的顺序约束必须显式。"""
        s = read(SPEC)
        self.assertRegex(s, r"先.*归档.*再.*删除|先归档.*再删除", "退役顺序约束未显式成文")


class L4Consistency(unittest.TestCase):
    def test_holdout_ref_format(self):
        """holdout 引用只允许 id@sha8，禁止 payload 内容泄露。"""
        s = read(SPEC)
        ho_sec = s.split("## holdout 测试设计")[1]
        self.assertIn("HO-0009@", ho_sec, "缺 HO-0009@sha8 引用")
        self.assertIn("HO-0010@", ho_sec, "缺 HO-0010@sha8 引用")
        for m in re.finditer(r"HO-\d{4}@([0-9a-f]{8})", ho_sec):
            self.assertEqual(len(m.group(1)), 8, "sha8 必须恰为 8 hex")
        # 泄题防线：payload 的商家名/电话等具体字段不得出现
        for leak in ("巷口小馆", "静颜皮肤管理", "028-", "笋子烧牛肉", "42元"):
            self.assertNotIn(leak, s, f"holdout payload 内容泄露进 spec: {leak}")

    def test_holdout_ids_are_registered(self):
        """引用的 holdout id 必须在合法已注册区间（HO-0009/HO-0010 于 holdout 仓 PR#10 注册）。"""
        s = read(SPEC)
        ids = set(re.findall(r"(HO-\d{4})@", s))
        self.assertTrue(ids.issubset({"HO-0009", "HO-0010"}),
                        f"引用了未注册 holdout 条目: {ids}")

    def test_ac_card_derivation_anchor(self):
        """AC-11 必须锚定红队 survived 与卡派生来源。"""
        s = read(SPEC)
        ac11 = re.search(r"- id: AC-11.*?(?=\n\s*- id:|\nnonGoals:)", s, re.S)
        self.assertIsNotNone(ac11, "缺 AC-11")
        self.assertIn("survived", ac11.group(0), "AC-11 缺红队 survived 锚")

    def test_no_template_reuse(self):
        """防模板句复用：then 段按标点切子句，跨 AC 整子句复用才 fail——
        受控术语（capability profile 等）跨 AC 引用是契约引用不是模板抄袭，
        整句复制粘贴才是（对齐先例防摆拍口径 S1'）。"""
        s = read(SPEC)
        acs = re.findall(r"- id: AC-\d+\n\s+given:(.*?)\n\s+when:(.*?)\n\s+then:(.*?)\n", s, re.S)
        seen = {}
        for idx, (_, _, t) in enumerate(acs, 1):
            clauses = re.split(r"[；，。、——]()（）]", t)
            for c in clauses:
                c = re.sub(r"\s+", "", c)
                if len(c) < 10:
                    continue  # 短语（如「携带 AIGC 标识」）属共享术语
                if c in seen:
                    self.fail(f"AC-{idx} then 子句与 AC-{seen[c]} 整句复用: {c}")
                seen[c] = idx

    def test_testing_yaml_coverage(self):
        """测试设计逐类讨论：active_now 15 项 + L 族 + R 族 + triggered 逐项 adopt/reject。"""
        s = read(SPEC)
        sec = s.split("## 测试设计")[1].split("## holdout")[0]
        for tid in ("T-01", "T-02", "T-03", "T-04", "T-05", "T-06", "T-07", "T-08",
                    "T-09", "T-10", "T-11", "T-12", "T-13", "T-14", "T-15"):
            self.assertIn(tid, sec, f"测试设计缺 {tid} 逐类讨论")
        for tid in ("L-01", "L-02", "L-03", "L-04", "L-05", "L-06"):
            self.assertIn(tid, sec, f"测试设计缺 {tid}")
        for tid in ("R-01", "R-06"):
            self.assertIn(tid, sec, f"测试设计缺 {tid}")
        self.assertIn("reject", sec, "测试设计缺 reject 理由（全部 adopt = 未过堂）")
        self.assertIn("adopt", sec, "测试设计缺 adopt")

    def test_ir_alignment(self):
        """IR 对齐：IR-0007 期望可观察变化的五要素必须映射进 AC。"""
        s = read(SPEC)
        for kw in ("capability profile", "出片率", "mock 商家", "假设看板",
                   "退役", "make check", "保持全绿"):
            self.assertIn(kw, s, f"IR 期望要素未映射: {kw}")


if __name__ == "__main__":
    unittest.main(verbosity=2)
