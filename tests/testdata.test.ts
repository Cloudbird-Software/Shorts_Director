// G1 冻结门 harness：schema/testdata/<entity>/{valid,invalid}/*.json
// 规则（schema/AGENTS.md）：
//   valid ≥5（minimal/typical/maximal/边界）必须全部通过校验
//   invalid ≥15（文件名即断言）必须全部被拒绝
// 本文件零实体特判——新增实体只要放好 schema + testdata 目录即自动纳管。
import { readdirSync, readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import Ajv2020 from "ajv/dist/2020.js";
import addFormats from "ajv-formats";

const schemaRoot = new URL("../schema/", import.meta.url);
const testdataRoot = new URL("../schema/testdata/", import.meta.url);

function buildAjv(): InstanceType<typeof Ajv2020> {
  const ajv = new Ajv2020({ strict: false, allErrors: true });
  addFormats(ajv);
  // 注册全部 common + entities schema，跨文件 $ref（versioned_ref 等）可解析
  for (const dir of ["common", "entities"]) {
    const d = new URL(`${dir}/`, schemaRoot);
    for (const f of readdirSync(d)) {
      if (f.endsWith(".json")) {
        ajv.addSchema(JSON.parse(readFileSync(new URL(f, d), "utf8")));
      }
    }
  }
  return ajv;
}

const ajv = buildAjv();

const listJson = (dir: URL): string[] =>
  readdirSync(dir)
    .filter((f) => f.endsWith(".json"))
    .sort();

const load = (dir: URL, f: string): unknown =>
  JSON.parse(readFileSync(new URL(f, dir), "utf8"));

const entities = readdirSync(testdataRoot);

describe("G1 testdata harness", () => {
  it("schema/testdata 至少覆盖一个实体", () => {
    expect(entities.length).toBeGreaterThan(0);
  });
});

describe.each(entities)("G1: %s", (entity) => {
  const validDir = new URL(`${entity}/valid/`, testdataRoot);
  const invalidDir = new URL(`${entity}/invalid/`, testdataRoot);
  const validate = ajv.getSchema(
    `https://shorts.director/schemas/v1/${entity}.json`,
  ) as (data: unknown) => boolean;

  it("样本规模达标（G1：≥5 valid + ≥15 invalid）", () => {
    expect(listJson(validDir).length).toBeGreaterThanOrEqual(5);
    expect(listJson(invalidDir).length).toBeGreaterThanOrEqual(15);
  });

  it.each(listJson(validDir))("valid/%s 通过", (f) => {
    const data = load(validDir, f);
    const ok = validate(data);
    expect(validate.errors ?? []).toEqual([]);
    expect(ok).toBe(true);
  });

  it.each(listJson(invalidDir))("invalid/%s 被拒绝", (f) => {
    const ok = validate(load(invalidDir, f));
    expect(ok).toBe(false);
    expect(validate.errors).not.toBeNull();
  });

  it("invalid 文件名 = 断言（snake_case，描述失败原因）", () => {
    for (const f of listJson(invalidDir)) {
      expect(f).toMatch(/^[a-z0-9_]+\.json$/);
    }
  });
});
