// AUTO-GENERATED from schema/vocab/v1/*.yaml by make gen —— 禁止手改。
// 词表演进规则见 schema/AGENTS.md：enum 只允许追加，废弃用 deprecated/replaced_by。
package vocab

// Meta 是词表值的元数据：zh 展示名 / def 定义 / 等价类 / 废弃链。
type Meta struct {
	Zh               string
	Def              string
	EquivalenceClass []string
	Deprecated       bool
	ReplacedBy       *string
}

// VocabFiles 是词表清单（schema/vocab/v1/*.yaml 文件名，不含后缀）。
var VocabFiles = [...]string{"action", "audio_role", "beat_role", "camera_motion", "compliance_risk", "copy_function", "defect_type", "mood", "negative_space", "overlay_intent", "proof_type", "remedy_action", "scene.food", "season", "shot_type", "subject.food", "ttl_class"}

// ── 枚举值清单 ─────────────────────────────────────────────

// Action 是 action 词表的值清单（37 值）。
var Action = [...]string{"CUT", "WASH", "MARINATE", "STIR", "POUR", "SPRINKLE", "SEASON", "WRAP", "ROLL", "KNEAD", "STIR_FRY", "DEEP_FRY", "STEAM", "BOIL", "GRILL", "SIMMER", "PLATE", "GARNISH", "LIFT", "OPEN_LID", "STEAM_RISE", "SERVE", "PACK", "TAKE_ORDER", "CHECKOUT", "UNBOX", "WEIGH", "SMILE", "WAVE", "POINT", "RECOMMEND", "THUMBS_UP", "EAT", "DRINK", "REACT_SURPRISE", "QUEUE", "WALK_THROUGH"}

// AudioRole 是 audio_role 词表的值清单（4 值）。
var AudioRole = [...]string{"VO", "SYNC", "SILENT", "SFX"}

// BeatRole 是 beat_role 词表的值清单（7 值，冻结）。
var BeatRole = [...]string{"HOOK", "CONTEXT", "PROOF", "CONTRAST", "OFFER", "CTA", "BUMPER"}

// CameraMotion 是 camera_motion 词表的值清单（9 值）。
var CameraMotion = [...]string{"STATIC", "PUSH", "PULL", "PAN", "TILT", "TRACK", "HANDHELD", "ORBIT", "WHIP"}

// ComplianceRisk 是 compliance_risk 词表的值清单（16 值）。
var ComplianceRisk = [...]string{"AD_LAW_SUPERLATIVE", "AD_LAW_UNPROVEN_CLAIM", "MEDICAL_CLAIM", "FINANCIAL_PROMISE", "PRICE_FRAUD", "MISSING_QUALIFICATION", "REQUIRED_DISCLAIMER_MISSING", "PORTRAIT_RIGHT", "VOICE_RIGHT", "TRADEMARK", "MUSIC_LICENSE", "THIRD_PARTY_CONTENT", "AIGC_LABEL_REQUIRED", "AI_FACE_MISUSE", "FACT_STALE", "FACT_UNVERIFIABLE"}

// CopyFunction 是 copy_function 词表的值清单（18 值）。
var CopyFunction = [...]string{"QUESTION_HOOK", "NUMBER_HOOK", "COUNTERINTUITIVE_HOOK", "SCENE_HOOK", "RESULT_FIRST", "PAIN_CALLOUT", "ORIGIN_STORY", "CRAFT_STORY", "PERSONA_STORY", "TESTIMONY_QUOTE", "DATA_CITE", "OBJECTION_RESPOND", "COMPARISON", "SCARCITY", "OFFER_CLAIM", "DIRECTION_CTA", "LOCATION_CUE", "SLOGAN"}

// DefectType 是 defect_type 词表的值清单（39 值）。
var DefectType = [...]string{"BLURRY", "SHAKE", "OVEREXPOSED", "UNDEREXPOSED", "FLICKER", "BLACK_FRAME", "FREEZE_FRAME", "AUDIO_CLIPPING", "AUDIO_TOO_QUIET", "NOISY_AUDIO", "SILENT_AUDIO", "RESOLUTION_LOW", "ASPECT_MISMATCH", "SUBJECT_MISSING", "SUBJECT_TOO_SMALL", "SUBJECT_TRUNCATED", "WRONG_SHOT_TYPE", "WRONG_MOTION", "BAD_FRAMING", "NEGATIVE_SPACE_MISSING", "OBSTRUCTED", "BACKGROUND_CLUTTER", "HANDLES_MISSING", "DURATION_SHORT", "WRONG_SCENE", "LIPSYNC_OFF", "FACE_WARP", "HAND_WARP", "TEXT_WARP", "TEMPORAL_WARP", "PLASTIC_LOOK", "IDENTITY_DRIFT", "THIRD_PARTY_FACE", "THIRD_PARTY_LOGO", "BANNED_TERM", "CLAIM_WITHOUT_PROOF", "LICENSE_MUSIC_MISSING", "AIGC_LABEL_MISSING", "PORTRAIT_AUTH_MISSING"}

// Mood 是 mood 词表的值清单（8 值）。
var Mood = [...]string{"WARM", "ENERGETIC", "COZY", "FRESH", "APPETIZING", "PROFESSIONAL", "PLAYFUL", "CALM"}

// NegativeSpace 是 negative_space 词表的值清单（5 值）。
var NegativeSpace = [...]string{"TOP", "BOTTOM", "LEFT", "RIGHT", "NONE"}

// OverlayIntent 是 overlay_intent 词表的值清单（14 值）。
var OverlayIntent = [...]string{"EMPHASIZE_NUMBER", "EMPHASIZE_KEYWORD", "KARAOKE_CAPTION", "STATIC_CAPTION", "ANNOTATE_PART", "ANNOTATE_PROOF", "PROGRESS_STEPS", "COUNTDOWN", "PRICE_TAG", "OFFER_BADGE", "LOCATION_CARD", "LOGO_WATERMARK", "AIGC_DISCLOSURE", "SUBTITLE_FOREIGN"}

// ProofType 是 proof_type 词表的值清单（8 值）。
var ProofType = [...]string{"LIVE_DEMO", "CUSTOMER_TESTIMONY", "QUALIFICATION", "DATA", "BEFORE_AFTER", "SOURCE_TRACE", "CRAFT_DETAIL", "SCENE_ATMOSPHERE"}

