import { describe, expect, it } from "vitest";
import { VOCAB_FILES, VOCAB_IDS, VOCAB_META } from "../codegen/ts/vocab.js";
import {
  assertVocabId,
  equivalenceClassOf,
  isDeprecated,
  isVocabId,
  replacedBy,
} from "../src/index.js";

describe("词表注册表完整性（15 张 × ids/meta 双射）", () => {
  it("注册表键 = VOCAB_FILES", () => {
    expect([...VOCAB_FILES].sort()).toEqual([...Object.keys(VOCAB_IDS)].sort());
    expect([...VOCAB_FILES].sort()).toEqual(
      [...Object.keys(VOCAB_META)].sort(),
    );
  });

  it("每张词表 meta 键与 ids 严格一致（无孤儿、无缺失）", () => {
    for (const name of VOCAB_FILES) {
      expect([...Object.keys(VOCAB_META[name])].sort()).toEqual(
        [...VOCAB_IDS[name]].sort(),
      );
    }
  });
});

describe("运行期助手", () => {
  it("isVocabId / assertVocabId", () => {
    expect(isVocabId("shot_type", "CLOSEUP")).toBe(true);
    expect(isVocabId("shot_type", "NONSENSE")).toBe(false);
    expect(() => assertVocabId("beat_role", "NONSENSE")).toThrowError(
      /beat_role/,
    );
    expect(() => assertVocabId("beat_role", "HOOK")).not.toThrow();
  });

  it("v1 全量无废弃值；等价类可查询", () => {
    for (const name of VOCAB_FILES) {
      for (const id of VOCAB_IDS[name]) {
        expect(isDeprecated(name, id)).toBe(false);
        expect(replacedBy(name, id)).toBeNull();
        expect(equivalenceClassOf(name, id).length).toBeGreaterThan(0);
      }
    }
  });

  it("等价类锚点：HOOK∈OPENING、CLOSEUP∈DETAIL", () => {
    expect(equivalenceClassOf("beat_role", "HOOK")).toContain("OPENING");
    expect(equivalenceClassOf("shot_type", "CLOSEUP")).toContain("DETAIL");
  });
});
