// AUTO-GENERATED from schema/vocab/v1/*.yaml by make gen —— 禁止手改。
// 契约条款 B-1：BAML function 的返回类型必须引用本文件 enum，不允许 string。
// 词表演进规则见 schema/AGENTS.md：enum 只允许追加，废弃用 deprecated/replaced_by。

// action（37 值）
enum Action {
  CUT @description("切：刀具分割食材")
  WASH @description("清洗：水洗食材/器具")
  MARINATE @description("腌制：加调料腌制静置")
  STIR @description("搅拌：搅拌混合（馅料/面糊/饮品）")
  POUR @description("倒：倾倒液体/浇汁")
  SPRINKLE @description("撒：撒放调料/葱花/芝麻等")
  SEASON @description("调味：加调味料动作（勺/壶）")
  WRAP @description("包裹：包制（包子/饺子/卷类）")
  ROLL @description("擀卷：擀制/卷制面食")
  KNEAD @description("揉面：揉制面团")
  STIR_FRY @description("翻炒：锅中翻炒")
  DEEP_FRY @description("炸：油炸烹饪")
  STEAM @description("蒸：蒸制烹饪")
  BOIL @description("煮烫：水煮/汆烫")
  GRILL @description("煎烤：煎/烤/烙制")
  SIMMER @description("炖焖：炖煮/焖制（汤类/卤味）")
  PLATE @description("摆盘：装盘造型")
  GARNISH @description("点缀：成品上加点缀物")
  LIFT @description("起锅端起：起锅/端起成品展示")
  OPEN_LID @description("揭盖：揭开锅盖/蒸笼（蒸汽释放瞬间）")
  STEAM_RISE @description("热气升腾：无主体的热气/锅气画面（欲望触发物）")
  SERVE @description("上菜递出：递送/上菜到桌")
  PACK @description("打包：装盒/封袋打包")
  TAKE_ORDER @description("点单：接单/推荐点单交互")
  CHECKOUT @description("收银结账：结账/收款动作")
  UNBOX @description("验货拆箱：进货验收/拆箱")
  WEIGH @description("称重：称量食材/成品")
  SMILE @description("微笑：人物微笑表情")
  WAVE @description("招呼：挥手/招呼手势")
  POINT @description("指向：手势指向主体（引导视线）")
  RECOMMEND @description("推荐：推荐/介绍手势与姿态")
  THUMBS_UP @description("认可：点赞/认可手势")
  EAT @description("进食：试吃/进食（反应类内容核心）")
  DRINK @description("饮用：饮用/品鉴")
  REACT_SURPRISE @description("惊讶：惊讶/满足等外显情绪反应")
  QUEUE @description("排队等候：顾客排队/候餐（热度证据）")
  WALK_THROUGH @description("穿行：人物在场景中走动（运镜跟随时也标注）")
}

// audio_role（4 值）
enum AudioRole {
  VO @description("口播：主体叙事音频（TTS 或真人），决定 beat 时长下界")
  SYNC @description("同期声：现场收音（环境声/操作声），增强真实感")
  SILENT @description("静音：无音频输出（BGM 由平台侧添加时使用，配合卡点标记）")
  SFX @description("音效：点缀性音效（切菜声/收银声），不承担叙事")
}

// beat_role（7 值，冻结）
enum BeatRole {
  HOOK @description("钩子：前 3 秒内承担停留率职责的开口，必须制造信息差或冲突")
  CONTEXT @description("语境：交代人物/地点/时间等背景信息，为 PROOF 建立理解前提")
  PROOF @description("证据：用可见画面证明主张（演示/对比/证言/资质/数据）")
  CONTRAST @description("对比：呈现\"没有对比就没有伤害\"，正面回应 audience.objection")
  OFFER @description("主张：给出具体的转化主张（价格/活动/权益），与 CTA 分工：OFFER 说\"什么\"，CTA 说\"怎么做\"")
  CTA @description("行动号召：明确的下一步动作指令，通常在末尾")
  BUMPER @description("尾板：品牌记忆板（店名/位置/slogan），帧数短、可复用")
}