// RemedyAction 是 remedy_action 词表的值清单（15 值）。
var RemedyAction = [...]string{"RESHOOT", "REGENERATE", "RE_CROP", "RECOLOR", "AUDIO_FIX", "REPLACE_SHOT", "TRIM", "SPEED_ADJUST", "REWRITE_COPY", "MUTE_AUDIO", "BLUR_MASK", "ADD_DISCLAIMER", "ADD_LABEL", "RE_ORDER", "REJECT_SOURCE"}

// SceneFood 是 scene.food 词表的值清单（14 值）。
var SceneFood = [...]string{"STOREFRONT", "ENTRANCE", "NEIGHBORHOOD", "OUTDOOR_SEATING", "COUNTER", "DELIVERY_COUNTER", "DINING_AREA", "PRIVATE_ROOM", "OPEN_KITCHEN", "KITCHEN", "PREP_STATION", "WASH_STATION", "STORAGE", "SUPPLIER_ARRIVAL"}

// Season 是 season 词表的值清单（6 值）。
var Season = [...]string{"ALL", "SPRING", "SUMMER", "AUTUMN", "WINTER", "FESTIVAL"}

// ShotType 是 shot_type 词表的值清单（8 值）。
var ShotType = [...]string{"EXTREME_CLOSEUP", "CLOSEUP", "MEDIUM", "WIDE", "OTS", "TABLETOP", "POV", "INSERT"}

// SubjectFood 是 subject.food 词表的值清单（24 值）。
var SubjectFood = [...]string{"OWNER", "CHEF", "STAFF", "CUSTOMER", "REGULAR_CUSTOMER", "COURIER", "HANDS", "SIGNAGE", "MENU_BOARD", "QUALIFICATION_CERT", "RECEIPT", "DISH_FINISHED", "DISH_PLATING", "INGREDIENT_RAW", "INGREDIENT_PREPPED", "MEAT", "SEAFOOD", "VEGETABLE", "STAPLE", "SEASONING", "BEVERAGE", "TABLEWARE", "EQUIPMENT", "PACKAGING"}

// TtlClass 是 ttl_class 词表的值清单（4 值）。
var TtlClass = [...]string{"SHORT", "MEDIUM", "LONG", "EVERGREEN"}

// ── 词表元数据 ─────────────────────────────────────────────

var actionMeta = map[string]Meta{
	"CUT":            {Zh: "切", Def: "刀具分割食材", EquivalenceClass: []string{"PREP"}},
	"WASH":           {Zh: "清洗", Def: "水洗食材/器具", EquivalenceClass: []string{"PREP", "HYGIENE"}},
	"MARINATE":       {Zh: "腌制", Def: "加调料腌制静置", EquivalenceClass: []string{"PREP"}},
	"STIR":           {Zh: "搅拌", Def: "搅拌混合（馅料/面糊/饮品）", EquivalenceClass: []string{"PREP"}},
	"POUR":           {Zh: "倒", Def: "倾倒液体/浇汁", EquivalenceClass: []string{"ASSEMBLE"}},
	"SPRINKLE":       {Zh: "撒", Def: "撒放调料/葱花/芝麻等", EquivalenceClass: []string{"ASSEMBLE"}},
	"SEASON":         {Zh: "调味", Def: "加调味料动作（勺/壶）", EquivalenceClass: []string{"ASSEMBLE"}},
	"WRAP":           {Zh: "包裹", Def: "包制（包子/饺子/卷类）", EquivalenceClass: []string{"ASSEMBLE"}},
	"ROLL":           {Zh: "擀卷", Def: "擀制/卷制面食", EquivalenceClass: []string{"ASSEMBLE"}},
	"KNEAD":          {Zh: "揉面", Def: "揉制面团", EquivalenceClass: []string{"PREP"}},
	"STIR_FRY":       {Zh: "翻炒", Def: "锅中翻炒", EquivalenceClass: []string{"COOK"}},
	"DEEP_FRY":       {Zh: "炸", Def: "油炸烹饪", EquivalenceClass: []string{"COOK"}},
	"STEAM":          {Zh: "蒸", Def: "蒸制烹饪", EquivalenceClass: []string{"COOK"}},
	"BOIL":           {Zh: "煮烫", Def: "水煮/汆烫", EquivalenceClass: []string{"COOK"}},
	"GRILL":          {Zh: "煎烤", Def: "煎/烤/烙制", EquivalenceClass: []string{"COOK"}},
	"SIMMER":         {Zh: "炖焖", Def: "炖煮/焖制（汤类/卤味）", EquivalenceClass: []string{"COOK"}},
	"PLATE":          {Zh: "摆盘", Def: "装盘造型", EquivalenceClass: []string{"PRESENT"}},
	"GARNISH":        {Zh: "点缀", Def: "成品上加点缀物", EquivalenceClass: []string{"PRESENT"}},
	"LIFT":           {Zh: "起锅端起", Def: "起锅/端起成品展示", EquivalenceClass: []string{"PRESENT"}},
	"OPEN_LID":       {Zh: "揭盖", Def: "揭开锅盖/蒸笼（蒸汽释放瞬间）", EquivalenceClass: []string{"PRESENT"}},
	"STEAM_RISE":     {Zh: "热气升腾", Def: "无主体的热气/锅气画面（欲望触发物）", EquivalenceClass: []string{"PRESENT"}},
	"SERVE":          {Zh: "上菜递出", Def: "递送/上菜到桌", EquivalenceClass: []string{"SERVICE"}},
	"PACK":           {Zh: "打包", Def: "装盒/封袋打包", EquivalenceClass: []string{"SERVICE"}},
	"TAKE_ORDER":     {Zh: "点单", Def: "接单/推荐点单交互", EquivalenceClass: []string{"SERVICE"}},
	"CHECKOUT":       {Zh: "收银结账", Def: "结账/收款动作", EquivalenceClass: []string{"SERVICE"}},
	"UNBOX":          {Zh: "验货拆箱", Def: "进货验收/拆箱", EquivalenceClass: []string{"SUPPLY"}},
	"WEIGH":          {Zh: "称重", Def: "称量食材/成品", EquivalenceClass: []string{"SUPPLY"}},
	"SMILE":          {Zh: "微笑", Def: "人物微笑表情", EquivalenceClass: []string{"REACTION"}},
	"WAVE":           {Zh: "招呼", Def: "挥手/招呼手势", EquivalenceClass: []string{"REACTION"}},
	"POINT":          {Zh: "指向", Def: "手势指向主体（引导视线）", EquivalenceClass: []string{"REACTION"}},
	"RECOMMEND":      {Zh: "推荐", Def: "推荐/介绍手势与姿态", EquivalenceClass: []string{"REACTION"}},
	"THUMBS_UP":      {Zh: "认可", Def: "点赞/认可手势", EquivalenceClass: []string{"REACTION"}},
	"EAT":            {Zh: "进食", Def: "试吃/进食（反应类内容核心）", EquivalenceClass: []string{"REACTION"}},
	"DRINK":          {Zh: "饮用", Def: "饮用/品鉴", EquivalenceClass: []string{"REACTION"}},
	"REACT_SURPRISE": {Zh: "惊讶", Def: "惊讶/满足等外显情绪反应", EquivalenceClass: []string{"REACTION"}},
	"QUEUE":          {Zh: "排队等候", Def: "顾客排队/候餐（热度证据）", EquivalenceClass: []string{"SOCIAL"}},
	"WALK_THROUGH":   {Zh: "穿行", Def: "人物在场景中走动（运镜跟随时也标注）", EquivalenceClass: []string{"SOCIAL"}},
}

