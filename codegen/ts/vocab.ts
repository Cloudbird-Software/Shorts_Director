// AUTO-GENERATED from schema/vocab/v1/*.yaml by make gen —— 禁止手改。
// 词表演进规则见 schema/AGENTS.md：enum 只允许追加，废弃用 deprecated/replaced_by。

/** 词表清单（schema/vocab/v1/*.yaml 文件名，不含后缀） */
export const VOCAB_FILES = [
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
export type VocabName = (typeof VOCAB_FILES)[number];

/** action（37 值） */
export const action = [
  "CUT",
  "WASH",
  "MARINATE",
  "STIR",
  "POUR",
  "SPRINKLE",
  "SEASON",
  "WRAP",
  "ROLL",
  "KNEAD",
  "STIR_FRY",
  "DEEP_FRY",
  "STEAM",
  "BOIL",
  "GRILL",
  "SIMMER",
  "PLATE",
  "GARNISH",
  "LIFT",
  "OPEN_LID",
  "STEAM_RISE",
  "SERVE",
  "PACK",
  "TAKE_ORDER",
  "CHECKOUT",
  "UNBOX",
  "WEIGH",
  "SMILE",
  "WAVE",
  "POINT",
  "RECOMMEND",
  "THUMBS_UP",
  "EAT",
  "DRINK",
  "REACT_SURPRISE",
  "QUEUE",
  "WALK_THROUGH",
] as const;
export type Action = (typeof action)[number];

/** audio_role（4 值） */
export const audioRole = ["VO", "SYNC", "SILENT", "SFX"] as const;
export type AudioRole = (typeof audioRole)[number];

/** beat_role（7 值，冻结） */
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

/** camera_motion（9 值） */
export const cameraMotion = [
  "STATIC",
  "PUSH",
  "PULL",
  "PAN",
  "TILT",
  "TRACK",
  "HANDHELD",
  "ORBIT",
  "WHIP",
] as const;
export type CameraMotion = (typeof cameraMotion)[number];

/** compliance_risk（16 值） */
export const complianceRisk = [
  "AD_LAW_SUPERLATIVE",
  "AD_LAW_UNPROVEN_CLAIM",
  "MEDICAL_CLAIM",
  "FINANCIAL_PROMISE",
  "PRICE_FRAUD",
  "MISSING_QUALIFICATION",
  "REQUIRED_DISCLAIMER_MISSING",
  "PORTRAIT_RIGHT",
  "VOICE_RIGHT",
  "TRADEMARK",
  "MUSIC_LICENSE",
  "THIRD_PARTY_CONTENT",
  "AIGC_LABEL_REQUIRED",
  "AI_FACE_MISUSE",
  "FACT_STALE",
  "FACT_UNVERIFIABLE",
] as const;
export type ComplianceRisk = (typeof complianceRisk)[number];

/** copy_function（18 值） */
export const copyFunction = [
  "QUESTION_HOOK",
  "NUMBER_HOOK",
  "COUNTERINTUITIVE_HOOK",
  "SCENE_HOOK",
  "RESULT_FIRST",
  "PAIN_CALLOUT",
  "ORIGIN_STORY",
  "CRAFT_STORY",
  "PERSONA_STORY",
  "TESTIMONY_QUOTE",
  "DATA_CITE",
  "OBJECTION_RESPOND",
  "COMPARISON",
  "SCARCITY",
  "OFFER_CLAIM",
  "DIRECTION_CTA",
  "LOCATION_CUE",
  "SLOGAN",
] as const;
export type CopyFunction = (typeof copyFunction)[number];

/** defect_type（39 值） */
export const defectType = [
  "BLURRY",
  "SHAKE",
  "OVEREXPOSED",
  "UNDEREXPOSED",
  "FLICKER",
  "BLACK_FRAME",
  "FREEZE_FRAME",
  "AUDIO_CLIPPING",
  "AUDIO_TOO_QUIET",
  "NOISY_AUDIO",
  "SILENT_AUDIO",
  "RESOLUTION_LOW",
  "ASPECT_MISMATCH",
  "SUBJECT_MISSING",
  "SUBJECT_TOO_SMALL",
  "SUBJECT_TRUNCATED",
  "WRONG_SHOT_TYPE",
  "WRONG_MOTION",
  "BAD_FRAMING",
  "NEGATIVE_SPACE_MISSING",
  "OBSTRUCTED",
  "BACKGROUND_CLUTTER",
  "HANDLES_MISSING",
  "DURATION_SHORT",
  "WRONG_SCENE",
  "LIPSYNC_OFF",
  "FACE_WARP",
  "HAND_WARP",
  "TEXT_WARP",
  "TEMPORAL_WARP",
  "PLASTIC_LOOK",
  "IDENTITY_DRIFT",
  "THIRD_PARTY_FACE",
  "THIRD_PARTY_LOGO",
  "BANNED_TERM",
  "CLAIM_WITHOUT_PROOF",
  "LICENSE_MUSIC_MISSING",
  "AIGC_LABEL_MISSING",
  "PORTRAIT_AUTH_MISSING",
] as const;
export type DefectType = (typeof defectType)[number];

/** overlay_intent（14 值） */
export const overlayIntent = [
  "EMPHASIZE_NUMBER",
  "EMPHASIZE_KEYWORD",
  "KARAOKE_CAPTION",
  "STATIC_CAPTION",
  "ANNOTATE_PART",
  "ANNOTATE_PROOF",
  "PROGRESS_STEPS",
  "COUNTDOWN",
  "PRICE_TAG",
  "OFFER_BADGE",
  "LOCATION_CARD",
  "LOGO_WATERMARK",
  "AIGC_DISCLOSURE",
  "SUBTITLE_FOREIGN",
] as const;
export type OverlayIntent = (typeof overlayIntent)[number];

/** proof_type（8 值） */
export const proofType = [
  "LIVE_DEMO",
  "CUSTOMER_TESTIMONY",
  "QUALIFICATION",
  "DATA",
  "BEFORE_AFTER",
  "SOURCE_TRACE",
  "CRAFT_DETAIL",
  "SCENE_ATMOSPHERE",
] as const;
export type ProofType = (typeof proofType)[number];

/** remedy_action（14 值） */
export const remedyAction = [
  "RESHOOT",
  "REGENERATE",
  "RE_CROP",
  "RECOLOR",
  "AUDIO_FIX",
  "REPLACE_SHOT",
  "TRIM",
  "SPEED_ADJUST",
  "REWRITE_COPY",
  "MUTE_AUDIO",
  "BLUR_MASK",
  "ADD_DISCLAIMER",
  "ADD_LABEL",
  "RE_ORDER",
] as const;
export type RemedyAction = (typeof remedyAction)[number];

/** scene.food（14 值） */
export const sceneFood = [
  "STOREFRONT",
  "ENTRANCE",
  "NEIGHBORHOOD",
  "OUTDOOR_SEATING",
  "COUNTER",
  "DELIVERY_COUNTER",
  "DINING_AREA",
  "PRIVATE_ROOM",
  "OPEN_KITCHEN",
  "KITCHEN",
  "PREP_STATION",
  "WASH_STATION",
  "STORAGE",
  "SUPPLIER_ARRIVAL",
] as const;
export type SceneFood = (typeof sceneFood)[number];

/** season（6 值） */
export const season = [
  "ALL",
  "SPRING",
  "SUMMER",
  "AUTUMN",
  "WINTER",
  "FESTIVAL",
] as const;
export type Season = (typeof season)[number];

/** shot_type（8 值） */
export const shotType = [
  "EXTREME_CLOSEUP",
  "CLOSEUP",
  "MEDIUM",
  "WIDE",
  "OTS",
  "TABLETOP",
  "POV",
  "INSERT",
] as const;
export type ShotType = (typeof shotType)[number];

/** subject.food（24 值） */
export const subjectFood = [
  "OWNER",
  "CHEF",
  "STAFF",
  "CUSTOMER",
  "REGULAR_CUSTOMER",
  "COURIER",
  "HANDS",
  "SIGNAGE",
  "MENU_BOARD",
  "QUALIFICATION_CERT",
  "RECEIPT",
  "DISH_FINISHED",
  "DISH_PLATING",
  "INGREDIENT_RAW",
  "INGREDIENT_PREPPED",
  "MEAT",
  "SEAFOOD",
  "VEGETABLE",
  "STAPLE",
  "SEASONING",
  "BEVERAGE",
  "TABLEWARE",
  "EQUIPMENT",
  "PACKAGING",
] as const;
export type SubjectFood = (typeof subjectFood)[number];

/** ttl_class（4 值） */
export const ttlClass = ["SHORT", "MEDIUM", "LONG", "EVERGREEN"] as const;
export type TtlClass = (typeof ttlClass)[number];
