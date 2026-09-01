// Freeze Gate G2（TS 侧锚点）：Go 结构性豁免登记表防腐烂。
// 登记表 testdata/g2_go_pass_invalid.json 记录「ajv 拒绝、Go Validate 放行」
// 的 invalid 样本（纯结构性失败 + 理由）。Go 侧一致性测试
// （internal/videoplan/consistency_test.go）负责判定对照；本测试保证：
//   1. 登记条目必须对应真实存在的 invalid 样本文件（防删除样本后登记残留）
//   2. 登记的实体必须有对应 testdata 目录（防拼错实体名静默失效）
//   3. 登记理由非空（每个豁免必须可评审）
import { existsSync, readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

const testdataRoot = new URL("../schema/testdata/", import.meta.url);

interface Allowlist {
  _comment?: string;
  [entity: string]: { [file: string]: string } | string | undefined;
}

const allowlist: Allowlist = JSON.parse(
  readFileSync(
    new URL("../testdata/g2_go_pass_invalid.json", import.meta.url),
    "utf8",
  ),
);

describe("G2: Go 结构性豁免登记表", () => {
  const entities = Object.keys(allowlist).filter((k) => k !== "_comment");

  it("登记表非空（G2 机制在线）", () => {
    expect(entities.length).toBeGreaterThan(0);
  });

  it.each(entities)("实体 %s 的登记条目全部有效", (entity) => {
    const entries = allowlist[entity] as { [file: string]: string };
    const names = Object.keys(entries);
    expect(names.length).toBeGreaterThan(0);
    for (const f of names) {
      // 条目必须是真实存在的 invalid 样本
      expect(
        existsSync(new URL(`${entity}/invalid/${f}`, testdataRoot)),
        `${entity}/invalid/${f} 不存在——登记过期`,
      ).toBe(true);
      // 理由非空（豁免必须可评审）
      expect(entries[f].trim().length).toBeGreaterThan(0);
    }
  });

  it("没有登记 whole valid 目录的实体（豁免只针对 invalid）", () => {
    for (const entity of entities) {
      expect(entity).not.toMatch(/\/valid/);
    }
  });
});