var audio_roleMeta = map[string]Meta{
	"VO":     {Zh: "口播", Def: "主体叙事音频（TTS 或真人），决定 beat 时长下界", EquivalenceClass: []string{"SPEECH"}},
	"SYNC":   {Zh: "同期声", Def: "现场收音（环境声/操作声），增强真实感", EquivalenceClass: []string{"AMBIENT"}},
	"SILENT": {Zh: "静音", Def: "无音频输出（BGM 由平台侧添加时使用，配合卡点标记）", EquivalenceClass: []string{"AMBIENT"}},
	"SFX":    {Zh: "音效", Def: "点缀性音效（切菜声/收银声），不承担叙事", EquivalenceClass: []string{"AMBIENT"}},
}

var beat_roleMeta = map[string]Meta{
	"HOOK":     {Zh: "钩子", Def: "前 3 秒内承担停留率职责的开口，必须制造信息差或冲突", EquivalenceClass: []string{"OPENING"}},
	"CONTEXT":  {Zh: "语境", Def: "交代人物/地点/时间等背景信息，为 PROOF 建立理解前提", EquivalenceClass: []string{"BODY"}},
	"PROOF":    {Zh: "证据", Def: "用可见画面证明主张（演示/对比/证言/资质/数据）", EquivalenceClass: []string{"BODY"}},
	"CONTRAST": {Zh: "对比", Def: "呈现\"没有对比就没有伤害\"，正面回应 audience.objection", EquivalenceClass: []string{"BODY"}},
	"OFFER":    {Zh: "主张", Def: "给出具体的转化主张（价格/活动/权益），与 CTA 分工：OFFER 说\"什么\"，CTA 说\"怎么做\"", EquivalenceClass: []string{"CLOSING"}},
	"CTA":      {Zh: "行动号召", Def: "明确的下一步动作指令，通常在末尾", EquivalenceClass: []string{"CLOSING"}},
	"BUMPER":   {Zh: "尾板", Def: "品牌记忆板（店名/位置/slogan），帧数短、可复用", EquivalenceClass: []string{"CLOSING"}},
}

var camera_motionMeta = map[string]Meta{
	"STATIC":   {Zh: "固定", Def: "机位不动（含手机支架固定），画面内运动仅来自主体", EquivalenceClass: []string{"STABLE"}},
	"PUSH":     {Zh: "推", Def: "向主体推近，制造聚焦/进入感；dir=IN", EquivalenceClass: []string{"AXIAL"}},
	"PULL":     {Zh: "拉", Def: "从主体拉远，交代环境；dir=OUT", EquivalenceClass: []string{"AXIAL"}},
	"PAN":      {Zh: "摇", Def: "机位不动水平转动；dir=LEFT/RIGHT", EquivalenceClass: []string{"ROTATIONAL"}},
	"TILT":     {Zh: "俯仰", Def: "机位不动垂直转动；dir=UP/DOWN", EquivalenceClass: []string{"ROTATIONAL"}},
	"TRACK":    {Zh: "移", Def: "机位平移跟随主体；dir=LEFT/RIGHT", EquivalenceClass: []string{"LATERAL"}},
	"HANDHELD": {Zh: "手持", Def: "手持跟拍，允许轻微晃动（shake_score 阈值放宽）", EquivalenceClass: []string{"LATERAL", "DYNAMIC"}},
	"ORBIT":    {Zh: "环绕", Def: "围绕主体弧形运动", EquivalenceClass: []string{"ROTATIONAL", "DYNAMIC"}},
	"WHIP":     {Zh: "甩", Def: "快速甩镜转场，常接匹配剪辑", EquivalenceClass: []string{"DYNAMIC", "TRANSITION"}},
}

var compliance_riskMeta = map[string]Meta{
	"AD_LAW_SUPERLATIVE":          {Zh: "广告法极限词", Def: "最/第一/国家级等绝对化用语", EquivalenceClass: []string{"AD_LAW"}},
	"AD_LAW_UNPROVEN_CLAIM":       {Zh: "无依据功效宣称", Def: "未经证实的功效/效果宣称", EquivalenceClass: []string{"AD_LAW"}},
	"MEDICAL_CLAIM":               {Zh: "医疗功效表述", Def: "涉及治疗/预防疾病表述（食品高危）", EquivalenceClass: []string{"AD_LAW", "CATEGORY"}},
	"FINANCIAL_PROMISE":           {Zh: "收益承诺", Def: "投资回报类承诺表述", EquivalenceClass: []string{"AD_LAW", "CATEGORY"}},
	"PRICE_FRAUD":                 {Zh: "价格误导", Def: "虚构原价/不实折扣", EquivalenceClass: []string{"AD_LAW"}},
	"MISSING_QUALIFICATION":       {Zh: "类目资质缺失", Def: "类目准入资质未验证（医疗/医美/金融等）", EquivalenceClass: []string{"CATEGORY"}},
	"REQUIRED_DISCLAIMER_MISSING": {Zh: "缺必需声明", Def: "类目要求的声明缺失（如\"效果因人而异\"）", EquivalenceClass: []string{"CATEGORY"}},
	"PORTRAIT_RIGHT":              {Zh: "肖像权", Def: "未授权人脸出镜", EquivalenceClass: []string{"IP_RIGHTS"}},
	"VOICE_RIGHT":                 {Zh: "声音权", Def: "声音克隆授权缺失或撤销", EquivalenceClass: []string{"IP_RIGHTS"}},
	"TRADEMARK":                   {Zh: "商标权", Def: "第三方 logo/品牌标识露出", EquivalenceClass: []string{"IP_RIGHTS"}},
	"MUSIC_LICENSE":               {Zh: "音乐版权", Def: "音轨授权缺失/过期/超范围", EquivalenceClass: []string{"IP_RIGHTS"}},
	"THIRD_PARTY_CONTENT":         {Zh: "三方内容", Def: "未授权引用他人素材/画面", EquivalenceClass: []string{"IP_RIGHTS"}},
	"AIGC_LABEL_REQUIRED":         {Zh: "AIGC 标识义务", Def: "生成内容未按规定标识（显式+隐式）", EquivalenceClass: []string{"AIGC_REG"}},
	"AI_FACE_MISUSE":              {Zh: "深度合成滥用", Def: "生成人脸未显著标识/冒充真人", EquivalenceClass: []string{"AIGC_REG"}},
	"FACT_STALE":                  {Zh: "事实过期", Def: "BrandKernel 事实字段过期未核验（价格/活动/店主变更）", EquivalenceClass: []string{"FACTUAL"}},
	"FACT_UNVERIFIABLE":           {Zh: "事实不可验证", Def: "主张无凭证且无法交叉验证", EquivalenceClass: []string{"FACTUAL"}},
}

