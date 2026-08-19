// 受控词表 codegen：schema/vocab/v1/*.yaml → codegen/ts/vocab.ts + codegen/go/vocab/vocab.go
// 用法：make gen（写盘）/ import { renderVocabTs, renderVocabGo } 供 CI 新鲜度测试调用。
import { mkdirSync, readdirSync, readFileSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";
import yaml from "js-yaml";
import prettier from "prettier";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const vocabDir = path.join(root, "schema", "vocab", "v1");
const outFile = path.join(root, "codegen", "ts", "vocab.ts");
const outGoFile = path.join(root, "codegen", "go", "vocab", "vocab.go");

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
    for (const v of doc.values) {
      assert(
        typeof v.zh === "string" && v.zh.length > 0,
        `${file}/${v.id}: zh 缺失`,
      );
      assert(
        typeof v.def === "string" && v.def.length > 0,
        `${file}/${v.id}: def 缺失`,
      );
      assert(
        Array.isArray(v.equivalence_class) && v.equivalence_class.length > 0,
        `${file}/${v.id}: equivalence_class 至少 1 个`,
      );
      assert(
        v.equivalence_class.every((c) => /^[A-Z][A-Z0-9_]*$/.test(c)),
        `${file}/${v.id}: equivalence_class 必须大写蛇形`,
      );
      if (v.deprecated) {
        assert(
          typeof v.replaced_by === "string" && ids.includes(v.replaced_by),
          `${file}/${v.id}: 废弃值必须 replaced_by 同表合法 id`,
        );
      }
    }
    return { name, frozen: doc.frozen === true, values: doc.values };
  });
}

const metaEntry = (v) =>
  `{ zh: ${JSON.stringify(v.zh)}, def: ${JSON.stringify(v.def)}, equivalenceClass: [${v.equivalence_class
    .map((c) => `"${c}"`)
    .join(", ")}], deprecated: ${v.deprecated === true}, replacedBy: ${
    v.replaced_by === null || v.replaced_by === undefined
      ? "null"
      : JSON.stringify(v.replaced_by)
  } }`;

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
    "// ── 枚举值（id 元组 + 字面量类型） ─────────────────────────────",
    "",
  ];
  for (const v of vocabs) {
    const ids = v.values.map((x) => x.id);
    lines.push(
      `/** ${v.name}（${ids.length} 值${v.frozen ? "，冻结" : ""}） */`,
      `export const ${camel(v.name)} = [${ids
        .map((id) => `"${id}"`)
        .join(", ")}] as const;`,
      `export type ${pascal(v.name)} = (typeof ${camel(v.name)})[number];`,
      "",
    );
  }
  lines.push(
    "// ── 词表元数据（zh 展示名 / def 定义 / 等价类 / 废弃链） ─────",
    "",
  );
  for (const v of vocabs) {
    lines.push(
      `export const ${camel(v.name)}Meta = {`,
      ...v.values.map((x) => `  ${x.id}: ${metaEntry(x)},`),
      `} as const;`,
      "",
    );
  }
  lines.push(
    "// ── 注册表（按 VocabName 泛型取用，供运行期校验/查询助手） ────",
    "",
    "export type VocabMetaEntry = {",
    "  readonly zh: string;",
    "  readonly def: string;",
    "  readonly equivalenceClass: readonly string[];",
    "  readonly deprecated: boolean;",
    "  readonly replacedBy: string | null;",
    "};",
    "",
    "export const VOCAB_IDS = {",
    ...vocabs.map((v) => `  "${v.name}": ${camel(v.name)},`),
    "} satisfies Readonly<Record<VocabName, readonly string[]>>;",
    "",
    "export const VOCAB_META = {",
    ...vocabs.map((v) => `  "${v.name}": ${camel(v.name)}Meta,`),
    "} satisfies Readonly<Record<VocabName, Readonly<Record<string, VocabMetaEntry>>>>;",
    "",
  );
  return prettier.format(lines.join("\n"), { filepath: outFile });
}

/** Go 标识符：scene.food → SceneFood / beat_role → BeatRole。 */
const goIdent = (s) => {
  const parts = s.split(/[._]/).map((p) => p[0].toUpperCase() + p.slice(1));
  return parts.join("");
};

