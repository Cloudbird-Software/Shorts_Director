/**
 * RFC 8785（JCS，JSON Canonicalization Scheme）TS 侧实现 + 内容寻址摘要。
 *
 * 与 Go 侧 internal/digest 逐字节一致（A2 公理：跨语言摘要失配 = 内容寻址
 * 失效）。两侧共享向量集 testdata/digest/jcs_vectors.json，Go/TS 各算 digest
 * 对照，进 CI（Freeze Gate G3）。
 *
 * 实现要点（对应 RFC 8785）：
 * - 数值：String(n) 即 ECMAScript Number::toString(10)——JCS §3.2.2.3 直接
 *   引用该算法定义，JS 原生即是规范实现；NaN/Infinity 不是 JSON 数值，拒。
 * - key 排序：Array.prototype.sort 默认比较即 UTF-16 码元序（§3.2.3），
 *   星体平面代理对自然小于 U+E000 起的 BMP 高区——禁止传入本地化比较器。
 * - 字符串：JSON.stringify 的转义规则与 §3.2.2.2 一致（最小化转义，
 *   \b\t\n\f\r\"\\ 与 <0x20 的 \u00xx 小写十六进制）。
 * - 输入只接受 JSON 数据模型（JSON.parse 产物）；不接受 undefined/
 *   bigint/symbol/function（非 JSON 值域，误入即编码错误）。
 */

export class JcsError extends Error {}

function isJsonModel(v: unknown): boolean {
  if (v === null || typeof v === "string" || typeof v === "boolean") {
    return true;
  }
  if (typeof v === "number") {
    return Number.isFinite(v); // NaN/Infinity 拒（含 ±0：合法）
  }
  if (Array.isArray(v)) {
    return v.every(isJsonModel);
  }
  if (typeof v === "object") {
    return Object.values(v).every(isJsonModel);
  }
  return false; // undefined / bigint / symbol / function
}

/** canonicalize 返回值的规范化 JCS 文本（无空白、key 排序、ES 数字）。 */
export function canonicalize(value: unknown): string {
  if (!isJsonModel(value)) {
    throw new JcsError(
      "jcs: 值不在 JSON 数据模型内（NaN/Infinity/undefined/bigint…）",
    );
  }
  return write(value);
}

function write(v: unknown): string {
  if (v === null) return "null";
  switch (typeof v) {
    case "boolean":
      return v ? "true" : "false";
    case "number":
      return esNumber(v);
    case "string":
      return JSON.stringify(v); // 转义规则 = RFC 8785 §3.2.2.2
    case "object":
      if (Array.isArray(v)) {
        return "[" + v.map(write).join(",") + "]";
      }
      // 默认 sort = UTF-16 码元序（RFC 8785 §3.2.3）；不得换成 localeCompare
      return (
        "{" +
        Object.keys(v as Record<string, unknown>)
          .sort()
          .map(
            (k) =>
              JSON.stringify(k) +
              ":" +
              write((v as Record<string, unknown>)[k]),
          )
          .join(",") +
        "}"
      );
    default:
      throw new JcsError(`jcs: 不可达类型 ${typeof v}`);
  }
}

/** esNumber：ES Number::toString(10)；(-0) → "0"。 */
function esNumber(n: number): string {
  if (Object.is(n, -0)) return "0";
  return String(n);
}

/** canonicalizeJsonText 解析 JSON 文本并规范化（与 Go CanonicalizeJSON 对应）。 */
export function canonicalizeJsonText(text: string): string {
  let parsed: unknown;
  try {
    parsed = JSON.parse(text);
  } catch (e) {
    throw new JcsError(`jcs: invalid JSON: ${(e as Error).message}`);
  }
  return canonicalize(parsed);
}

/** contentDigest 返回 "sha256:<64 hex 小写>"（与 Go ContentDigest / schema
 * content_hash pattern ^sha256:[0-9a-f]{64}$ 一致）。 */
export async function contentDigestJsonText(text: string): Promise<string> {
  const canonical = canonicalizeJsonText(text);
  const sum = await crypto.subtle.digest(
    "SHA-256",
    new TextEncoder().encode(canonical),
  );
  const hex = Array.from(new Uint8Array(sum))
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
  return `sha256:${hex}`;
}