var copy_functionMeta = map[string]Meta{
	"QUESTION_HOOK":         {Zh: "提问钩", Def: "以疑问句开头制造信息差", EquivalenceClass: []string{"HOOK"}},
	"NUMBER_HOOK":           {Zh: "数字钩", Def: "以具体数字开场（年限/销量/价格）", EquivalenceClass: []string{"HOOK"}},
	"COUNTERINTUITIVE_HOOK": {Zh: "反常识钩", Def: "挑战常识认知引发好奇", EquivalenceClass: []string{"HOOK"}},
	"SCENE_HOOK":            {Zh: "场景带入钩", Def: "描绘时间地点场景让目标客代入", EquivalenceClass: []string{"HOOK"}},
	"RESULT_FIRST":          {Zh: "结果前置", Def: "先给成品/结果再讲过程", EquivalenceClass: []string{"HOOK", "DESIRE"}},
	"PAIN_CALLOUT":          {Zh: "痛点点名", Def: "点名目标客群的具体困扰", EquivalenceClass: []string{"CONTEXT"}},
	"ORIGIN_STORY":          {Zh: "来源故事", Def: "食材/物料的溯源叙事", EquivalenceClass: []string{"TRUST"}},
	"CRAFT_STORY":           {Zh: "工艺故事", Def: "独家手艺/制作工艺叙事", EquivalenceClass: []string{"TRUST"}},
	"PERSONA_STORY":         {Zh: "人格故事", Def: "店主/店员的人格化叙事", EquivalenceClass: []string{"TRUST"}},
	"TESTIMONY_QUOTE":       {Zh: "证言引用", Def: "引用顾客原话评价", EquivalenceClass: []string{"TRUST"}},
	"DATA_CITE":             {Zh: "数据引用", Def: "引用可核验的经营数据", EquivalenceClass: []string{"TRUST"}},
	"OBJECTION_RESPOND":     {Zh: "抗拒点回应", Def: "正面回应受众顾虑（贵/远/慢）", EquivalenceClass: []string{"CONTRAST"}},
	"COMPARISON":            {Zh: "对比句式", Def: "我家 vs 别家 / 之前 vs 之后", EquivalenceClass: []string{"CONTRAST"}},
	"SCARCITY":              {Zh: "限时限量", Def: "时间/数量受限的稀缺表达（广告法高危，需断言检查）", EquivalenceClass: []string{"CONVERSION"}},
	"OFFER_CLAIM":           {Zh: "权益主张", Def: "具体价格/活动/权益说明", EquivalenceClass: []string{"CONVERSION"}},
	"DIRECTION_CTA":         {Zh: "行动指令", Def: "明确下一步动作（到店/私信/点团购）", EquivalenceClass: []string{"CONVERSION"}},
	"LOCATION_CUE":          {Zh: "位置提示", Def: "位置/营业时间等到达信息", EquivalenceClass: []string{"CONVERSION", "BRAND"}},
	"SLOGAN":                {Zh: "品牌口号", Def: "店铺 slogan 或品牌句", EquivalenceClass: []string{"BRAND"}},
}

