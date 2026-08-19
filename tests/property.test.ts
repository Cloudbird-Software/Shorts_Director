/**
 * G9/G10 属性与变形测试（Freeze Gate，issue #44）——
 * fast-check 首次投入使用：词表生成物不变量 ≥1000 次无反例。
 *
 * 属性（G9）：
 *  - 任意词表 × 任意合法 id：isVocabId 为真（ids ↔ meta 双注册表闭合）
 *  - meta 元数据完整（zh 非空；等价类含自身或显式为空表）
 *  - 废弃值的 replacedBy 必落同表合法值（生成器结构断言的运行期对偶）
 * 变形（G10 补充面）：id 任意大小写变形 → isVocabId 为假（受控词表大小写敏感，
 *  生成物为全小写蛇形——大小写漂移不得静默命中）。
 */
import fc from "fast-check";
import { describe, expect, it } from "vitest";

import { VOCAB_IDS } from "../codegen/ts/vocab.js";
import {
  equivalenceClassOf,
  isDeprecated,
  isVocabId,
  replacedBy,
  zhOf,
} from "../src/contracts/vocab.js";

const vocabNames = Object.keys(VOCAB_IDS) as (keyof typeof VOCAB_IDS)[];

/** 任意 (词表, 该表合法 id) 二元组。 */
const arbVocabId: fc.Arbitrary<[string, string]> = fc
  .constantFrom(...vocabNames)
  .chain((vocab) =>
    fc
      .constantFrom(...(VOCAB_IDS[vocab] as readonly string[]))
      .map((id) => [vocab, id] as [string, string]),
  );

describe("G9: 词表生成物属性（≥1000 次）", () => {
  it("任意合法 id：isVocabId 恒真（ids ↔ meta 双闭合）", () => {
    fc.assert(
      fc.property(arbVocabId, ([vocab, id]) => {
        expect(isVocabId(vocab, id)).toBe(true);
      }),
      { numRuns: 1000 },
    );
  });

  it("任意合法 id：zh 非空、等价类为合法标签集、isDeprecated/replacedBy 同源", () => {
    fc.assert(
      fc.property(arbVocabId, ([vocab, id]) => {
        expect(zhOf(vocab, id).length).toBeGreaterThan(0);
        // 等价类是 id 所属的类别标签集（如 CUT → [PREP]）：非空字符串、无重复
        const eq = equivalenceClassOf(vocab, id);
        expect(new Set(eq).size).toBe(eq.length);
        for (const cls of eq) {
          expect(cls.length).toBeGreaterThan(0);
          expect(cls).toBe(cls.toUpperCase());
        }
        // 废弃 ⇔ replacedBy 非空，且继任者必为同表合法值（闭环替换链）
        const rep = replacedBy(vocab, id);
        expect(isDeprecated(vocab, id)).toBe(rep !== null);
        if (rep !== null) {
          expect(isVocabId(vocab, rep)).toBe(true);
        }
      }),
      { numRuns: 1000 },
    );
  });

  it("变形：id 大小写扰动 → isVocabId 恒假（受控词表禁静默命中漂移值）", () => {
    fc.assert(
      fc.property(arbVocabId, ([vocab, id]) => {
        // 词表 id 冻结为全大写/小写蛇形；大小写扰动后的值不得命中。
        // 无字母的 id（如纯数字）扰动后与原值相同，无变形可言，跳过。
        const mutated =
          id === id.toUpperCase() ? id.toLowerCase() : id.toUpperCase();
        if (mutated === id) return true;
        expect(isVocabId(vocab, mutated)).toBe(false);
      }),
      { numRuns: 1000 },
    );
  });
});
