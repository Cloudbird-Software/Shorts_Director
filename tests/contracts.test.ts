import { readdirSync } from "node:fs";
import { describe, expect, it } from "vitest";
import {
  CANVAS,
  CONTRACT_VERSIONS,
  RENDER_CRAFT,
  SCHEMA_VERSIONS,
  VOCAB_V1,
  beatRole,
} from "../src/index.js";

const schemaRoot = new URL("../schema/", import.meta.url);

const toSnake = (s: string): string =>
  s
    .replace(/([A-Z])/g, "_$1")
    .toLowerCase()
    .replace(/^_/, "");

describe("SCHEMA_VERSIONS 与 schema/entities 对齐", () => {
  it("每个实体常量都有对应 schema 文件", () => {
    const files = readdirSync(new URL("entities/", schemaRoot)).map((f) =>
      f.replace(".schema.json", ""),
    );
    const expected = (Object.keys(SCHEMA_VERSIONS) as string[]).map(toSnake);
    expect(expected.every((name) => files.includes(name))).toBe(true);
  });

  it("版本串格式为 <entity>/<major>", () => {
    for (const v of Object.values(SCHEMA_VERSIONS)) {
      expect(v).toMatch(/^[/a-z0-9_.]+\/\d+$/);
    }
  });
});

describe("VOCAB_V1 与 schema/vocab/v1 对齐", () => {
  it("词表清单与 YAML 文件一一对应", () => {
    const files = readdirSync(new URL("vocab/v1/", schemaRoot)).map((f) =>
      f.replace(".yaml", ""),
    );
    expect([...VOCAB_V1].sort()).toEqual(files.sort());
  });
});

describe("冻结契约", () => {
  it("beat_role 7 值冻结，顺序即叙事链路", () => {
    expect([...beatRole]).toHaveLength(7);
    expect(beatRole[0]).toBe("HOOK");
    expect(new Set(beatRole).size).toBe(7);
  });

  it("画布与工艺参数与设计一致", () => {
    expect(CANVAS).toEqual({ width: 1080, height: 1920, fps: [25, 30] });
    expect(RENDER_CRAFT.maxSpeed).toBeLessThanOrEqual(1.15);
    expect(RENDER_CRAFT.targetLufs).toBe(-14);
  });

  it("C2/C3 契约版本冻结在 1", () => {
    expect(CONTRACT_VERSIONS).toEqual({ operator: 1, render: 1 });
  });
});