var defect_typeMeta = map[string]Meta{
	"BLURRY":                 {Zh: "画面模糊", Def: "清晰度低于阈值（laplacian_var）", EquivalenceClass: []string{"L0_TECHNICAL"}},
	"SHAKE":                  {Zh: "抖动", Def: "运动抖动超标（optical_flow）", EquivalenceClass: []string{"L0_TECHNICAL"}},
	"OVEREXPOSED":            {Zh: "过曝", Def: "高光溢出占比超标", EquivalenceClass: []string{"L0_TECHNICAL"}},
	"UNDEREXPOSED":           {Zh: "欠曝", Def: "暗部死黑占比超标", EquivalenceClass: []string{"L0_TECHNICAL"}},
	"FLICKER":                {Zh: "频闪", Def: "帧间亮度闪烁超标（flicker_index）", EquivalenceClass: []string{"L0_TECHNICAL"}},
	"BLACK_FRAME":            {Zh: "黑帧", Def: "存在黑帧区间（blackdetect）", EquivalenceClass: []string{"L0_TECHNICAL"}},
	"FREEZE_FRAME":           {Zh: "画面冻结", Def: "画面静止区间超标（freezedetect）", EquivalenceClass: []string{"L0_TECHNICAL"}},
	"AUDIO_CLIPPING":         {Zh: "爆音", Def: "真峰值超过 0dBTP", EquivalenceClass: []string{"L0_AUDIO"}},
	"AUDIO_TOO_QUIET":        {Zh: "音量过低", Def: "响度低于目标 LUFS 下界", EquivalenceClass: []string{"L0_AUDIO"}},
	"NOISY_AUDIO":            {Zh: "音频噪声", Def: "信噪比低于阈值", EquivalenceClass: []string{"L0_AUDIO"}},
	"SILENT_AUDIO":           {Zh: "无音频", Def: "声轨为空但声明需同期声", EquivalenceClass: []string{"L0_AUDIO"}},
	"RESOLUTION_LOW":         {Zh: "分辨率不足", Def: "低于 1080x1920 要求", EquivalenceClass: []string{"L0_TECHNICAL"}},
	"ASPECT_MISMATCH":        {Zh: "画幅不符", Def: "非竖版 9:16 源", EquivalenceClass: []string{"L0_TECHNICAL"}},
	"SUBJECT_MISSING":        {Zh: "主体缺失", Def: "要求的主体未检出（L1 判定题）", EquivalenceClass: []string{"L1_CONTENT"}},
	"SUBJECT_TOO_SMALL":      {Zh: "主体过小", Def: "主体面积占比低于 spec 下界", EquivalenceClass: []string{"L1_CONTENT"}},
	"SUBJECT_TRUNCATED":      {Zh: "主体截断", Def: "主体越出画面/裁切后断头断尾", EquivalenceClass: []string{"L1_CONTENT"}},
	"WRONG_SHOT_TYPE":        {Zh: "镜位不符", Def: "实拍镜位与制作令 spec 不符", EquivalenceClass: []string{"L1_CONTENT"}},
	"WRONG_MOTION":           {Zh: "运镜不符", Def: "运镜类型/方向与 spec 不符", EquivalenceClass: []string{"L1_CONTENT"}},
	"BAD_FRAMING":            {Zh: "构图越界", Def: "主体/文字越过安全区或画面边缘", EquivalenceClass: []string{"L1_CONTENT"}},
	"NEGATIVE_SPACE_MISSING": {Zh: "无可用负空间", Def: "画面无可安放字幕的连续低纹理区域", EquivalenceClass: []string{"L1_CONTENT"}},
	"OBSTRUCTED":             {Zh: "遮挡", Def: "前景异物遮挡主体", EquivalenceClass: []string{"L1_CONTENT"}},
	"BACKGROUND_CLUTTER":     {Zh: "背景杂乱", Def: "背景干扰主体辨识（含杂乱后厨/无关人员）", EquivalenceClass: []string{"L1_CONTENT"}},
	"HANDLES_MISSING":        {Zh: "首尾无余量", Def: "动作开始/结束无 handles 余量，无法转场", EquivalenceClass: []string{"L1_CONTENT"}},
	"DURATION_SHORT":         {Zh: "时长不足", Def: "素材可用时长低于 beat duration_range 下界", EquivalenceClass: []string{"L1_CONTENT"}},
	"WRONG_SCENE":            {Zh: "场景不符", Def: "实拍场景与 spec.scene 不符", EquivalenceClass: []string{"L1_CONTENT"}},
	"LIPSYNC_OFF":            {Zh: "口型不同步", Def: "LSE-C/LSE-D 超标（生成物）", EquivalenceClass: []string{"L2_GENERATED"}},
	"FACE_WARP":              {Zh: "面部畸变", Def: "生成面部结构异常", EquivalenceClass: []string{"L2_GENERATED"}},
	"HAND_WARP":              {Zh: "手部畸变", Def: "生成手部结构异常", EquivalenceClass: []string{"L2_GENERATED"}},
	"TEXT_WARP":              {Zh: "文字畸变", Def: "画面内生成文字乱码/变形", EquivalenceClass: []string{"L2_GENERATED"}},
	"TEMPORAL_WARP":          {Zh: "时序扭曲", Def: "帧间运动不连贯（temporal_warp_error）", EquivalenceClass: []string{"L2_GENERATED"}},
	"PLASTIC_LOOK":           {Zh: "塑料感", Def: "无参考质量模型判定\"AI 感\"过强（DOVER/FastVQA）", EquivalenceClass: []string{"L2_GENERATED"}},
	"IDENTITY_DRIFT":         {Zh: "身份漂移", Def: "生成人物与目标身份相似度不足（face_identity_sim）", EquivalenceClass: []string{"L2_GENERATED"}},
	"THIRD_PARTY_FACE":       {Zh: "第三方人脸", Def: "画面含未授权出镜的第三人", EquivalenceClass: []string{"L3_COMPLIANCE"}},
	"THIRD_PARTY_LOGO":       {Zh: "第三方标识", Def: "画面含竞品/第三方 logo、招牌、车牌", EquivalenceClass: []string{"L3_COMPLIANCE"}},
	"BANNED_TERM":            {Zh: "违禁表达", Def: "文案/OCR 含违禁词或隐含功效宣称", EquivalenceClass: []string{"L3_COMPLIANCE"}},
	"CLAIM_WITHOUT_PROOF":    {Zh: "无证据宣称", Def: "主张无可视觉验证的证据支撑", EquivalenceClass: []string{"L3_COMPLIANCE"}},
	"LICENSE_MUSIC_MISSING":  {Zh: "音乐无授权", Def: "音轨缺少有效授权凭证或已过期", EquivalenceClass: []string{"L3_COMPLIANCE"}},
	"AIGC_LABEL_MISSING":     {Zh: "缺 AIGC 标识", Def: "缺少显式/隐式 AIGC 标识（GB 45438-2025）", EquivalenceClass: []string{"L3_COMPLIANCE"}},
	"PORTRAIT_AUTH_MISSING":  {Zh: "缺肖像授权", Def: "出镜人授权缺失或已撤销", EquivalenceClass: []string{"L3_COMPLIANCE"}},
}

var moodMeta = map[string]Meta{
	"WARM":         {Zh: "温暖", Def: "暖光/烟火气/家常感", EquivalenceClass: []string{"POSITIVE"}},
	"ENERGETIC":    {Zh: "热闹", Def: "快节奏/人群/市井喧闹", EquivalenceClass: []string{"POSITIVE"}},
	"COZY":         {Zh: "惬意", Def: "慢节奏/私密/治愈感", EquivalenceClass: []string{"POSITIVE"}},
	"FRESH":        {Zh: "清新", Def: "冷调/明亮/食材本味感", EquivalenceClass: []string{"POSITIVE"}},
	"APPETIZING":   {Zh: "馋人", Def: "高饱和/特写食欲诱发", EquivalenceClass: []string{"POSITIVE"}},
	"PROFESSIONAL": {Zh: "专业", Def: "工艺流程/匠心/秩序感", EquivalenceClass: []string{"NEUTRAL"}},
	"PLAYFUL":      {Zh: "俏皮", Def: "跳剪/表情包/轻快配乐", EquivalenceClass: []string{"POSITIVE"}},
	"CALM":         {Zh: "安静", Def: "静态构图/环境音/留白", EquivalenceClass: []string{"NEUTRAL"}},
}