// camera_motion（9 值）
enum CameraMotion {
  STATIC @description("固定：机位不动（含手机支架固定），画面内运动仅来自主体")
  PUSH @description("推：向主体推近，制造聚焦/进入感；dir=IN")
  PULL @description("拉：从主体拉远，交代环境；dir=OUT")
  PAN @description("摇：机位不动水平转动；dir=LEFT/RIGHT")
  TILT @description("俯仰：机位不动垂直转动；dir=UP/DOWN")
  TRACK @description("移：机位平移跟随主体；dir=LEFT/RIGHT")
  HANDHELD @description("手持：手持跟拍，允许轻微晃动（shake_score 阈值放宽）")
  ORBIT @description("环绕：围绕主体弧形运动")
  WHIP @description("甩：快速甩镜转场，常接匹配剪辑")
}

// compliance_risk（16 值）
enum ComplianceRisk {
  AD_LAW_SUPERLATIVE @description("广告法极限词：最/第一/国家级等绝对化用语")
  AD_LAW_UNPROVEN_CLAIM @description("无依据功效宣称：未经证实的功效/效果宣称")
  MEDICAL_CLAIM @description("医疗功效表述：涉及治疗/预防疾病表述（食品高危）")
  FINANCIAL_PROMISE @description("收益承诺：投资回报类承诺表述")
  PRICE_FRAUD @description("价格误导：虚构原价/不实折扣")
  MISSING_QUALIFICATION @description("类目资质缺失：类目准入资质未验证（医疗/医美/金融等）")
  REQUIRED_DISCLAIMER_MISSING @description("缺必需声明：类目要求的声明缺失（如\"效果因人而异\"）")
  PORTRAIT_RIGHT @description("肖像权：未授权人脸出镜")
  VOICE_RIGHT @description("声音权：声音克隆授权缺失或撤销")
  TRADEMARK @description("商标权：第三方 logo/品牌标识露出")
  MUSIC_LICENSE @description("音乐版权：音轨授权缺失/过期/超范围")
  THIRD_PARTY_CONTENT @description("三方内容：未授权引用他人素材/画面")
  AIGC_LABEL_REQUIRED @description("AIGC 标识义务：生成内容未按规定标识（显式+隐式）")
  AI_FACE_MISUSE @description("深度合成滥用：生成人脸未显著标识/冒充真人")
  FACT_STALE @description("事实过期：BrandKernel 事实字段过期未核验（价格/活动/店主变更）")
  FACT_UNVERIFIABLE @description("事实不可验证：主张无凭证且无法交叉验证")
}

// copy_function（18 值）
enum CopyFunction {
  QUESTION_HOOK @description("提问钩：以疑问句开头制造信息差")
  NUMBER_HOOK @description("数字钩：以具体数字开场（年限/销量/价格）")
  COUNTERINTUITIVE_HOOK @description("反常识钩：挑战常识认知引发好奇")
  SCENE_HOOK @description("场景带入钩：描绘时间地点场景让目标客代入")
  RESULT_FIRST @description("结果前置：先给成品/结果再讲过程")
  PAIN_CALLOUT @description("痛点点名：点名目标客群的具体困扰")
  ORIGIN_STORY @description("来源故事：食材/物料的溯源叙事")
  CRAFT_STORY @description("工艺故事：独家手艺/制作工艺叙事")
  PERSONA_STORY @description("人格故事：店主/店员的人格化叙事")
  TESTIMONY_QUOTE @description("证言引用：引用顾客原话评价")
  DATA_CITE @description("数据引用：引用可核验的经营数据")
  OBJECTION_RESPOND @description("抗拒点回应：正面回应受众顾虑（贵/远/慢）")
  COMPARISON @description("对比句式：我家 vs 别家 / 之前 vs 之后")
  SCARCITY @description("限时限量：时间/数量受限的稀缺表达（广告法高危，需断言检查）")
  OFFER_CLAIM @description("权益主张：具体价格/活动/权益说明")
  DIRECTION_CTA @description("行动指令：明确下一步动作（到店/私信/点团购）")
  LOCATION_CUE @description("位置提示：位置/营业时间等到达信息")
  SLOGAN @description("品牌口号：店铺 slogan 或品牌句")
}