/** gofmt 对 map 复合字面量 key 做列对齐：value 起点列 = 最长 key + 1。 */
const alignMapEntries = (entries) => {
  const w = Math.max(...entries.map(([k]) => k.length));
  return entries.map(([k, v]) => `\t${k.padEnd(w + 1)}${v}`);
};

/** 生成 codegen/go/vocab/vocab.go 的文本（确定性输出，gofmt 兼容）。 */
export function renderVocabGo() {
  const vocabs = loadVocabs();
  const L = [
    "// AUTO-GENERATED from schema/vocab/v1/*.yaml by make gen —— 禁止手改。",
    "// 词表演进规则见 schema/AGENTS.md：enum 只允许追加，废弃用 deprecated/replaced_by。",
    "package vocab",
    "",
    "// Meta 是词表值的元数据：zh 展示名 / def 定义 / 等价类 / 废弃链。",
    "type Meta struct {",
    "\tZh               string",
    "\tDef              string",
    "\tEquivalenceClass []string",
    "\tDeprecated       bool",
    "\tReplacedBy       *string",
    "}",
    "",
    "// VocabFiles 是词表清单（schema/vocab/v1/*.yaml 文件名，不含后缀）。",
    `var VocabFiles = [...]string{${vocabs
      .map((v) => `"${v.name}"`)
      .join(", ")}}`,
    "",
    "// ── 枚举值清单 ─────────────────────────────────────────────",
    "",
  ];
  for (const v of vocabs) {
    const ids = v.values.map((x) => x.id);
    L.push(
      `// ${goIdent(v.name)} 是 ${v.name} 词表的值清单（${ids.length} 值${v.frozen ? "，冻结" : ""}）。`,
      `var ${goIdent(v.name)} = [...]string{${ids
        .map((id) => `"${id}"`)
        .join(", ")}}`,
      "",
    );
  }
  L.push("// ── 词表元数据 ─────────────────────────────────────────────", "");
  const metaVar = (v) => `${v.name.replace(/[._]/g, "_")}Meta`;
  for (const v of vocabs) {
    L.push(`var ${metaVar(v)} = map[string]Meta{`);
    // gofmt 对 map 复合字面量 key 做列对齐：value 起点列 = 最长 key + 1。
    const keys = v.values.map((x) => `"${x.id}":`);
    const keyWidth = Math.max(...keys.map((k) => k.length));
    v.values.forEach((x, i) => {
      const fields = [
        `Zh: ${JSON.stringify(x.zh)}`,
        `Def: ${JSON.stringify(x.def)}`,
        `EquivalenceClass: []string{${x.equivalence_class
          .map((c) => `"${c}"`)
          .join(", ")}}`,
      ];
      if (x.deprecated === true) fields.push(`Deprecated: true`);
      if (x.replaced_by != null)
        fields.push(`ReplacedBy: ptr(${JSON.stringify(x.replaced_by)})`);
      L.push(`\t${keys[i].padEnd(keyWidth + 1)}{${fields.join(", ")}},`);
    });
    L.push("}", "");
  }
  L.push(
    "// ── 注册表（按词表名取用，供运行期校验/查询助手） ──────────",
    "",
    "// VocabIDs 按词表名取值清单。",
    "var VocabIDs = map[string][]string{",
    ...alignMapEntries(
      vocabs.map((v) => [`"${v.name}":`, `${goIdent(v.name)}[:],`]),
    ),
    "}",
    "",
    "// VocabMeta 按词表名取元数据表。",
    "var VocabMeta = map[string]map[string]Meta{",
    ...alignMapEntries(
      vocabs.map((v) => [`"${v.name}":`, `${metaVar(v)},`]),
    ),
    "}",
    "",
    "func ptr(s string) *string { return &s }",
    "",
  );
  return L.join("\n");
}

async function main() {
  const code = await renderVocabTs();
  mkdirSync(path.dirname(outFile), { recursive: true });
  writeFileSync(outFile, code);
  console.log(`[gen-vocab] wrote ${path.relative(root, outFile)}`);
  const go = renderVocabGo();
  mkdirSync(path.dirname(outGoFile), { recursive: true });
  writeFileSync(outGoFile, go);
  console.log(`[gen-vocab] wrote ${path.relative(root, outGoFile)}`);
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  await main();
}
