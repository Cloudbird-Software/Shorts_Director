// 受控词表 codegen：schema/vocab/v1/*.yaml → codegen/ts/vocab.ts
// 用法：make gen（写盘）/ import { renderVocabTs } 供 CI 新鲜度测试调用。
import { mkdirSync, readdirSync, readFileSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";
import yaml from "js-yaml";
import prettier from "prettier";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const vocabDir = path.join(root, "schema", "vocab", "v1");
const outFile = path.join(root, "codegen", "ts", "vocab.ts");

const assert = (cond, msg) => {
  if (!cond) throw new Error(`[gen-vocab] ${msg}`);
};

const camel = (s) => s.replace(/[._]([a-z0-9])/g, (_, c) => c.toUpperCase());
const pascal = (s) => {
  const c = camel(s);
  return c[0].toUpperCase() + c.slice(1);
};

/** 读取全部词表 YAML 并做结构断言（schema 改坏时 fail fast）。 */
export function loadVocabs() {
  const files = readdirSync(vocabDir)
    .filter((f) => f.endsWith(".yaml"))
    .sort();
  assert(files.length > 0, "schema/vocab/v1 为空");
  return files.map((file) => {
    const doc = yaml.load(readFileSync(path.join(vocabDir, file), "utf8"));
    const name = file.replace(/\.yaml$/, "");
    assert(doc.version === 1, `${file}: version 必须为 1`);
    assert(doc.kind === "enum", `${file}: 暂只支持 kind=enum`);
    assert(doc.name === name, `${file}: name "${doc.name}" 与文件名不一致`);
    const ids = doc.values.map((v) => v.id);
    assert(ids.length >= 3, `${file}: 词表至少 3 值`);
    assert(
      ids.every((id) => /^[A-Z][A-Z0-9_]*$/.test(id)),
      `${file}: id 必须大写蛇形（ /^[A-Z][A-Z0-9_]*$/ ）`,
    );
    assert(new Set(ids).size === ids.length, `${file}: id 重复`);
    return { name, frozen: doc.frozen === true, ids };
  });
}

/** 生成 codegen/ts/vocab.ts 的文本（已 prettier 格式化，确定性输出）。 */
export async function renderVocabTs() {
  const vocabs = loadVocabs();
  const lines = [
    "// AUTO-GENERATED from schema/vocab/v1/*.yaml by make gen —— 禁止手改。",
    "// 词表演进规则见 schema/AGENTS.md：enum 只允许追加，废弃用 deprecated/replaced_by。",
    "",
    "/** 词表清单（schema/vocab/v1/*.yaml 文件名，不含后缀） */",
    `export const VOCAB_FILES = [${vocabs
      .map((v) => `"${v.name}"`)
      .join(", ")}] as const;`,
    "export type VocabName = (typeof VOCAB_FILES)[number];",
    "",
  ];
  for (const v of vocabs) {
    lines.push(
      `/** ${v.name}（${v.ids.length} 值${v.frozen ? "，冻结" : ""}） */`,
      `export const ${camel(v.name)} = [${v.ids
        .map((id) => `"${id}"`)
        .join(", ")}] as const;`,
      `export type ${pascal(v.name)} = (typeof ${camel(v.name)})[number];`,
      "",
    );
  }
  return prettier.format(lines.join("\n"), { filepath: outFile });
}

async function main() {
  const code = await renderVocabTs();
  mkdirSync(path.dirname(outFile), { recursive: true });
  writeFileSync(outFile, code);
  console.log(`[gen-vocab] wrote ${path.relative(root, outFile)}`);
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  await main();
}
