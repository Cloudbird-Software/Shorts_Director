/**
 * 契约常量——schema/ 单一真源在 TS 侧的锚点。
 *
 * 受控词表已 codegen 化：清单与枚举值一律来自 codegen/ts/vocab.ts（make gen
 * 生成，禁止手改），本文件仅保留尚未生成化的人工锚点（实体/契约版本、画布、
 * 渲染工艺参数）。修改 schema/ 后必须跑 make gen，否则 CI 新鲜度测试会红。
 */

export { VOCAB_FILES as VOCAB_V1, beatRole } from "../../codegen/ts/vocab.js";
export type { BeatRole, VocabName } from "../../codegen/ts/vocab.js";

/** 实体 schema 版本（schema/entities/<name>.schema.json 一一对应） */
export const SCHEMA_VERSIONS = {
  brandKernel: "brand_kernel/1",
  shot: "shot/1",
  asset: "asset/1",
  shotSlotQuery: "shot_slot_query/1",
  videoPlan: "video_plan/1",
  qcAssertion: "qc_assertion/1",
  productionOrder: "production_order/1",
  event: "event/1",
} as const;

export type SchemaName = keyof typeof SCHEMA_VERSIONS;

/** 服务间契约版本（schema/contracts/ 下 request/response 的 contract_version） */
export const CONTRACT_VERSIONS = {
  /** C2 控制面 ↔ Python 算子 */
  operator: 1,
  /** C3 控制面 ↔ 渲染器 */
  render: 1,
} as const;

/** 画布硬约束（VideoPlan IR canvas） */
export const CANVAS = {
  width: 1080,
  height: 1920,
  fps: [25, 30] as const,
} as const;

/** 渲染工艺参数（工艺上限集中在常量，可调可测） */
export const RENDER_CRAFT = {
  /** 变速上限：超过有明显不自然感（Engineering_plan §5.5） */
  maxSpeed: 1.15,
  /** 卡点吸附容差（帧） */
  beatSnapToleranceFrames: 3,
  /** 目标响度（LUFS） */
  targetLufs: -14,
} as const;
