import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import {
  renderVocabBaml,
  renderVocabGo,
  renderVocabTs,
} from "../scripts/gen-vocab.mjs";
import { shotType, VOCAB_FILES } from "../codegen/ts/vocab.js";

describe("codegen 新鲜度（改 schema 不跑 make gen = CI 红）", () => {
  it("codegen/ts/vocab.ts 与 schema/vocab/v1/*.yaml 再生成结果一致", async () => {
    const committed = readFileSync(
      new URL("../codegen/ts/vocab.ts", import.meta.url),
      "utf8",
    );
    expect(await renderVocabTs()).toBe(committed);
  });

  it("codegen/go/vocab/vocab.go 与 schema/vocab/v1/*.yaml 再生成结果一致", () => {
    const committed = readFileSync(
      new URL("../codegen/go/vocab/vocab.go", import.meta.url),
      "utf8",
    );
    expect(renderVocabGo()).toBe(committed);
  });

  it("baml_src/vocab.baml 与 schema/vocab/v1/*.yaml 再生成结果一致（C1 B-1）", () => {
    const committed = readFileSync(
      new URL("../baml_src/vocab.baml", import.meta.url),
      "utf8",
    );
    expect(renderVocabBaml()).toBe(committed);
  });
});

describe("生成产物与人工锚点一致", () => {
  it("词表清单无重复且规模不缩水", () => {
    expect(VOCAB_FILES.length).toBeGreaterThanOrEqual(14);
    expect(new Set(VOCAB_FILES).size).toBe(VOCAB_FILES.length);
  });

  it("shot_type 冻结 8 值", () => {
    expect(shotType).toHaveLength(8);
    expect(shotType).toContain("CLOSEUP");
  });
});
