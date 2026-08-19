/**
 * 受控词表运行期助手——ShotSlotQuery 取材 / QC 断言 / VLM 打标约束的公共底座。
 *
 * 数据一律来自 codegen（make gen），本文件零自有数据。
 */

import {
  VOCAB_IDS,
  VOCAB_META,
  type VocabMetaEntry,
  type VocabName,
} from "../../codegen/ts/vocab.js";

/** id 是否为该词表合法值（含已废弃值——废弃 ≠ 非法，消费侧自行降级）。 */
export function isVocabId(vocab: VocabName, id: string): boolean {
  return (VOCAB_IDS[vocab] as readonly string[]).includes(id);
}

/** 断言合法值，否则抛错（错误信息带词表名，便于定位脏数据来源）。 */
export function assertVocabId(vocab: VocabName, id: string): void {
  if (!isVocabId(vocab, id)) {
    throw new Error(`[vocab] "${id}" 不是词表 ${vocab} 的合法值`);
  }
}

function metaOf(vocab: VocabName, id: string): VocabMetaEntry {
  assertVocabId(vocab, id);
  // satisfies 保留了字面量窄类型，这里显式按注册表声明收窄到宽类型
  const meta = (VOCAB_META[vocab] as Readonly<Record<string, VocabMetaEntry>>)[
    id
  ];
  if (meta === undefined) {
    // 理论不可达：assertVocabId 已验过 id 合法
    throw new Error(`[vocab] 词表 ${vocab} 缺少 ${id} 的元数据`);
  }
  return meta;
}

/** 是否废弃值。未知 id 直接抛错（调用方先用 assertVocabId 验明来源）。 */
export function isDeprecated(vocab: VocabName, id: string): boolean {
  return metaOf(vocab, id).deprecated;
}

/** 废弃值的继任者；非废弃值返回 null。slot query 按类取材前先查这条。 */
export function replacedBy(vocab: VocabName, id: string): string | null {
  return metaOf(vocab, id).replacedBy;
}

/** 等价类列表：ShotSlotQuery 按"类"而非"精确 id"取材的依据。 */
export function equivalenceClassOf(
  vocab: VocabName,
  id: string,
): readonly string[] {
  return metaOf(vocab, id).equivalenceClass;
}

/** 中文展示名（商家/外包界面用）。 */
export function zhOf(vocab: VocabName, id: string): string {
  return metaOf(vocab, id).zh;
}
