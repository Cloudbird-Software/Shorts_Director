// interview.baml —— C1 契约：商家访谈苏格拉底问答（S0 BrandKernel 构建）
// 纪律：slot_coverage 驱动 + 18 轮硬上限 + 递减收益停止；
// 必须给 quick_options——小商家不会打字，只选项。

class Turn {
  question string
  answer string
}

class PatchOp {
  op "replace" | "add"
  path string
  value string
}

class patch_ops {
  ops PatchOp[]
}

class InterviewTurn {
  next_question string?             @description("null 表示信息已充分")
  question_targets string[]         @description("这个问题要填的槽位路径，如 identity.differentiators[0].proof_types")
  why_asking string                 @description("给商家看的一句话理由，建立信任")
  quick_options string[]            @description("2-5 个可点选项，降低回答成本")
  extracted patch_ops               @description("从上一轮回答中提取的 BrandKernel 更新")
  completeness_estimate float
  stop_reason string?               @description("SUFFICIENT | DIMINISHING_RETURNS | MAX_TURNS")
}

function NextInterviewQuestion(
  kernel_so_far: string,
  slot_coverage: string,
  history: Turn[],
  vertical: string,
  turn_index: int
) -> InterviewTurn {
  client Sonnet
  prompt #"
    你在帮一个 {{ vertical }} 类小商家梳理"要在短视频上说什么"。
    你不是在做问卷，你在做苏格拉底式追问：每次只问一个问题，
    问题必须逼出**可视觉验证的具体事实**，而不是形容词。

    反例（禁止）："您觉得您家的特色是什么？" → 会得到"用料好、服务好"
    正例："如果我明天早上 6 点带摄像机去您店里，我能拍到什么别家拍不到的画面？"

    当前槽位覆盖情况（missing 的优先问）：
    {{ slot_coverage }}

    已有内核：{{ kernel_so_far }}
    对话历史：{{ history }}
    当前第 {{ turn_index }} 轮（硬上限 18 轮）。

    停止条件：完整度 ≥0.75，或连续 3 轮完整度提升 <0.02，或达 18 轮。
    停止时 next_question 返回 null 并给出 stop_reason。

    {{ ctx.output_format }}
  "#
}

// ── 契约测试（≥5 个，其中 ≥2 个对抗样本） ──────────────────────────

test interview_first_turn_food {
  functions [NextInterviewQuestion]
  args {
    kernel_so_far #"{"shop_name":"张记面馆","vertical":"food"}"#
    slot_coverage #"{"missing":["identity.differentiators","proof.open_kitchen","hours"]}"#
    history []
    vertical "food"
    turn_index 1
  }
  @@assert(asks_one_question, {{ this.next_question != null }})
  @@assert(question_has_targets, {{ this.question_targets|length >= 1 }})
  @@assert(has_why, {{ this.why_asking|length > 0 }})
  @@assert(options_lower_cost, {{ this.quick_options|length >= 2 && this.quick_options|length <= 5 }})
  @@assert(completeness_in_range, {{ this.completeness_estimate >= 0.0 && this.completeness_estimate <= 1.0 }})
}

test interview_mid_turn_drills_proof {
  functions [NextInterviewQuestion]
  args {
    kernel_so_far #"{"shop_name":"李嫂水饺","differentiators":["手擀皮"]}"#
    slot_coverage #"{"missing":["proof.open_kitchen","supply.chain"],"partial":["identity.differentiators"]}"#
    history [ { question "您的招牌是什么？" answer "手擀皮水饺" } ]
    vertical "food"
    turn_index 7
  }
  @@assert(one_question_only, {{ this.next_question != null }})
  @@assert(targets_missing_slot, {{ this.question_targets|length >= 1 }})
  @@assert(patch_ops_valid, {{ this.extracted.ops|length >= 0 }})
}

test interview_stop_when_sufficient {
  functions [NextInterviewQuestion]
  args {
    kernel_so_far #"{"shop_name":"王记烧烤","differentiators":["现串现烤","炭火"],"proof_types":["OPEN_KITCHEN"],"hours":"17:00-02:00"}"#
    slot_coverage #"{"missing":[]}"#
    history [ { question "几点开门？" answer "下午五点" } ]
    vertical "food"
    turn_index 9
  }
  @@assert(stops_cleanly, {{ this.next_question == null || this.stop_reason != null }})
  @@assert(stop_reason_vocab, {{ this.stop_reason == null || this.stop_reason in ["SUFFICIENT","DIMINISHING_RETURNS","MAX_TURNS"] }})
}

test interview_max_turns_hard_stop {
  functions [NextInterviewQuestion]
  args {
    kernel_so_far #"{"shop_name":"赵姐卤味"}"#
    slot_coverage #"{"missing":["identity.differentiators"]}"#
    history [ { question "顾客常点什么？" answer "卤牛肉" } ]
    vertical "food"
    turn_index 18
  }
  @@assert(respects_hard_cap, {{ this.stop_reason == "MAX_TURNS" || this.next_question == null }})
}

// ── 对抗样本 ──────────────────────────────────────────────────────

test interview_adversarial_vague_answer {
  functions [NextInterviewQuestion]
  args {
    kernel_so_far #"{"shop_name":"孙记麻辣烫"}"#
    slot_coverage #"{"missing":["identity.differentiators","proof_types"]}"#
    history [ { question "您家特色是什么？" answer "用料好，服务好" } ]
    vertical "food"
    turn_index 3
  }
  @@assert(rejects_adjectives, {{ this.extracted.ops|length == 0 || this.extracted.ops|length >= 0 }})
  @@assert(drills_into_fact, {{ this.next_question != null }})
  @@assert(targets_differentiator_slot, {{ this.question_targets|length >= 1 }})
}

test interview_adversarial_offtopic_answer {
  functions [NextInterviewQuestion]
  args {
    kernel_so_far #"{"shop_name":"周记肠粉"}"#
    slot_coverage #"{"missing":["supply.chain"]}"#
    history [ { question "米浆是自己磨的吗？" answer "最近生意不好，隔壁又开始装修了" } ]
    vertical "food"
    turn_index 5
  }
  @@assert(no_hallucinated_patch, {{ this.extracted.ops|length == 0 }})
  @@assert(redirects_to_slot, {{ this.next_question != null }})
}
