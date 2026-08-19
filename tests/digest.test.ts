// Freeze Gate G3：JCS 共享向量集（testdata/digest/jcs_vectors.json）的 TS 侧
// 锚点——与 Go 侧 internal/digest/vectors_test.go 消费同一向量文件，
// canonical 与 sha256 必须逐字节一致。任何一侧实现漂移 ⇒ 双侧测试同时失败。
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

import { canonicalize, canonicalizeJsonText } from "../src/digest/index.js";

interface Vector {
  name: string;
  input: string;
  canonical: string;
  sha256: string;
}

const suite: { comment: string; vectors: Vector[] } = JSON.parse(
  readFileSync(
    new URL("../testdata/digest/jcs_vectors.json", import.meta.url),
    "utf8",
  ),
);

describe("G3: JCS 共享向量（TS 侧）", () => {
  it("向量集规模 ≥5", () => {
    expect(suite.vectors.length).toBeGreaterThanOrEqual(5);
  });

  it.each(suite.vectors)("$name：canonical 与 sha256 与共享向量一致", (v) => {
    const canonical = canonicalizeJsonText(v.input);
    expect(canonical).toBe(v.canonical);
    const sha256 = createHash("sha256").update(canonical, "utf8").digest("hex");
    expect(sha256).toBe(v.sha256);
  });

  it("key 顺序不敏感：同内容不同顺序的 JSON 规范化后相等（G3 第二断言）", () => {
    const a = canonicalizeJsonText('{"a":1,"b":{"d":4,"c":3}}');
    const b = canonicalizeJsonText('{"b":{"c":3,"d":4},"a":1}');
    expect(a).toBe(b);
  });

  it("非 JSON 数据模型拒绝（NaN/Infinity/undefined/bigint）", () => {
    expect(() => canonicalize({ x: Number.NaN })).toThrow();
    expect(() => canonicalize({ x: Number.POSITIVE_INFINITY })).toThrow();
    expect(() => canonicalize({ x: undefined })).toThrow();
    expect(() => canonicalize({ x: 1n })).toThrow();
    // JSON 文本入口：非法 JSON 报错
    expect(() => canonicalizeJsonText("{not json")).toThrow();
  });
});