var negative_spaceMeta = map[string]Meta{
	"TOP":    {Zh: "顶部", Def: "画面上 1/3 连续空白", EquivalenceClass: []string{"LAYOUT"}},
	"BOTTOM": {Zh: "底部", Def: "画面下 1/3 连续空白", EquivalenceClass: []string{"LAYOUT"}},
	"LEFT":   {Zh: "左侧", Def: "画面左 1/3 连续空白", EquivalenceClass: []string{"LAYOUT"}},
	"RIGHT":  {Zh: "右侧", Def: "画面右 1/3 连续空白", EquivalenceClass: []string{"LAYOUT"}},
	"NONE":   {Zh: "无", Def: "无满足面积阈值的空白区（字幕只能压画面）", EquivalenceClass: []string{"LAYOUT"}},
}

var overlay_intentMeta = map[string]Meta{
	"EMPHASIZE_NUMBER":  {Zh: "强调数字", Def: "放大关键数字（价格/年限/销量）", EquivalenceClass: []string{"EMPHASIS"}},
	"EMPHASIZE_KEYWORD": {Zh: "强调关键词", Def: "高亮文案关键词", EquivalenceClass: []string{"EMPHASIS"}},
	"KARAOKE_CAPTION":   {Zh: "逐字字幕", Def: "词级同步字幕（依赖 word_timings）", EquivalenceClass: []string{"CAPTION"}},
	"STATIC_CAPTION":    {Zh: "静态字幕", Def: "整句字幕块", EquivalenceClass: []string{"CAPTION"}},
	"ANNOTATE_PART":     {Zh: "标注部位", Def: "指向画面部位/部件的标注", EquivalenceClass: []string{"ANNOTATE"}},
	"ANNOTATE_PROOF":    {Zh: "标注证据", Def: "为证照/检测报告等证据加注", EquivalenceClass: []string{"ANNOTATE", "TRUST"}},
	"PROGRESS_STEPS":    {Zh: "步骤进度", Def: "工艺步骤进度条", EquivalenceClass: []string{"STRUCTURE"}},
	"COUNTDOWN":         {Zh: "倒计时", Def: "活动倒计时（配 SCARCITY 文案）", EquivalenceClass: []string{"STRUCTURE", "CONVERSION"}},
	"PRICE_TAG":         {Zh: "价格牌", Def: "价格/团购价标牌", EquivalenceClass: []string{"CONVERSION"}},
	"OFFER_BADGE":       {Zh: "权益角标", Def: "活动/权益角标", EquivalenceClass: []string{"CONVERSION"}},
	"LOCATION_CARD":     {Zh: "位置卡片", Def: "店名+位置+营业时间卡片", EquivalenceClass: []string{"CONVERSION", "BRAND"}},
	"LOGO_WATERMARK":    {Zh: "品牌水印", Def: "店标水印（安全区常驻）", EquivalenceClass: []string{"BRAND"}},
	"AIGC_DISCLOSURE":   {Zh: "AIGC 标识", Def: "合规显式标识（ComplianceGate 强制注入，禁止 StyleTheme 覆盖）", EquivalenceClass: []string{"COMPLIANCE"}},
	"SUBTITLE_FOREIGN":  {Zh: "译制字幕", Def: "外语字幕轨（预留）", EquivalenceClass: []string{"CAPTION"}},
}

var proof_typeMeta = map[string]Meta{
	"LIVE_DEMO":          {Zh: "现场演示", Def: "拍摄过程本身作为证据（现场切配/现炒/现做）", EquivalenceClass: []string{"PROCESS"}},
	"CUSTOMER_TESTIMONY": {Zh: "顾客证言", Def: "真实顾客出镜评价（需肖像授权）", EquivalenceClass: []string{"SOCIAL"}},
	"QUALIFICATION":      {Zh: "资质证书", Def: "证照/检测报告/授权书等文件画面", EquivalenceClass: []string{"DOCUMENT"}},
	"DATA":               {Zh: "数据", Def: "可核验的数字（销量/年限/复购率），来源需可追溯", EquivalenceClass: []string{"DOCUMENT"}},
	"BEFORE_AFTER":       {Zh: "前后对比", Def: "同主体处理前后对比（注意广告法风险，禁止夸大）", EquivalenceClass: []string{"CONTRAST"}},
	"SOURCE_TRACE":       {Zh: "溯源", Def: "食材/物料来源的现场证据（进货单/农场/冷链）", EquivalenceClass: []string{"DOCUMENT", "PROCESS"}},
	"CRAFT_DETAIL":       {Zh: "工艺细节", Def: "独家手艺/工艺的细节特写", EquivalenceClass: []string{"PROCESS"}},
	"SCENE_ATMOSPHERE":   {Zh: "现场氛围", Def: "排队/满座/出餐节奏等现场热度画面", EquivalenceClass: []string{"SOCIAL"}},
}

var remedy_actionMeta = map[string]Meta{
	"RESHOOT":        {Zh: "重拍", Def: "重新拍摄该镜头（返修首选，配构图示意图与反例图）", EquivalenceClass: []string{"MANUAL"}},
	"REGENERATE":     {Zh: "重新生成", Def: "重新生成该生成物（换 seed/降语速）", EquivalenceClass: []string{"AUTO_RETRY"}},
	"RE_CROP":        {Zh: "重新裁切", Def: "调整裁切窗口重新构图（含自动重定位 auto_crop_recenter）", EquivalenceClass: []string{"AUTO_FIX"}},
	"RECOLOR":        {Zh: "调色", Def: "曝光/色彩修正（LUT/曲线）", EquivalenceClass: []string{"AUTO_FIX"}},
	"AUDIO_FIX":      {Zh: "音频修复", Def: "降噪/增益/补录", EquivalenceClass: []string{"AUTO_FIX", "MANUAL"}},
	"REPLACE_SHOT":   {Zh: "换镜头", Def: "用素材池中其他候选 shot 替换", EquivalenceClass: []string{"AUTO_FIX"}},
	"TRIM":           {Zh: "裁剪时长", Def: "调整入出点", EquivalenceClass: []string{"AUTO_FIX"}},
	"SPEED_ADJUST":   {Zh: "变速", Def: "调整播放速度（上限 1.15x，工艺参数见 craft_params）", EquivalenceClass: []string{"AUTO_FIX"}},
	"REWRITE_COPY":   {Zh: "改写文案", Def: "绕开违禁表达/修正事实性错误后重写", EquivalenceClass: []string{"AUTO_RETRY"}},
	"MUTE_AUDIO":     {Zh: "静音处理", Def: "去除问题音轨（改走卡点标记模式）", EquivalenceClass: []string{"AUTO_FIX"}},
	"BLUR_MASK":      {Zh: "打码", Def: "对第三方人脸/logo 做局部模糊或贴图遮盖", EquivalenceClass: []string{"AUTO_FIX", "MANUAL"}},
	"ADD_DISCLAIMER": {Zh: "补声明", Def: "补必需声明（如\"效果因人而异\"）", EquivalenceClass: []string{"AUTO_FIX"}},
	"ADD_LABEL":      {Zh: "补标识", Def: "补 AIGC 显式/隐式标识", EquivalenceClass: []string{"AUTO_FIX"}},
	"RE_ORDER":       {Zh: "重排", Def: "调整 beat/clip 顺序", EquivalenceClass: []string{"AUTO_FIX"}},
	"REJECT_SOURCE":  {Zh: "拒收素材", Def: "素材不可修复（黑帧/损坏/分辨率不足），报废不入库", EquivalenceClass: []string{"AUTO_FIX"}},
}

