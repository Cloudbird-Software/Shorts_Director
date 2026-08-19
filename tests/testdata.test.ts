// G1 冻结门 harness：schema/testdata/<entity>/{valid,invalid}/*.json
// 规则（schema/AGENTS.md）：
//   valid ≥5（minimal/typical/maximal/边界）必须全部通过校验
//   invalid ≥15（文件名即断言）必须全部被拒绝
// 本文件零实体特判——新增实体只要放好 schema + testdata 目录即自动纳管。
import { existsSync, readdirSync, readFileSync, statSync } from "node:fs";
import { describe, expect, it } from "vitest";
import Ajv2020 from "ajv/dist/2020.js";
import addFormats from "ajv-formats";

const schemaRoot = new URL("../schema/", import.meta.url);
const testdataRoot = new URL("../schema/testdata/", import.meta.url);

function buildAjv(): InstanceType<typeof Ajv2020> {
  const ajv = new Ajv2020({ strict: false, allErrors: true });
  addFormats(ajv);
  // 注册全部 common + entities + contracts schema，跨文件 $ref 可解析
  const registerDir = (dir: URL) => {
    for (const f of readdirSync(dir)) {
      const p = new URL(f, dir);
      if (f.endsWith(".json")) {
        ajv.addSchema(JSON.parse(readFileSync(p, "utf8")));
      } else if (statSync(p).isDirectory()) {
        registerDir(new URL(`${f}/`, dir));
      }
    }
  };
  for (const dir of ["common", "entities", "contracts"]) {
    registerDir(new URL(`${dir}/`, schemaRoot));
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

// 纳管发现：testdata/<key>/ 存在 ⇒ key 对应 schema $id .../v1/<key>.json
// key 可嵌套（如 contracts/operator/request），与 schema 目录结构一致
const ID_PREFIX = "https://shorts.director/schemas/v1/";
const discoverEntities = (): string[] => {
  const out: string[] = [];
  const walk = (dir: URL) => {
    for (const f of readdirSync(dir)) {
      if (f.endsWith(".json")) {
        const schema = JSON.parse(readFileSync(new URL(f, dir), "utf8"));
        const id: string = schema.$id ?? "";
        if (id.startsWith(ID_PREFIX)) {
          const key = id.slice(ID_PREFIX.length).replace(/\.json$/, "");
          if (existsSync(new URL(`${key}/`, testdataRoot))) out.push(key);
        }
      } else {
        const sub = new URL(`${f}/`, dir);
        if (statSync(sub).isDirectory()) walk(sub);
      }
    }
  };
  for (const dir of ["common", "entities", "contracts"]) {
    walk(new URL(`${dir}/`, schemaRoot));
  }
  return out.sort();
};

const entities = discoverEntities();

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
      expect(f).toMatch(/^[a-z0-9]+(_[a-z0-9]+)*\.json$/);
    }
  });
});
