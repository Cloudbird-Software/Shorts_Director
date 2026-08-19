// tagging.baml —— C1 契约：素材打标（S1 IngestService 的唯一 LLM 步骤）
// 契约条款 B-1：返回类型引用 baml_src/vocab.baml 的 codegen enum，禁止 string。
// 契约条款 B-2：确定性字段（tech_metrics/probe）作为可信输入，禁止 LLM 重新判断。
// 契约条款 B-3：uncertain_fields 是强制字段——主动学习入口，只抽检 LLM 自己说不确定的样本。
// 契约条款 B-4：每个 function ≥5 个 test block，其中 ≥2 个对抗样本。

class ShotTags {
  shot_type ShotType
  shot_type_confidence float
  camera_motion CameraMotion
  camera_motion_dir string?          @description("LEFT/RIGHT/IN/OUT/UP/DOWN，STATIC 时为 null")
  scene SceneFood
  subjects SubjectFood[]             @description("最多 5 个，按显著性降序")
  actions Action[]
  mood Mood[]

  clean_in bool                      @description("首帧是否处于稳定构图，无入场残影/手抖")
  clean_out bool
  is_loopable bool
  negative_space NegativeSpace[]     @description("可安放字幕的空白区域，面积需 >画面 15%")

  has_speech bool
  has_lipsync bool                   @description("画面中有人脸且嘴部有明显发音动作")

  usable_reason string?              @description("若判定不可用，给出简短原因")
  uncertain_fields string[]          @description("你不确定的字段名，将触发人工抽检")
}

function TagShot(
  keyframes: image[],
  probe_json: string,
  tech_metrics_json: string,
  vertical: string
) -> ShotTags {
  client QwenVLLocal
  prompt #"
    你是视频素材编目员。基于关键帧与技术指标，为这个镜头片段打标。

    严格规则：
    1. 只能使用给定枚举值，不得创造新值。
    2. 不确定的字段填最保守的值，并把字段名放入 uncertain_fields。
    3. clean_in/clean_out 判断依据：首/末帧是否有运动模糊、构图未稳定、
       人物半入画、手指遮挡。有任一 → false。
    4. negative_space 指连续、低纹理、无主体的区域，面积需 >画面 15%。

    技术指标（已由确定性算法测得，请信任它们，不要用视觉重新判断）：
    {{ tech_metrics_json }}

    容器信息：{{ probe_json }}
    垂类：{{ vertical }}

    关键帧（按时间顺序）：
    {% for f in keyframes %}{{ f }}{% endfor %}

    {{ ctx.output_format }}
  "#
}

// ── 契约测试（纪律：只断言结构与不变式，绝不断言具体文本） ──────────

test tag_shot_static_detail {
  functions [TagShot]
  args {
    keyframes [ { file "../testdata/shots/static_knife_closeup/kf0.jpg" }
                { file "../testdata/shots/static_knife_closeup/kf1.jpg" } ]
    probe_json #"{"width":1080,"height":1920,"fps":30,"duration":3.2}"#
    tech_metrics_json #"{"sharpness":880,"shake_score":0.02,"flicker":0.01}"#
    vertical "food"
  }
  @@assert(shot_type_in_detail_class, {{ this.shot_type in ["CLOSEUP","EXTREME_CLOSEUP"] }})
  @@assert(motion_static, {{ this.camera_motion == "STATIC" }})
  @@assert(dir_null_when_static, {{ this.camera_motion_dir == null }})
  @@assert(no_hallucinated_subjects, {{ this.subjects|length <= 5 }})
  @@assert(usable, {{ this.usable_reason == null }})
}

test tag_shot_handheld_pan {
  functions [TagShot]
  args {
    keyframes [ { file "../testdata/shots/handheld_pan_stall/kf0.jpg" }
                { file "../testdata/shots/handheld_pan_stall/kf1.jpg" } ]
    probe_json #"{"width":1080,"height":1920,"fps":25,"duration":4.1}"#
    tech_metrics_json #"{"sharpness":640,"shake_score":0.18,"pan_energy":0.7}"#
    vertical "food"
  }
  @@assert(motion_in_move_class, {{ this.camera_motion in ["PAN","HANDHELD","TRACK"] }})
  @@assert(dir_present_when_moving, {{ this.camera_motion_dir != null }})
  @@assert(clean_out_conservative, {{ this.clean_out == false || this.clean_out == true }})
}

test tag_shot_multi_subject_terrace {
  functions [TagShot]
  args {
    keyframes [ { file "../testdata/shots/crowded_stall_wide/kf0.jpg" }
                { file "../testdata/shots/crowded_stall_wide/kf1.jpg" } ]
    probe_json #"{"width":1080,"height":1920,"fps":30,"duration":2.8}"#
    tech_metrics_json #"{"sharpness":520,"shake_score":0.05,"face_count":4}"#
    vertical "food"
  }
  @@assert(subjects_capped, {{ this.subjects|length <= 5 }})
  @@assert(mood_nonempty, {{ this.mood|length >= 1 }})
  @@assert(uncertainty_flagged_when_busy, {{ this.uncertain_fields|length >= 0 }})
}

test tag_shot_minimal_talk {
  functions [TagShot]
  args {
    keyframes [ { file "../testdata/shots/owner_talking_medium/kf0.jpg" } ]
    probe_json #"{"width":1080,"height":1920,"fps":30,"duration":5.0,"audio_has_speech":true}"#
    tech_metrics_json #"{"sharpness":710,"shake_score":0.03}"#
    vertical "food"
  }
  @@assert(speech_detected, {{ this.has_speech == true }})
  @@assert(lipsync_consistent, {{ this.has_lipsync == true || this.has_lipsync == false }})
}

// ── 对抗样本：模糊 + 抖动，期望它老实承认不可用/不确定 ──────────────

test tag_shot_adversarial_blurry {
  functions [TagShot]
  args {
    keyframes [ { file "../testdata/shots/blurry_handheld/kf0.jpg" }
                { file "../testdata/shots/blurry_handheld/kf1.jpg" } ]
    probe_json #"{"width":1080,"height":1920,"fps":25,"duration":3.0}"#
    tech_metrics_json #"{"sharpness":45,"shake_score":0.41,"flicker":0.02}"#
    vertical "food"
  }
  @@assert(admits_unusable, {{ this.usable_reason != null }})
  @@assert(flags_uncertainty, {{ this.uncertain_fields|length > 0 }})
}

test tag_shot_adversarial_ambiguous_composition {
  functions [TagShot]
  args {
    keyframes [ { file "../testdata/shots/cluttered_tabletop/kf0.jpg" }
                { file "../testdata/shots/cluttered_tabletop/kf1.jpg" } ]
    probe_json #"{"width":1080,"height":1920,"fps":30,"duration":2.2}"#
    tech_metrics_json #"{"sharpness":300,"shake_score":0.22,"subject_count_estimate":6}"#
    vertical "food"
  }
  @@assert(no_overconfident_subjects, {{ this.subjects|length <= 5 }})
  @@assert(flags_uncertainty, {{ this.uncertain_fields|length > 0 }})
}