// defect_type（39 值）
enum DefectType {
  BLURRY @description("画面模糊：清晰度低于阈值（laplacian_var）")
  SHAKE @description("抖动：运动抖动超标（optical_flow）")
  OVEREXPOSED @description("过曝：高光溢出占比超标")
  UNDEREXPOSED @description("欠曝：暗部死黑占比超标")
  FLICKER @description("频闪：帧间亮度闪烁超标（flicker_index）")
  BLACK_FRAME @description("黑帧：存在黑帧区间（blackdetect）")
  FREEZE_FRAME @description("画面冻结：画面静止区间超标（freezedetect）")
  AUDIO_CLIPPING @description("爆音：真峰值超过 0dBTP")
  AUDIO_TOO_QUIET @description("音量过低：响度低于目标 LUFS 下界")
  NOISY_AUDIO @description("音频噪声：信噪比低于阈值")
  SILENT_AUDIO @description("无音频：声轨为空但声明需同期声")
  RESOLUTION_LOW @description("分辨率不足：低于 1080x1920 要求")
  ASPECT_MISMATCH @description("画幅不符：非竖版 9:16 源")
  SUBJECT_MISSING @description("主体缺失：要求的主体未检出（L1 判定题）")
  SUBJECT_TOO_SMALL @description("主体过小：主体面积占比低于 spec 下界")
  SUBJECT_TRUNCATED @description("主体截断：主体越出画面/裁切后断头断尾")
  WRONG_SHOT_TYPE @description("镜位不符：实拍镜位与制作令 spec 不符")
  WRONG_MOTION @description("运镜不符：运镜类型/方向与 spec 不符")
  BAD_FRAMING @description("构图越界：主体/文字越过安全区或画面边缘")
  NEGATIVE_SPACE_MISSING @description("无可用负空间：画面无可安放字幕的连续低纹理区域")
  OBSTRUCTED @description("遮挡：前景异物遮挡主体")
  BACKGROUND_CLUTTER @description("背景杂乱：背景干扰主体辨识（含杂乱后厨/无关人员）")
  HANDLES_MISSING @description("首尾无余量：动作开始/结束无 handles 余量，无法转场")
  DURATION_SHORT @description("时长不足：素材可用时长低于 beat duration_range 下界")
  WRONG_SCENE @description("场景不符：实拍场景与 spec.scene 不符")
  LIPSYNC_OFF @description("口型不同步：LSE-C/LSE-D 超标（生成物）")
  FACE_WARP @description("面部畸变：生成面部结构异常")
  HAND_WARP @description("手部畸变：生成手部结构异常")
  TEXT_WARP @description("文字畸变：画面内生成文字乱码/变形")
  TEMPORAL_WARP @description("时序扭曲：帧间运动不连贯（temporal_warp_error）")
  PLASTIC_LOOK @description("塑料感：无参考质量模型判定\"AI 感\"过强（DOVER/FastVQA）")
  IDENTITY_DRIFT @description("身份漂移：生成人物与目标身份相似度不足（face_identity_sim）")
  THIRD_PARTY_FACE @description("第三方人脸：画面含未授权出镜的第三人")
  THIRD_PARTY_LOGO @description("第三方标识：画面含竞品/第三方 logo、招牌、车牌")
  BANNED_TERM @description("违禁表达：文案/OCR 含违禁词或隐含功效宣称")
  CLAIM_WITHOUT_PROOF @description("无证据宣称：主张无可视觉验证的证据支撑")
  LICENSE_MUSIC_MISSING @description("音乐无授权：音轨缺少有效授权凭证或已过期")
  AIGC_LABEL_MISSING @description("缺 AIGC 标识：缺少显式/隐式 AIGC 标识（GB 45438-2025）")
  PORTRAIT_AUTH_MISSING @description("缺肖像授权：出镜人授权缺失或已撤销")
}

// mood（8 值）
enum Mood {
  WARM @description("温暖：暖光/烟火气/家常感")
  ENERGETIC @description("热闹：快节奏/人群/市井喧闹")
  COZY @description("惬意：慢节奏/私密/治愈感")
  FRESH @description("清新：冷调/明亮/食材本味感")
  APPETIZING @description("馋人：高饱和/特写食欲诱发")
  PROFESSIONAL @description("专业：工艺流程/匠心/秩序感")
  PLAYFUL @description("俏皮：跳剪/表情包/轻快配乐")
  CALM @description("安静：静态构图/环境音/留白")
}

// negative_space（5 值）
enum NegativeSpace {
  TOP @description("顶部：画面上 1/3 连续空白")
  BOTTOM @description("底部：画面下 1/3 连续空白")
  LEFT @description("左侧：画面左 1/3 连续空白")
  RIGHT @description("右侧：画面右 1/3 连续空白")
  NONE @description("无：无满足面积阈值的空白区（字幕只能压画面）")
}

