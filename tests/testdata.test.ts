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
const schemaKeys = (): Set<string> => {
  const keys = new Set<string>();
  const walk = (dir: URL) => {
    for (const f of readdirSync(dir)) {
      if (f.endsWith(".json")) {
        const schema = JSON.parse(readFileSync(new URL(f, dir), "utf8"));
        const id: string = schema.$id ?? "";
        if (id.startsWith(ID_PREFIX)) {
          keys.add(id.slice(ID_PREFIX.length).replace(/\.json$/, ""));
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
  return keys;
};

// testdata 侧实际存在的 key（含 valid/invalid 子目录的目录）
const testdataKeys = (): string[] => {
  const out: string[] = [];
  const walk = (prefix: string, dir: URL) => {
    for (const f of readdirSync(dir)) {
      const sub = new URL(`${f}/`, dir);
      if (!statSync(sub).isDirectory()) continue;
      const key = prefix ? `${prefix}/${f}` : f;
      if (
        existsSync(new URL("valid/", sub)) ||
        existsSync(new URL("invalid/", sub))
      ) {
        out.push(key);
      } else {
        walk(key, sub);
      }
    }
  };
  walk("", testdataRoot);
  return out.sort();
};

const discoverEntities = (): string[] => {
  const keys = schemaKeys();
  return testdataKeys().filter((k) => keys.has(k));
};
const orphanTestdata = (): string[] =>
  testdataKeys().filter((k) => !schemaKeys().has(k));

const entities = discoverEntities();

describe("G1 testdata harness", () => {
  it("schema/testdata 至少覆盖一个实体", () => {
    expect(entities.length).toBeGreaterThan(0);
  });

  it("无孤儿样本目录（拼错/$id 不匹配的 testdata 必须报错，禁止静默跳过）", () => {
    expect(orphanTestdata()).toEqual([]);
  });
});

describe.each(entities)("G1: %s", (entity) => {
  const validDir = new URL(`${entity}/valid/`, testdataRoot);
  const invalidDir = new URL(`${entity}/invalid/`, testdataRoot);
  const evolutionDir = new URL(`${entity}/evolution/`, testdataRoot);
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

  // G5 向后兼容（Freeze Gate）：evolution/ 存放上一 major 的真实样本，
  // 当前 schema 必须仍能消费。v1 期间先落 v1 基线（v1_minimal.json），
  // v2 破坏性变更时这些文件即回归基线，禁止随 v2 改写。
  const evolutionSamples = existsSync(evolutionDir)
    ? listJson(evolutionDir)
    : [];
  if (evolutionSamples.length > 0) {
    it.each(evolutionSamples)(
      "evolution/%s 当前 schema 仍可消费（G5）",
      (f) => {
        const ok = validate(load(evolutionDir, f));
        expect(ok).toBe(true);
        expect(validate.errors ?? []).toEqual([]);
      },
    );
    it("evolution 样本文件名标注来源 major（v<major>_ 前缀）", () => {
      for (const f of evolutionSamples) {
        expect(f).toMatch(/^v[0-9]+_[a-z0-9]+(_[a-z0-9]+)*\.json$/);
      }
    });
  }
});
