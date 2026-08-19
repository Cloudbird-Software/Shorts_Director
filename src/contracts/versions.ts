/**
 * 契约常量——schema/ 单一真源在 TS 侧的锚点。
 *
 * 定位：Phase 0 的临时手工锚点。codegen 流水线（make gen）就位后，
 * 本文件内容将由生成代码替换，调用方经 src/index.ts 的导出无感切换。
 * 修改这里必须同步修改 schema/ 下对应文件（tests/contracts.test.ts 会校验漂移）。
 */

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

/** 受控词表 v1（schema/vocab/v1/*.yaml 文件名一一对应，不含 .yaml 后缀） */
export const VOCAB_V1 = [
  "action",
  "audio_role",
  "beat_role",
  "camera_motion",
  "compliance_risk",
  "copy_function",
  "defect_type",
  "overlay_intent",
  "proof_type",
  "remedy_action",
  "scene.food",
  "season",
  "shot_type",
  "subject.food",
  "ttl_class",
] as const;

export type VocabName = (typeof VOCAB_V1)[number];

/** beat_role：7 值冻结表（扩值 = major bump）。等价类见 schema/vocab/v1/beat_role.yaml */
export const beatRole = [
  "HOOK",
  "CONTEXT",
  "PROOF",
  "CONTRAST",
  "OFFER",
  "CTA",
  "BUMPER",
] as const;
export type BeatRole = (typeof beatRole)[number];

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