// overlay_intent（14 值）
enum OverlayIntent {
  EMPHASIZE_NUMBER @description("强调数字：放大关键数字（价格/年限/销量）")
  EMPHASIZE_KEYWORD @description("强调关键词：高亮文案关键词")
  KARAOKE_CAPTION @description("逐字字幕：词级同步字幕（依赖 word_timings）")
  STATIC_CAPTION @description("静态字幕：整句字幕块")
  ANNOTATE_PART @description("标注部位：指向画面部位/部件的标注")
  ANNOTATE_PROOF @description("标注证据：为证照/检测报告等证据加注")
  PROGRESS_STEPS @description("步骤进度：工艺步骤进度条")
  COUNTDOWN @description("倒计时：活动倒计时（配 SCARCITY 文案）")
  PRICE_TAG @description("价格牌：价格/团购价标牌")
  OFFER_BADGE @description("权益角标：活动/权益角标")
  LOCATION_CARD @description("位置卡片：店名+位置+营业时间卡片")
  LOGO_WATERMARK @description("品牌水印：店标水印（安全区常驻）")
  AIGC_DISCLOSURE @description("AIGC 标识：合规显式标识（ComplianceGate 强制注入，禁止 StyleTheme 覆盖）")
  SUBTITLE_FOREIGN @description("译制字幕：外语字幕轨（预留）")
}

// proof_type（8 值）
enum ProofType {
  LIVE_DEMO @description("现场演示：拍摄过程本身作为证据（现场切配/现炒/现做）")
  CUSTOMER_TESTIMONY @description("顾客证言：真实顾客出镜评价（需肖像授权）")
  QUALIFICATION @description("资质证书：证照/检测报告/授权书等文件画面")
  DATA @description("数据：可核验的数字（销量/年限/复购率），来源需可追溯")
  BEFORE_AFTER @description("前后对比：同主体处理前后对比（注意广告法风险，禁止夸大）")
  SOURCE_TRACE @description("溯源：食材/物料来源的现场证据（进货单/农场/冷链）")
  CRAFT_DETAIL @description("工艺细节：独家手艺/工艺的细节特写")
  SCENE_ATMOSPHERE @description("现场氛围：排队/满座/出餐节奏等现场热度画面")
}

// remedy_action（15 值）
enum RemedyAction {
  RESHOOT @description("重拍：重新拍摄该镜头（返修首选，配构图示意图与反例图）")
  REGENERATE @description("重新生成：重新生成该生成物（换 seed/降语速）")
  RE_CROP @description("重新裁切：调整裁切窗口重新构图（含自动重定位 auto_crop_recenter）")
  RECOLOR @description("调色：曝光/色彩修正（LUT/曲线）")
  AUDIO_FIX @description("音频修复：降噪/增益/补录")
  REPLACE_SHOT @description("换镜头：用素材池中其他候选 shot 替换")
  TRIM @description("裁剪时长：调整入出点")
  SPEED_ADJUST @description("变速：调整播放速度（上限 1.15x，工艺参数见 craft_params）")
  REWRITE_COPY @description("改写文案：绕开违禁表达/修正事实性错误后重写")
  MUTE_AUDIO @description("静音处理：去除问题音轨（改走卡点标记模式）")
  BLUR_MASK @description("打码：对第三方人脸/logo 做局部模糊或贴图遮盖")
  ADD_DISCLAIMER @description("补声明：补必需声明（如\"效果因人而异\"）")
  ADD_LABEL @description("补标识：补 AIGC 显式/隐式标识")
  RE_ORDER @description("重排：调整 beat/clip 顺序")
  REJECT_SOURCE @description("拒收素材：素材不可修复（黑帧/损坏/分辨率不足），报废不入库")
}