var scene_foodMeta = map[string]Meta{
	"STOREFRONT":       {Zh: "门头", Def: "店铺正面外观与招牌", EquivalenceClass: []string{"EXTERNAL"}},
	"ENTRANCE":         {Zh: "出入口", Def: "进出通道、迎宾位", EquivalenceClass: []string{"EXTERNAL", "TRANSIT"}},
	"NEIGHBORHOOD":     {Zh: "社区周边", Def: "店铺所在街区环境（通勤人流/邻里氛围）", EquivalenceClass: []string{"EXTERNAL"}},
	"OUTDOOR_SEATING":  {Zh: "外摆区", Def: "户外座位区", EquivalenceClass: []string{"EXTERNAL", "FRONT_OF_HOUSE"}},
	"COUNTER":          {Zh: "前台", Def: "收银/点单/出餐前台", EquivalenceClass: []string{"FRONT_OF_HOUSE"}},
	"DELIVERY_COUNTER": {Zh: "外卖档口", Def: "外卖取餐与打包区", EquivalenceClass: []string{"FRONT_OF_HOUSE"}},
	"DINING_AREA":      {Zh: "堂食区", Def: "顾客用餐区域", EquivalenceClass: []string{"FRONT_OF_HOUSE"}},
	"PRIVATE_ROOM":     {Zh: "包间", Def: "独立包房", EquivalenceClass: []string{"FRONT_OF_HOUSE"}},
	"OPEN_KITCHEN":     {Zh: "明档", Def: "顾客可见的开放操作档口", EquivalenceClass: []string{"BACK_OF_HOUSE", "FRONT_OF_HOUSE"}},
	"KITCHEN":          {Zh: "后厨", Def: "封闭厨房（炉灶/炒锅区）", EquivalenceClass: []string{"BACK_OF_HOUSE"}},
	"PREP_STATION":     {Zh: "备餐台", Def: "切配/组装/摆盘操作台", EquivalenceClass: []string{"BACK_OF_HOUSE"}},
	"WASH_STATION":     {Zh: "清洗区", Def: "食材清洗/餐具洗消区", EquivalenceClass: []string{"BACK_OF_HOUSE"}},
	"STORAGE":          {Zh: "仓储区", Def: "冷库/货架/物料存储", EquivalenceClass: []string{"BACK_OF_HOUSE"}},
	"SUPPLIER_ARRIVAL": {Zh: "进货到货", Def: "收货/验货现场（清晨进货场景）", EquivalenceClass: []string{"BACK_OF_HOUSE", "TRANSIT"}},
}

var seasonMeta = map[string]Meta{
	"ALL":      {Zh: "全季", Def: "全年可用（默认值）", EquivalenceClass: []string{"EVERGREEN"}},
	"SPRING":   {Zh: "春", Def: "3–5 月（含春季时令食材/氛围）", EquivalenceClass: []string{"SEASONAL"}},
	"SUMMER":   {Zh: "夏", Def: "6–8 月（冷饮/夜市/冰品）", EquivalenceClass: []string{"SEASONAL"}},
	"AUTUMN":   {Zh: "秋", Def: "9–11 月（秋季时令/贴秋膘）", EquivalenceClass: []string{"SEASONAL"}},
	"WINTER":   {Zh: "冬", Def: "12–2 月（火锅/热饮/年末）", EquivalenceClass: []string{"SEASONAL"}},
	"FESTIVAL": {Zh: "节庆", Def: "特定节庆窗口（春节/中秋等，配 campaign 绑定）", EquivalenceClass: []string{"SEASONAL", "EVENT"}},
}

var shot_typeMeta = map[string]Meta{
	"EXTREME_CLOSEUP": {Zh: "大特写", Def: "主体占画面 >70%，用于质感/细节", EquivalenceClass: []string{"DETAIL"}},
	"CLOSEUP":         {Zh: "特写", Def: "主体占画面 30%–70%，面部或单一主体充满视觉中心", EquivalenceClass: []string{"DETAIL", "FACE"}},
	"MEDIUM":          {Zh: "中景", Def: "主体腰部以上或操作台面全貌，动作可辨认", EquivalenceClass: []string{"SUBJECT"}},
	"WIDE":            {Zh: "全景", Def: "环境与主体关系完整可见，交代场所", EquivalenceClass: []string{"CONTEXT", "ESTABLISHING"}},
	"OTS":             {Zh: "过肩/第一视角", Def: "过肩或第一视角，制造代入感", EquivalenceClass: []string{"SUBJECT", "POV"}},
	"TABLETOP":        {Zh: "俯拍平铺", Def: "垂直俯拍桌面，适合成品/食材平铺陈列", EquivalenceClass: []string{"DETAIL", "PRODUCT"}},
	"POV":             {Zh: "主观视角", Def: "模拟观众视角（递给 viewer/端起碗），强代入", EquivalenceClass: []string{"POV"}},
	"INSERT":          {Zh: "插入镜头", Def: "补充细节的切入镜头（价签/手部动作/成品切面）", EquivalenceClass: []string{"DETAIL"}},
}