// scene.food（14 值）
enum SceneFood {
  STOREFRONT @description("门头：店铺正面外观与招牌")
  ENTRANCE @description("出入口：进出通道、迎宾位")
  NEIGHBORHOOD @description("社区周边：店铺所在街区环境（通勤人流/邻里氛围）")
  OUTDOOR_SEATING @description("外摆区：户外座位区")
  COUNTER @description("前台：收银/点单/出餐前台")
  DELIVERY_COUNTER @description("外卖档口：外卖取餐与打包区")
  DINING_AREA @description("堂食区：顾客用餐区域")
  PRIVATE_ROOM @description("包间：独立包房")
  OPEN_KITCHEN @description("明档：顾客可见的开放操作档口")
  KITCHEN @description("后厨：封闭厨房（炉灶/炒锅区）")
  PREP_STATION @description("备餐台：切配/组装/摆盘操作台")
  WASH_STATION @description("清洗区：食材清洗/餐具洗消区")
  STORAGE @description("仓储区：冷库/货架/物料存储")
  SUPPLIER_ARRIVAL @description("进货到货：收货/验货现场（清晨进货场景）")
}

// season（6 值）
enum Season {
  ALL @description("全季：全年可用（默认值）")
  SPRING @description("春：3–5 月（含春季时令食材/氛围）")
  SUMMER @description("夏：6–8 月（冷饮/夜市/冰品）")
  AUTUMN @description("秋：9–11 月（秋季时令/贴秋膘）")
  WINTER @description("冬：12–2 月（火锅/热饮/年末）")
  FESTIVAL @description("节庆：特定节庆窗口（春节/中秋等，配 campaign 绑定）")
}

// shot_type（8 值）
enum ShotType {
  EXTREME_CLOSEUP @description("大特写：主体占画面 >70%，用于质感/细节")
  CLOSEUP @description("特写：主体占画面 30%–70%，面部或单一主体充满视觉中心")
  MEDIUM @description("中景：主体腰部以上或操作台面全貌，动作可辨认")
  WIDE @description("全景：环境与主体关系完整可见，交代场所")
  OTS @description("过肩/第一视角：过肩或第一视角，制造代入感")
  TABLETOP @description("俯拍平铺：垂直俯拍桌面，适合成品/食材平铺陈列")
  POV @description("主观视角：模拟观众视角（递给 viewer/端起碗），强代入")
  INSERT @description("插入镜头：补充细节的切入镜头（价签/手部动作/成品切面）")
}

// subject.food（24 值）
enum SubjectFood {
  OWNER @description("店主：店主本人出镜（人格化叙事核心主体）")
  CHEF @description("厨师：后厨烹饪人员")
  STAFF @description("店员：前场服务人员")
  CUSTOMER @description("顾客：到店顾客（出镜需肖像合规检查）")
  REGULAR_CUSTOMER @description("熟客：可识别的常客（证言类内容主体）")
  COURIER @description("骑手：外卖配送人员")
  HANDS @description("手部：手部动作特写主体（不计入人数，规避肖像问题）")
  SIGNAGE @description("招牌：店名字号/灯箱/门头牌匾")
  MENU_BOARD @description("菜单牌：菜单/价目表/灯箱菜单")
  QUALIFICATION_CERT @description("证照：营业执照/食品经营许可等资质文件")
  RECEIPT @description("小票价签：收银小票、手写价签")
  DISH_FINISHED @description("成品菜：制作完成的出品")
  DISH_PLATING @description("摆盘出品：摆盘/装盘过程中的出品")
  INGREDIENT_RAW @description("生鲜食材：未加工的原始食材")
  INGREDIENT_PREPPED @description("半成品：已切配/腌制的加工中食材")
  MEAT @description("肉类：生鲜肉类食材")
  SEAFOOD @description("水产：活鲜/冰鲜水产")
  VEGETABLE @description("蔬菜：蔬菜类食材")
  STAPLE @description("主食：米面/包子/饺子等主食")
  SEASONING @description("调料：调味料及其使用瞬间")
  BEVERAGE @description("饮品：现制饮品/酒水")
  TABLEWARE @description("餐具：碗碟筷杯等器皿")
  EQUIPMENT @description("设备器具：炉灶/蒸箱/料理机等设备")
  PACKAGING @description("餐盒包装：外卖餐盒/打包袋")
}

// ttl_class（4 值）
enum TtlClass {
  SHORT @description("短时效：默认 30 天（活动氛围/时事相关）")
  MEDIUM @description("中时效：默认 90 天（当季菜品/着装场景）")
  LONG @description("长时效：默认 180 天（工艺/环境/设备）")
  EVERGREEN @description("常青：默认 365 天（门头/证照/人格出镜，仍需年检式复核）")
}