var subject_foodMeta = map[string]Meta{
	"OWNER":              {Zh: "店主", Def: "店主本人出镜（人格化叙事核心主体）", EquivalenceClass: []string{"PERSON", "STAFF_ROLE"}},
	"CHEF":               {Zh: "厨师", Def: "后厨烹饪人员", EquivalenceClass: []string{"PERSON", "STAFF_ROLE"}},
	"STAFF":              {Zh: "店员", Def: "前场服务人员", EquivalenceClass: []string{"PERSON", "STAFF_ROLE"}},
	"CUSTOMER":           {Zh: "顾客", Def: "到店顾客（出镜需肖像合规检查）", EquivalenceClass: []string{"PERSON"}},
	"REGULAR_CUSTOMER":   {Zh: "熟客", Def: "可识别的常客（证言类内容主体）", EquivalenceClass: []string{"PERSON"}},
	"COURIER":            {Zh: "骑手", Def: "外卖配送人员", EquivalenceClass: []string{"PERSON"}},
	"HANDS":              {Zh: "手部", Def: "手部动作特写主体（不计入人数，规避肖像问题）", EquivalenceClass: []string{"PERSON_PART"}},
	"SIGNAGE":            {Zh: "招牌", Def: "店名字号/灯箱/门头牌匾", EquivalenceClass: []string{"OBJECT", "BRAND_ASSET"}},
	"MENU_BOARD":         {Zh: "菜单牌", Def: "菜单/价目表/灯箱菜单", EquivalenceClass: []string{"OBJECT", "BRAND_ASSET"}},
	"QUALIFICATION_CERT": {Zh: "证照", Def: "营业执照/食品经营许可等资质文件", EquivalenceClass: []string{"DOCUMENT"}},
	"RECEIPT":            {Zh: "小票价签", Def: "收银小票、手写价签", EquivalenceClass: []string{"DOCUMENT"}},
	"DISH_FINISHED":      {Zh: "成品菜", Def: "制作完成的出品", EquivalenceClass: []string{"FOOD", "PRODUCT"}},
	"DISH_PLATING":       {Zh: "摆盘出品", Def: "摆盘/装盘过程中的出品", EquivalenceClass: []string{"FOOD", "PRODUCT"}},
	"INGREDIENT_RAW":     {Zh: "生鲜食材", Def: "未加工的原始食材", EquivalenceClass: []string{"FOOD", "INGREDIENT"}},
	"INGREDIENT_PREPPED": {Zh: "半成品", Def: "已切配/腌制的加工中食材", EquivalenceClass: []string{"FOOD", "INGREDIENT"}},
	"MEAT":               {Zh: "肉类", Def: "生鲜肉类食材", EquivalenceClass: []string{"FOOD", "INGREDIENT"}},
	"SEAFOOD":            {Zh: "水产", Def: "活鲜/冰鲜水产", EquivalenceClass: []string{"FOOD", "INGREDIENT"}},
	"VEGETABLE":          {Zh: "蔬菜", Def: "蔬菜类食材", EquivalenceClass: []string{"FOOD", "INGREDIENT"}},
	"STAPLE":             {Zh: "主食", Def: "米面/包子/饺子等主食", EquivalenceClass: []string{"FOOD"}},
	"SEASONING":          {Zh: "调料", Def: "调味料及其使用瞬间", EquivalenceClass: []string{"FOOD", "INGREDIENT"}},
	"BEVERAGE":           {Zh: "饮品", Def: "现制饮品/酒水", EquivalenceClass: []string{"FOOD", "PRODUCT"}},
	"TABLEWARE":          {Zh: "餐具", Def: "碗碟筷杯等器皿", EquivalenceClass: []string{"OBJECT"}},
	"EQUIPMENT":          {Zh: "设备器具", Def: "炉灶/蒸箱/料理机等设备", EquivalenceClass: []string{"OBJECT"}},
	"PACKAGING":          {Zh: "餐盒包装", Def: "外卖餐盒/打包袋", EquivalenceClass: []string{"OBJECT", "PRODUCT"}},
}

var ttl_classMeta = map[string]Meta{
	"SHORT":     {Zh: "短时效", Def: "默认 30 天（活动氛围/时事相关）", EquivalenceClass: []string{"DECAYING"}},
	"MEDIUM":    {Zh: "中时效", Def: "默认 90 天（当季菜品/着装场景）", EquivalenceClass: []string{"DECAYING"}},
	"LONG":      {Zh: "长时效", Def: "默认 180 天（工艺/环境/设备）", EquivalenceClass: []string{"DECAYING"}},
	"EVERGREEN": {Zh: "常青", Def: "默认 365 天（门头/证照/人格出镜，仍需年检式复核）", EquivalenceClass: []string{"EVERGREEN"}},
}

// ── 注册表（按词表名取用，供运行期校验/查询助手） ──────────

// VocabIDs 按词表名取值清单。
var VocabIDs = map[string][]string{
	"action":          Action[:],
	"audio_role":      AudioRole[:],
	"beat_role":       BeatRole[:],
	"camera_motion":   CameraMotion[:],
	"compliance_risk": ComplianceRisk[:],
	"copy_function":   CopyFunction[:],
	"defect_type":     DefectType[:],
	"mood":            Mood[:],
	"negative_space":  NegativeSpace[:],
	"overlay_intent":  OverlayIntent[:],
	"proof_type":      ProofType[:],
	"remedy_action":   RemedyAction[:],
	"scene.food":      SceneFood[:],
	"season":          Season[:],
	"shot_type":       ShotType[:],
	"subject.food":    SubjectFood[:],
	"ttl_class":       TtlClass[:],
}

// VocabMeta 按词表名取元数据表。
var VocabMeta = map[string]map[string]Meta{
	"action":          actionMeta,
	"audio_role":      audio_roleMeta,
	"beat_role":       beat_roleMeta,
	"camera_motion":   camera_motionMeta,
	"compliance_risk": compliance_riskMeta,
	"copy_function":   copy_functionMeta,
	"defect_type":     defect_typeMeta,
	"mood":            moodMeta,
	"negative_space":  negative_spaceMeta,
	"overlay_intent":  overlay_intentMeta,
	"proof_type":      proof_typeMeta,
	"remedy_action":   remedy_actionMeta,
	"scene.food":      scene_foodMeta,
	"season":          seasonMeta,
	"shot_type":       shot_typeMeta,
	"subject.food":    subject_foodMeta,
	"ttl_class":       ttl_classMeta,
}

func ptr(s string) *string { return &s }
