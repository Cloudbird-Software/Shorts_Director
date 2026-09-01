> **归档说明（IR-0007 退役-3/3）**：本工程设计服务于旧意图「实拍素材治理型
> 生产系统」（Freeze Gate 契约冻结路线），其中与「生成一等公民」范式不符的
> 模块（slotquery/planner/brandkernel/shot 治理/compat）已随 IR-0007 退役。
> 现行架构纪律见 [../ARCHITECTURE.md](../ARCHITECTURE.md)，现行规格见
> [../../specs/IR-0007/spec.md](../../specs/IR-0007/spec.md)。本文仅作历史
> 参考，不再是实现依据。

我先核实几个会直接影响架构选型的时效性事实（BAML 的 Go 支持方式、Remotion 授权、OTIO 的语言绑定、AIGC 标识国标编号等），然后给你完整设计。再核实几个直接影响技术选型的点。下面是完整的工程设计文档。我按你提示的顺序推进：**设计公理 → 本体论 → Schema → 契约 → 冻结前契约测试 → 逐服务 Spec → 开源栈 → 路线图**。

有三个地方我先纠正/补充架构层面的判断，因为它们会决定后面所有设计：

1. **OTIO 不能当主 IR。** OTIO 核心是 C++ + 官方 Python binding，据我所知**没有官方 Go 绑定**（你自己确认一下）。而且 OTIO 的模型是"剪辑台时间线"，缺少你需要的"意图/约束/来源"信息。正确做法是：**自研语义 IR 作为主真源，OTIO 只做单向导出**（给人类剪辑师返修用）。
2. **纯 Go 不可能。** VLM/CV/音频分析生态全在 Python。但可以把 Python **降级为"无状态算子（operator）"**——只接受 JSON+文件路径，只输出 JSON，不含任何业务逻辑、不连数据库。这样它是可替换的黑盒，符合"受约束"的精神。
3. **Remotion 不该承担合成，只承担"图层"。** 让 Remotion 渲染带 alpha 的覆盖层（字幕/角标/动效），主视频轨用 FFmpeg 合成。理由：确定性、可缓存、渲染成本、以及 Remotion 按 render 计费。

---

# §0 设计公理（Design Axioms）

这七条是后面所有设计的裁决依据。任何设计争论回到这里判。

| # | 公理 | 强制后果 |
|---|---|---|
| A1 | **意图与产物分离** | 每个视频有两份 artifact：`VideoPlan`（意图 IR，人可读、可 diff）和 `RenderArtifact`（产物）。禁止只存产物。 |
| A2 | **一切非确定性显式化** | LLM 调用、随机选择、模型推理结果，全部落盘为**内容寻址 artifact**（`sha256(输入+模型版本+seed)`）。Planner 本身必须是纯函数。 |
| A3 | **重推理前置，日常纯确定性** | onboarding 阶段允许烧 token；D+1 之后的每日产出禁止调用 VLM/大模型（除文案微调）。写成 lint 规则。 |
| A4 | **约束优先于生成** | 先有硬约束（去重/安全区/合规/预算），再让生成器在可行域内取材。不允许"先生成再检查"。 |
| A5 | **评估必须判定题化** | 任何质量判断都必须表达为 `assertion → bool + 证据`。禁止 1–10 打分作为门禁。 |
| A6 | **契约先冻结，实现后填充** | Schema 与契约先冻结并有测试，服务实现可以是 stub。 |
| A7 | **每客户每月算力预算是硬约束** | Planner 拿不到预算配额就拒绝出方案，而不是超支。 |

---

# §1 本体论（Ontology）

## 1.1 四层分层

这是全系统的心智模型，务必让每个人背下来：

```
L4  意图层  BrandKernel / CampaignGoal / ContentPillar
            —— "这家店是谁，要说什么，对谁说"
L3  范式层  BeatSchema / ShotSlotQuery / CopyFunction / StyleTheme
            —— "怎样的叙事骨架 + 怎样的表现风格"（与具体素材无关）
L2  物料层  Asset / Shot / AudioTrack / VoiceProfile / Overlay
            —— "手上有什么可用的东西"（与具体视频无关）
L1  产物层  MonthlySchedule / VideoPlan(IR) / RenderArtifact / QCReport / Delivery
            —— "今天这一条具体是什么"
```

**关键不变式：L3 永远不引用 L2 的具体实例，只引用"等价类谓词"。** 这是复用性与多样性的来源，也是这套系统区别于"模板库"的根本。一旦某个 BeatSchema 里出现了 `asset_id`，架构就烂了 —— 写成 CI 检查。

## 1.2 实体清单与不变式

### L4 意图层

**`BrandKernel`** — Onboarding 苏格拉底问答的唯一产物，全系统的根契约。

不变式：
- `IV-BK-1`：必须包含至少 3 个 `ContentPillar`，每个 pillar 至少绑定 2 个 `ProofType`。
- `IV-BK-2`：`completeness_score ≥ 0.75` 才允许进入 L3 匹配。（这就是苏格拉底问答的**停止条件** —— 不是"聊够了"，是**槽位覆盖度达标**。）
- `IV-BK-3`：`category` 决定 `ComplianceProfile`，且不可由 LLM 自由生成，必须来自受控枚举。

**`ContentPillar`** — 内容支柱（如"食材溯源""老板人格""顾客证言"）。它是 BeatSchema 检索的主键之一。

**`ComplianceProfile`** — 类目准入 + 禁用词表 + 必需声明 + 是否强制人审。**这是硬门禁实体，不是配置项。**

### L3 范式层

**`BeatSchema`** — 叙事骨架。

```
BeatSchema
├── id, version, vertical[], pillar_affinity[]
├── total_duration_range: [min, max]
├── beats: Beat[]         (有序，2–8 个)
│   └── Beat
│       ├── role: HOOK|CONTEXT|PROOF|CONTRAST|OFFER|CTA|BUMPER
│       ├── duration_range
│       ├── slot_query: ShotSlotQuery     ← 谓词，不是 asset_id
│       ├── copy_function: CopyFunctionRef
│       ├── audio_role: VO|SYNC|SILENT|SFX
│       └── overlay_intents: OverlayIntent[]
├── structural_signature: string          ← 用于"同构判定"
└── diversity_axes: {...}
```

不变式：
- `IV-BS-1`：`beats[0].role == HOOK`，最后一个 beat 的 role ∈ {CTA, OFFER}。
- `IV-BS-2`：`sum(duration_range.min) ≤ total_duration_range.max`（可满足性静态检查）。
- `IV-BS-3`：**`structural_signature` 必须由 `beats[].role` 序列 + 主镜位类型序列确定性生成**，不能手写。这是判定"连续 3 条不得同构"的依据。

**`ShotSlotQuery`** — 这是整套系统里最重要的一个抽象：**把"我要什么样的镜头"表达成可编译成 SQL 的谓词 AST**。

```
ShotSlotQuery
├── must:   Predicate[]      (硬条件，不满足则整个 slot 失败)
├── should: WeightedPredicate[]  (软条件，用于排序打分)
├── forbid: Predicate[]
├── fallback_chain: ShotSlotQuery[]  ← 降级链，关键！
└── consumption_policy: { max_reuse_per_window, cooldown_days }
```

不变式：
- `IV-SQ-1`：`fallback_chain` 末端必须能命中"永远可用"的兜底类（如纯色背景 + Remotion 图形卡）。**任何 slot 都不允许出现"无解"。**
- `IV-SQ-2`：`must` 中只允许出现受控词表内的字段与取值。

**`StyleTheme`** — 表现层，与内容完全解耦。

不变式 `IV-ST-1`：StyleTheme 不得包含任何文案内容，只含样式参数（字体/描边/入场动画/转场/角标位置/配色/节奏系数）。

### L2 物料层

**`Asset`**（上传的原始文件）→ 拆分为 **`Shot`**（可用的最小剪辑单元）。这个区分很重要：一条 40 秒的实拍素材可能产出 6 个 Shot，其中 2 个不可用。

**`Shot`** 的元数据分五组（这是你之前遗漏的字段设计）：

```
Shot
├── identity:     id, asset_id, in_point, out_point, duration
├── semantic:     shot_type, subject[], scene, action, mood        ← 受控词表
├── affordance:   is_loopable, clean_in, clean_out, camera_motion{type,dir},
│                 negative_space[], subject_bbox_track, safe_crop_9x16,
│                 has_speech, has_lipsync, motion_energy
├── technical:    sharpness(laplacian_var), niqe, shake_score,
│                 exposure_hist, color_temp, flicker_score, audio{lufs,snr}
├── compliance:   third_party_faces[], third_party_logos[], plates[],
│                 ocr_text[], risk_flags[]
└── lifecycle:    shot_date, season[], ttl_at, linked_sku[], linked_campaign[],
                  use_count, last_used_at, quality_tier
```

不变式：
- `IV-SH-1`：`safe_crop_9x16.ok == false` 的 shot 不得进入任何 9:16 slot 候选池（除非声明 pillarbox 处理）。
- `IV-SH-2`：`compliance.risk_flags` 非空 → `usable = false`，直到人工放行或自动打码处理完成。
- `IV-SH-3`：`ttl_at < now()` → 自动出候选池并触发 `ReshootTask`。

**`VoiceProfile`** — 声音资产，强制携带 `authorization_id` 与 `revoked_at`。

不变式 `IV-VP-1`：`authorization.status != ACTIVE` → 所有引用该 profile 的未发布 VideoPlan 立即失效。（店长离职场景）

### L1 产物层

**`MonthlySchedule`** — 月度全局排期（背包/装箱问题的解）。
**`VideoPlan`** — 单条视频的语义 IR（详见 §2.2）。
**`ProductionOrder`** — 制作令（发给摄影师/生成商的机器可读规格）。
**`QCReport`** — 断言集的执行结果 + 返修单。
**`Delivery`** — 交付记录 + 商家侧行为事件。

## 1.3 生命周期状态机（必须实现为显式 FSM，不要用布尔字段拼）

**Shot 状态机**
```
UPLOADED → SEGMENTED → TAGGED → QC_L0 
   ├─(fail)→ REJECTED ─→ RESHOOT_REQUESTED
   └─(pass)→ AVAILABLE ⇄ COOLING (被用后进入冷却)
                 ├─→ EXPIRED (ttl)
                 └─→ QUARANTINED (合规/授权撤销)
```

**VideoPlan 状态机**
```
DRAFT → SOLVED (约束求解通过) → COMPILED (IR→渲染指令) 
      → RENDERED → QC_PASSED → COMPLIANCE_PASSED 
      → DELIVERED → {PUBLISHED | EDITED_PUBLISHED | REJECTED_BY_MERCHANT | IGNORED}
```

最后一层四态就是你的**第一飞轮燃料**，必须是一等公民字段，不是日志。

**ProductionOrder 状态机**
```
DRAFTED → DISPATCHED → SUBMITTED → QC_RUNNING 
        → {ACCEPTED | REWORK_REQUESTED(n) | REJECTED}
```
`REWORK_REQUESTED` 必须携带结构化 `RemedyInstruction[]`，这是你和外包之间的**合同界面**。

## 1.4 受控词表（Closed Vocabulary）—— 本体论的核心资产

这是整个系统最该先做、最该先冻结的东西。**它同时是：VLM 打标的输出约束、ShotSlotQuery 的合法取值域、QC 断言的谓词域、制作令的规格语言。** 四个地方共用一套词表，才能形成闭环。

用单一 YAML 真源，生成 Go/TS 常量 + BAML enum + DB enum + JSON Schema。

```yaml
# vocab/v1/shot_type.yaml
version: 1
name: shot_type
kind: enum
values:
  - id: EXTREME_CLOSEUP
    zh: 大特写
    def: 主体占画面 >70%，用于质感/细节
    equivalence_class: [DETAIL]        # slot query 可按等价类取材
  - id: CLOSEUP
    zh: 特写
    equivalence_class: [DETAIL, FACE]
  - id: MEDIUM
    zh: 中景
    equivalence_class: [SUBJECT]
  - id: WIDE
    zh: 全景
    equivalence_class: [CONTEXT, ESTABLISHING]
  - id: OTS            # over-the-shoulder / 第一视角
    zh: 过肩/第一视角
    equivalence_class: [SUBJECT, POV]
  - id: TABLETOP
    zh: 俯拍平铺
    equivalence_class: [DETAIL, PRODUCT]
```

**必须冻结的 14 张词表：**

| 词表 | 用途 | 规模建议 |
|---|---|---|
| `shot_type` | 镜位 | 8–12 |
| `camera_motion` | 静止/推/拉/摇/移/手持/环绕 + 方向 | 8 |
| `scene` | 场景（门店外/前台/厨房/操作台/顾客区…）**按垂类分表** | 每垂类 10–20 |
| `subject` | 主体（人物角色/产品/器具/环境元素） | 每垂类 20–40 |
| `action` | 动作（切/倒/搅拌/递给/微笑/指向…） | 30–50 |
| `beat_role` | 叙事功能 | 7（冻结，不可扩） |
| `copy_function` | 文案功能（提问钩/数字钩/反常识钩/证言/对比/限时…） | 15–25 |
| `overlay_intent` | 覆盖层意图（强调数字/标注部位/进度条/价格牌…） | 12–20 |
| `audio_role` | VO/同期声/静音/SFX | 4 |
| `defect_type` | 缺陷类型（QC 与返修共用） | 25–35 |
| `remedy_action` | 返修动作（重拍/裁切/调色/补音/换镜…） | 12–15 |
| `compliance_risk` | 合规风险类型 | 15–20 |
| `season` / `ttl_class` | 时效 | 6 / 4 |
| `proof_type` | 证明方式（现场演示/顾客证言/资质/数据/对比） | 8 |

> **这 14 张表 = 你的护城河的物质形态。** 别人抄 prompt 容易，抄一套经过真实交付校准的词表 + 等价类映射非常难。

---

# §2 Schema 设计

## 2.1 单一真源策略（Single Source of Truth）

你的技术栈是 Go + 受约束 TS + BAML，跨三种语言。必须有单一真源，否则三边漂移是必然的。

```
schema/                       ← 唯一真源，人手写
├── vocab/*.yaml              ← 受控词表
├── entities/*.cue            ← 实体 schema（推荐 CUE；退路 JSON Schema）
└── contracts/*.proto         ← 服务间 RPC（可选，见 §3.2）

codegen/  (make gen)
├── go/       types + validators + vocab consts
├── ts/       zod schemas + types + vocab consts
├── baml/     enum + class 定义（BAML 侧 import）
├── sql/      migrations (enum types + tables)
└── jsonschema/  用于 artifact 校验与契约测试
```

**为什么推荐 CUE 而不是 JSON Schema 手写**：CUE 能同时表达 schema、约束、默认值、以及**跨字段不变式**（如 `IV-BS-2` 的可满足性检查），并且能直接导出 JSON Schema / Go / OpenAPI。你的不变式很多是跨字段的，纯 JSON Schema 表达不了。

如果团队不想学 CUE，退路是：**JSON Schema 描述结构 + 一个 Go 包 `invariants` 手写跨字段校验，并且 TS 侧通过 WASM 调用同一个 Go 校验器**（避免两份实现漂移）。后者其实更务实。

> **决策建议**：初期用 `JSON Schema (结构) + Go invariants 包 (语义) + TS 侧通过 HTTP 调 Go validator`。等 schema 稳定后再考虑 CUE。理由：AI 辅助开发下，JSON Schema 的生态与模型熟悉度远高于 CUE。

## 2.2 核心 Schema

### 2.2.1 BrandKernel

```jsonc
// schema/entities/brand_kernel.schema.json  (v1)
{
  "$id": "https://x/schemas/v1/brand_kernel.json",
  "type": "object",
  "required": ["schema_version","tenant_id","category","identity",
               "audience","pillars","assets_intent","compliance_profile_id",
               "completeness"],
  "properties": {
    "schema_version": { "const": "brand_kernel/1" },
    "tenant_id": { "type": "string", "format": "uuid" },
    "category": { "$ref": "vocab/business_category.json" },

    "identity": {
      "type": "object",
      "required": ["name","one_liner","differentiators","persona"],
      "properties": {
        "name": { "type": "string" },
        "one_liner": { "type": "string", "maxLength": 40 },
        "differentiators": {                       // 差异点，必须可视觉验证
          "type": "array", "minItems": 2, "maxItems": 5,
          "items": {
            "type": "object",
            "required": ["claim","visual_provable","proof_types"],
            "properties": {
              "claim": { "type": "string", "maxLength": 30 },
              "visual_provable": { "type": "boolean" },
              "proof_types": { "type":"array",
                               "items": { "$ref":"vocab/proof_type.json" },
                               "minItems": 1 }
            }
          }
        },
        "persona": {                               // 口播人格，驱动 TTS 与文风
          "type": "object",
          "properties": {
            "voice_tone": { "enum": ["WARM","EXPERT","STREET","PLAYFUL","CALM"] },
            "speaking_rate": { "enum": ["SLOW","NORMAL","FAST"] },
            "first_person": { "enum": ["OWNER","STAFF","BRAND","CUSTOMER"] },
            "dialect_hint": { "type":"string" }    // 例：轻微川普
          }
        }
      }
    },

    "audience": {
      "type": "object",
      "required": ["segments","local_radius_km","decision_trigger"],
      "properties": {
        "segments": { "type":"array","minItems":1,"maxItems":3,
                      "items": {"type":"object","required":["label","pain","objection"],
                      "properties":{
                        "label":{"type":"string"},
                        "pain":{"type":"string"},
                        "objection":{"type":"string"}   // 关键：抗拒点驱动 CONTRAST beat
                      }}},
        "local_radius_km": { "type":"number" },
        "decision_trigger": { "enum":["IMPULSE","PLANNED","REFERRAL","SEASONAL"] }
      }
    },

    "pillars": {
      "type":"array","minItems":3,"maxItems":6,
      "items": {
        "type":"object",
        "required":["id","label","intent","proof_types","target_ratio"],
        "properties":{
          "id":{"type":"string"},
          "label":{"type":"string"},
          "intent":{"enum":["AWARENESS","TRUST","DESIRE","CONVERSION","RETENTION"]},
          "proof_types":{"type":"array","minItems":2,
                         "items":{"$ref":"vocab/proof_type.json"}},
          "target_ratio":{"type":"number","minimum":0.05,"maximum":0.6}
        }
      }
    },

    "assets_intent": {          // 决定制作令的总盘子
      "type":"object",
      "required":["shootable_scenes","non_shootable_needs","digital_human"],
      "properties":{
        "shootable_scenes":{"type":"array","items":{"$ref":"vocab/scene.json"}},
        "non_shootable_needs":{"type":"array","items":{"type":"string"}},
        "digital_human":{
          "type":"object",
          "properties":{
            "enabled":{"type":"boolean"},
            "source":{"enum":["REAL_OWNER","REAL_STAFF","STOCK_AVATAR"]},
            "authorization_id":{"type":["string","null"]}
          }
        }
      }
    },

    "offers": {                 // 可轮换的 OFFER 池，避免每天同一句
      "type":"array",
      "items":{"type":"object","required":["text","valid_from","valid_to","claim_risk"],
      "properties":{
        "text":{"type":"string"},
        "valid_from":{"type":"string","format":"date"},
        "valid_to":{"type":"string","format":"date"},
        "claim_risk":{"enum":["LOW","MEDIUM","HIGH"]}
      }}
    },

    "hard_negatives": {          // 明确禁止的表达/画面，来自访谈
      "type":"array","items":{"type":"string"}
    },

    "compliance_profile_id": { "type":"string" },

    "completeness": {
      "type":"object",
      "required":["score","missing_slots","interview_turns"],
      "properties":{
        "score":{"type":"number","minimum":0,"maximum":1},
        "missing_slots":{"type":"array","items":{"type":"string"}},
        "interview_turns":{"type":"integer"}
      }
    },

    "provenance": { "$ref": "common/provenance.json" }
  }
}
```

`provenance` 是通用块，**每个 LLM 产出的实体都必须带**（对应公理 A2）：

```jsonc
// common/provenance.json
{
  "type":"object",
  "required":["generated_by","model_id","prompt_version","input_digest","created_at"],
  "properties":{
    "generated_by":{"enum":["LLM","HUMAN","DETERMINISTIC","HYBRID"]},
    "model_id":{"type":"string"},          // e.g. "qwen3-vl-32b@2025-11"
    "prompt_version":{"type":"string"},    // BAML function version
    "input_digest":{"type":"string"},      // sha256 of canonicalized input
    "seed":{"type":["integer","null"]},
    "human_edits":{"type":"array","items":{
        "type":"object","properties":{
          "path":{"type":"string"},"before":{},"after":{},
          "editor":{"type":"string"},"at":{"type":"string","format":"date-time"}}}},
    "created_at":{"type":"string","format":"date-time"}
  }
}
```

`human_edits` 是你飞轮的第二个高密度信号源 —— **JSON Patch 级别的人类修正记录**。

### 2.2.2 ShotSlotQuery（谓词 AST，可编译成 SQL）

这是最需要精心设计的 schema。要点：**受限表达力**，只允许能编译成索引友好 SQL 的形式。

```jsonc
// schema/entities/shot_slot_query.schema.json (v1)
{
  "$id":"https://x/schemas/v1/shot_slot_query.json",
  "definitions": {
    "Field": {
      "enum": [                                  // 白名单，禁止任意字段
        "shot_type","shot_type_class","camera_motion.type","camera_motion.dir",
        "scene","subject","action","mood",
        "is_loopable","clean_in","clean_out","has_speech","has_lipsync",
        "negative_space","safe_crop_9x16.ok","motion_energy",
        "duration","sharpness_tier","quality_tier",
        "season","ttl_at","linked_sku","use_count","last_used_at",
        "source_kind"                            // REAL_SHOT | GEN_AVATAR | GEN_BROLL | STOCK | GRAPHIC
      ]
    },
    "Predicate": {
      "oneOf": [
        { "type":"object","required":["op","field","value"],
          "properties":{
            "op":{"enum":["eq","in","neq","nin"]},
            "field":{"$ref":"#/definitions/Field"},
            "value":{}
          }},
        { "type":"object","required":["op","field","value"],
          "properties":{
            "op":{"enum":["gte","lte","gt","lt"]},
            "field":{"$ref":"#/definitions/Field"},
            "value":{"type":"number"}
          }},
        { "type":"object","required":["op","field","range"],
          "properties":{
            "op":{"const":"between"},
            "field":{"$ref":"#/definitions/Field"},
            "range":{"type":"array","minItems":2,"maxItems":2,"items":{"type":"number"}}
          }},
        { "type":"object","required":["op","query","top_k"],
          "properties":{                          // 语义兜底，仅允许出现在 should
            "op":{"const":"semantic"},
            "query":{"type":"string"},
            "top_k":{"type":"integer","maximum":50}
          }},
        { "type":"object","required":["op","operands"],
          "properties":{
            "op":{"enum":["and","or","not"]},
            "operands":{"type":"array","items":{"$ref":"#/definitions/Predicate"}}
          }}
      ]
    }
  },
  "type":"object",
  "required":["slot_id","must","fallback_chain","consumption_policy"],
  "properties":{
    "slot_id":{"type":"string"},
    "must":{"type":"array","items":{"$ref":"#/definitions/Predicate"}},
    "should":{"type":"array","items":{
        "type":"object","required":["predicate","weight"],
        "properties":{"predicate":{"$ref":"#/definitions/Predicate"},
                      "weight":{"type":"number","minimum":-5,"maximum":5}}}},
    "forbid":{"type":"array","items":{"$ref":"#/definitions/Predicate"}},
    "fallback_chain":{
      "type":"array","minItems":1,
      "items":{"type":"object","required":["level","must","degrade_note"],
        "properties":{
          "level":{"type":"integer"},
          "must":{"type":"array","items":{"$ref":"#/definitions/Predicate"}},
          "degrade_note":{"type":"string"},
          "is_terminal_graphic":{"type":"boolean"}   // 末端兜底：Remotion 图形卡
        }}
    },
    "consumption_policy":{
      "type":"object",
      "required":["cooldown_days","max_uses_per_30d"],
      "properties":{
        "cooldown_days":{"type":"integer","minimum":0},
        "max_uses_per_30d":{"type":"integer","minimum":1},
        "prefer_least_used":{"type":"boolean","default":true}
      }
    }
  }
}
```

**编译目标（Go 侧）**：`Predicate → (sqlFragment, args)`，`semantic` 谓词编译成 pgvector 子查询。因为字段是白名单，这个编译器可以做到零注入风险且完全可测。

### 2.2.3 VideoPlan IR —— 主 IR

这是你系统的心脏。设计原则：**语义完整（能回答"为什么这么剪"）+ 确定性（同一 IR 永远渲出同一帧）+ 可 diff**。

```jsonc
// schema/entities/video_plan.schema.json (v1)
{
  "$id":"https://x/schemas/v1/video_plan.json",
  "type":"object",
  "required":["schema_version","plan_id","tenant_id","scheduled_date",
              "canvas","timebase","beat_schema_ref","style_theme_ref",
              "tracks","copy","audio","overlays","constraints_report",
              "diversity_signature","budget","provenance"],
  "properties": {
    "schema_version": {"const":"video_plan/1"},
    "plan_id":{"type":"string"},
    "tenant_id":{"type":"string"},
    "scheduled_date":{"type":"string","format":"date"},

    "canvas":{"type":"object","required":["w","h","fps","safe_area"],
      "properties":{
        "w":{"const":1080},"h":{"const":1920},"fps":{"enum":[25,30]},
        "safe_area":{                       // 硬约束，来自平台 UI 遮挡
          "type":"object",
          "required":["top","bottom","left","right"],
          "properties":{"top":{"type":"integer"},"bottom":{"type":"integer"},
                        "left":{"type":"integer"},"right":{"type":"integer"}}
        }
      }},

    "timebase":{"type":"object","required":["unit","rate"],
      "properties":{"unit":{"const":"frame"},"rate":{"type":"integer"}}},
      // 关键：所有时间用整数帧，禁止浮点秒。避免舍入不一致。

    "beat_schema_ref":{"$ref":"common/versioned_ref.json"},
    "style_theme_ref":{"$ref":"common/versioned_ref.json"},

    "tracks":{
      "type":"array",
      "items":{
        "type":"object",
        "required":["track_id","kind","clips"],
        "properties":{
          "track_id":{"type":"string"},
          "kind":{"enum":["VIDEO_MAIN","VIDEO_INSERT","OVERLAY_RENDER",
                          "AUDIO_VO","AUDIO_MUSIC","AUDIO_SFX"]},
          "clips":{"type":"array","items":{"$ref":"#/definitions/Clip"}}
        }
      }
    },

    "copy":{
      "type":"object",
      "required":["caption_blocks","post_text","hashtags"],
      "properties":{
        "caption_blocks":{"type":"array","items":{
          "type":"object",
          "required":["block_id","text","start_frame","end_frame",
                      "copy_function","word_timings"],
          "properties":{
            "block_id":{"type":"string"},
            "text":{"type":"string"},
            "start_frame":{"type":"integer"},
            "end_frame":{"type":"integer"},
            "copy_function":{"$ref":"vocab/copy_function.json"},
            "word_timings":{"type":"array","items":{     // 来自 WhisperX/FunASR 强制对齐
              "type":"object","required":["w","s","e"],
              "properties":{"w":{"type":"string"},
                            "s":{"type":"integer"},"e":{"type":"integer"}}}},
            "emphasis":{"type":"array","items":{"type":"integer"}}  // word index
          }}},
        "post_text":{"type":"string"},
        "hashtags":{"type":"array","items":{"type":"string"}}
      }
    },

    "audio":{
      "type":"object",
      "required":["target_lufs","music_ref","vo_ref","ducking"],
      "properties":{
        "target_lufs":{"type":"number","default":-14},
        "music_ref":{"$ref":"common/licensed_ref.json"},   // 必须带授权来源
        "vo_ref":{"type":["object","null"]},
        "ducking":{"type":"object","properties":{
          "enabled":{"type":"boolean"},"amount_db":{"type":"number"}}},
        "beat_grid":{"type":"array","items":{"type":"integer"}} // 卡点帧号
      }
    },

    "overlays":{"type":"array","items":{
      "type":"object",
      "required":["overlay_id","intent","component","props",
                  "start_frame","end_frame","layout_box"],
      "properties":{
        "overlay_id":{"type":"string"},
        "intent":{"$ref":"vocab/overlay_intent.json"},
        "component":{"type":"string"},        // Remotion 组件名（受控白名单）
        "props":{"type":"object"},            // 由该组件的 zod schema 约束
        "start_frame":{"type":"integer"},
        "end_frame":{"type":"integer"},
        "layout_box":{"type":"object",
          "required":["x","y","w","h","anchor"],
          "properties":{"x":{"type":"integer"},"y":{"type":"integer"},
                        "w":{"type":"integer"},"h":{"type":"integer"},
                        "anchor":{"enum":["TL","TC","TR","CL","CC","CR","BL","BC","BR"]}}}
      }}},

    "compliance":{
      "type":"object",
      "required":["aigc_disclosure","checks_passed"],
      "properties":{
        "aigc_disclosure":{
          "type":"object",
          "required":["required","explicit_overlay_id","implicit_metadata"],
          "properties":{
            "required":{"type":"boolean"},
            "explicit_overlay_id":{"type":["string","null"]},
            "implicit_metadata":{"type":"object"}   // 见 §5.8 与待核实事项
          }},
        "checks_passed":{"type":"array","items":{"type":"string"}}
      }},

    "constraints_report":{         // ← 求解过程的可解释性记录，非常重要
      "type":"object",
      "required":["hard_satisfied","soft_scores","fallbacks_used","rejected_candidates"],
      "properties":{
        "hard_satisfied":{"type":"array","items":{"type":"string"}},
        "soft_scores":{"type":"object"},
        "fallbacks_used":{"type":"array","items":{
          "type":"object","properties":{
            "slot_id":{"type":"string"},"level":{"type":"integer"},
            "reason":{"type":"string"}}}},
        "rejected_candidates":{"type":"array","items":{
          "type":"object","properties":{
            "shot_id":{"type":"string"},"rule":{"type":"string"}}}}
      }},

    "diversity_signature":{
      "type":"object",
      "required":["structural","visual_phash_first","copy_ngrams","music_id","style_id"],
      "properties":{
        "structural":{"type":"string"},
        "visual_phash_first":{"type":"string"},
        "copy_ngrams":{"type":"array","items":{"type":"string"}},
        "music_id":{"type":"string"},
        "style_id":{"type":"string"},
        "shot_ids":{"type":"array","items":{"type":"string"}}
      }},

    "budget":{"type":"object",
      "required":["planned_cost_cents","llm_calls","gpu_seconds","render_count"],
      "properties":{
        "planned_cost_cents":{"type":"integer"},
        "llm_calls":{"type":"integer"},
        "gpu_seconds":{"type":"number"},
        "render_count":{"type":"integer"}}},

    "provenance":{"$ref":"common/provenance.json"}
  },

  "definitions": {
    "Clip": {
      "type":"object",
      "required":["clip_id","beat_role","source","src_in","src_out",
                  "tl_start","tl_end","transform","transition_in"],
      "properties":{
        "clip_id":{"type":"string"},
        "beat_role":{"$ref":"vocab/beat_role.json"},
        "source":{"type":"object","required":["kind","ref"],
          "properties":{
            "kind":{"enum":["SHOT","GRAPHIC","GENERATED","COLOR"]},
            "ref":{"type":"string"},                 // shot_id 或 graphic_id
            "content_hash":{"type":"string"}         // 关键：钉死媒体版本
          }},
        "src_in":{"type":"integer"},"src_out":{"type":"integer"},
        "tl_start":{"type":"integer"},"tl_end":{"type":"integer"},
        "speed":{"type":"number","default":1.0},
        "transform":{"type":"object",
          "required":["crop","scale","position"],
          "properties":{
            "crop":{"type":"object","properties":{
              "x":{"type":"integer"},"y":{"type":"integer"},
              "w":{"type":"integer"},"h":{"type":"integer"}}},
            "scale":{"type":"number"},
            "position":{"type":"object","properties":{
              "x":{"type":"integer"},"y":{"type":"integer"}}},
            "ken_burns":{"type":["object","null"],"properties":{
              "from":{"type":"array"},"to":{"type":"array"},
              "easing":{"enum":["LINEAR","EASE_IN_OUT"]}}}
          }},
        "color":{"type":"object","properties":{
          "lut_id":{"type":["string","null"]},
          "exposure":{"type":"number"},"saturation":{"type":"number"}}},
        "transition_in":{"type":"object","properties":{
          "kind":{"enum":["CUT","FADE","WHIP","ZOOM_PUNCH","WIPE"]},
          "duration_frames":{"type":"integer"}}},
        "audio_from_source":{"type":"boolean","default":false}
      }
    }
  }
}
```

**几个设计要点，每一条都是为了防止你日后返工：**

- **整数帧作为唯一时间单位**（`timebase.unit = frame`）。FFmpeg 与 Remotion 的浮点秒舍入行为不一致，这会导致字幕与画面错开 1–2 帧，肉眼可见。**这是最容易踩且最难查的坑。**
- **`source.content_hash`**：钉死媒体版本。素材重新转码后 hash 变了，旧 plan 应该显式失效而不是静默渲出不同结果。
- **`constraints_report`**：不只是日志，它是你调试"为什么今天这条这么烂"的唯一途径，也是给运营看的解释。
- **`diversity_signature` 与 IR 同层存储**：判重不需要加载完整 IR。
- **`overlays[].component` 是白名单**：Remotion 组件不能由 LLM 自由命名，必须是注册过的组件（见 §5.6 的组件注册表）。

### 2.2.4 ProductionOrder（制作令）

**这是你最该重视的 schema**，因为它是"创意 → 工业采购规格"的翻译器，也是你的第一护城河。

```jsonc
// schema/entities/production_order.schema.json (v1)
{
  "type":"object",
  "required":["order_id","tenant_id","kind","vendor_type","items",
              "acceptance_spec","deadline","budget_cents"],
  "properties":{
    "order_id":{"type":"string"},
    "kind":{"enum":["REAL_SHOOT","GEN_AVATAR","GEN_BROLL","UGC_RESHOOT","GRAPHIC"]},
    "vendor_type":{"enum":["PHOTOGRAPHER","GEN_VENDOR","MERCHANT_SELF","INTERNAL"]},

    "items":{"type":"array","items":{
      "type":"object",
      "required":["item_id","intent","spec","acceptance_assertions"],
      "properties":{
        "item_id":{"type":"string"},

        "intent":{                          // 为什么要这个镜头（给人看）
          "type":"object",
          "required":["serves_slots","pillar_id","narrative_note"],
          "properties":{
            "serves_slots":{"type":"array","items":{"type":"string"}},
            "pillar_id":{"type":"string"},
            "narrative_note":{"type":"string"}
          }},

        "spec":{                            // 拍什么（给机器校验）
          "type":"object",
          "required":["shot_type","camera_motion","scene","subject",
                      "duration_sec","framing","technical"],
          "properties":{
            "shot_type":{"$ref":"vocab/shot_type.json"},
            "camera_motion":{"type":"object"},
            "scene":{"$ref":"vocab/scene.json"},
            "subject":{"type":"array","items":{"$ref":"vocab/subject.json"}},
            "action":{"$ref":"vocab/action.json"},
            "duration_sec":{"type":"array","minItems":2,"maxItems":2},
            "framing":{
              "type":"object",
              "required":["subject_area_ratio","negative_space","headroom"],
              "properties":{
                "subject_area_ratio":{"type":"array"},   // [min,max]
                "negative_space":{"enum":["TOP","BOTTOM","LEFT","RIGHT","NONE"]},
                "headroom":{"enum":["TIGHT","NORMAL","LOOSE"]},
                "reference_image_url":{"type":"string"}  // 构图示意图（AI 生成）
              }},
            "coverage":{                     // 一个动作要几种覆盖，避免返工
              "type":"array",
              "items":{"enum":["MASTER","INSERT","REACTION","ALT_ANGLE"]}},
            "technical":{
              "type":"object",
              "properties":{
                "min_resolution":{"const":"1080x1920"},
                "min_fps":{"type":"integer","default":30},
                "handles_sec":{"type":"number","default":1.0},  // 首尾余量！
                "clean_in_out":{"type":"boolean","default":true},
                "no_talking_unless":{"type":"boolean","default":true},
                "lighting":{"enum":["NATURAL","SOFT_KEY","AMBIENT_ONLY"]},
                "audio_required":{"type":"boolean"}
              }}
          }},

        "acceptance_assertions":{             // ← 合同界面：机器可读验收条款
          "type":"array","minItems":3,
          "items":{"$ref":"qc_assertion.json"}
        },

        "hard_negatives":{"type":"array","items":{"type":"string"}}
      }}},

    "acceptance_spec":{
      "type":"object",
      "required":["auto_gate_level","rework_rounds_included","payment_terms"],
      "properties":{
        "auto_gate_level":{"enum":["L0","L0+L1","L0+L1+L2"]},
        "rework_rounds_included":{"type":"integer"},
        "payment_terms":{"type":"string"},
        "pass_threshold":{"type":"object","properties":{
          "blocker_count":{"const":0},
          "major_count_max":{"type":"integer"}}}
      }},
    "deadline":{"type":"string","format":"date-time"},
    "budget_cents":{"type":"integer"}
  }
}
```

`technical.handles_sec`（首尾余量）这一条看起来微小，但它是**外包实拍素材可用率从 40% 提到 85% 的单点最大杠杆**。摄影师默认会"卡着动作开始/结束"，导致没有转场余量，剪辑时全是硬切。

### 2.2.5 QCAssertion（断言 DSL）

```jsonc
// schema/entities/qc_assertion.schema.json (v1)
{
  "type":"object",
  "required":["assertion_id","level","severity","probe","expect","remedy"],
  "properties":{
    "assertion_id":{"type":"string"},          // e.g. "L1.SUBJECT_PRESENT.knife"
    "level":{"enum":["L0","L1","L2","L3"]},
    "severity":{"enum":["BLOCKER","MAJOR","MINOR"]},

    "probe":{                                   // 可插拔算子
      "type":"object",
      "required":["op","args"],
      "properties":{
        "op":{"enum":[
          // L0 确定性
          "ffprobe_field","blackdetect_ratio","freezedetect_ratio",
          "laplacian_var","optical_flow_magnitude","loudness_lufs",
          "true_peak_dbtp","silence_ratio","flicker_index","resolution","fps",
          // L1 一致性（开放词表检测 / VLM 判定题）
          "object_present","object_area_ratio","shot_type_match",
          "camera_motion_match","negative_space_at","subject_bbox_within_safe",
          "vlm_boolean",
          // L2 生成物缺陷
          "lipsync_lse_c","lipsync_lse_d","temporal_warp_error",
          "nr_video_quality","face_identity_sim",
          // L3 合规
          "banned_terms","required_disclaimer","third_party_logo",
          "third_party_face","aigc_metadata_present","aigc_overlay_present"
        ]},
        "args":{"type":"object"}
      }},

    "expect":{
      "type":"object",
      "required":["op","value"],
      "properties":{
        "op":{"enum":["gte","lte","eq","neq","between","is_true","is_false",
                      "contains_none","contains_all"]},
        "value":{}
      }},

    "remedy":{                                  // ← 返修指令模板，不是错误消息
      "type":"object",
      "required":["action","instruction_template"],
      "properties":{
        "action":{"$ref":"vocab/remedy_action.json"},
        "instruction_template":{"type":"string"},  // 支持 {{measured}} {{expected}}
        "auto_fixable":{"type":"boolean"},
        "auto_fix_op":{"type":["string","null"]}   // 如 "auto_crop_recenter"
      }},

    "sampling":{"type":"object","properties":{
      "frames":{"enum":["ALL","EVERY_N","KEYFRAMES","FIRST_LAST","N_UNIFORM"]},
      "n":{"type":"integer"}}},
    "applies_when":{"$ref":"shot_slot_query.json#/definitions/Predicate"}
  }
}
```

关键设计：`applies_when` 复用 Predicate AST —— 断言是**条件性**的（比如"口型同步"只对 `has_lipsync=true` 的生成片段适用）。这让断言库可以无脑全量执行，自己筛选适用范围。

### 2.2.6 事件（Feedback / 飞轮燃料）

```jsonc
// schema/entities/event.schema.json  —— 不可变事件日志
{
  "type":"object",
  "required":["event_id","tenant_id","ts","kind","payload"],
  "properties":{
    "event_id":{"type":"string"},
    "tenant_id":{"type":"string"},
    "ts":{"type":"string","format":"date-time"},
    "kind":{"enum":[
      "PLAN_DELIVERED","PLAN_VIEWED",
      "PLAN_PUBLISHED","PLAN_IGNORED","PLAN_REJECTED",
      "COPY_EDITED","CLIP_REMOVED","CLIP_REORDERED","MUSIC_CHANGED",
      "REGENERATE_REQUESTED","RESHOOT_SUBMITTED",
      "PLATFORM_METRICS_SYNCED",
      "QC_FAILED","ORDER_REWORKED"
    ]},
    "payload":{"type":"object"},
    "plan_id":{"type":["string","null"]},
    "actor":{"enum":["MERCHANT","OPERATOR","SYSTEM","VENDOR"]}
  }
}
```

`COPY_EDITED` 的 payload 必须是 **JSON Patch**（RFC 6902），不是"编辑后的全文"。原因：diff 才是信号，全文是噪声。

## 2.3 数据库 DDL（PostgreSQL 16+，含 pgvector）

```sql
-- ============ 词表（从 YAML 生成，不手写） ============
CREATE TYPE shot_type AS ENUM ('EXTREME_CLOSEUP','CLOSEUP','MEDIUM','WIDE','OTS','TABLETOP','POV','INSERT');
CREATE TYPE camera_motion_type AS ENUM ('STATIC','PUSH','PULL','PAN','TILT','TRACK','HANDHELD','ORBIT','WHIP');
CREATE TYPE beat_role AS ENUM ('HOOK','CONTEXT','PROOF','CONTRAST','OFFER','CTA','BUMPER');
CREATE TYPE source_kind AS ENUM ('REAL_SHOT','GEN_AVATAR','GEN_BROLL','STOCK','GRAPHIC');
CREATE TYPE shot_state AS ENUM ('UPLOADED','SEGMENTED','TAGGED','REJECTED','AVAILABLE','COOLING','EXPIRED','QUARANTINED');

-- ============ 租户与意图 ============
CREATE TABLE tenants (
  id uuid PRIMARY KEY,
  name text NOT NULL,
  category text NOT NULL,
  compliance_profile_id text NOT NULL,
  plan_tier text NOT NULL,
  subscription_start date NOT NULL,
  subscription_months int NOT NULL DEFAULT 3,
  monthly_budget_cents int NOT NULL,
  created_at timestamptz DEFAULT now()
);

CREATE TABLE brand_kernels (
  id uuid PRIMARY KEY,
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  version int NOT NULL,
  doc jsonb NOT NULL,                       -- 校验过的 BrandKernel
  completeness numeric NOT NULL,
  is_active boolean NOT NULL DEFAULT false,
  created_at timestamptz DEFAULT now(),
  UNIQUE (tenant_id, version)
);
CREATE UNIQUE INDEX ON brand_kernels (tenant_id) WHERE is_active;

-- ============ 物料层 ============
CREATE TABLE assets (
  id uuid PRIMARY KEY,
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  source_kind source_kind NOT NULL,
  content_hash text NOT NULL,               -- sha256，去重与版本钉死
  storage_uri text NOT NULL,
  order_item_id uuid,                       -- 溯源到制作令
  probe jsonb NOT NULL,                     -- ffprobe 原始
  uploaded_at timestamptz DEFAULT now(),
  UNIQUE (tenant_id, content_hash)
);

CREATE TABLE shots (
  id uuid PRIMARY KEY,
  tenant_id uuid NOT NULL,
  asset_id uuid NOT NULL REFERENCES assets(id),
  state shot_state NOT NULL DEFAULT 'SEGMENTED',

  in_frame int NOT NULL, out_frame int NOT NULL,
  fps int NOT NULL,
  duration_frames int GENERATED ALWAYS AS (out_frame - in_frame) STORED,

  -- semantic (受控)
  shot_type shot_type,
  shot_type_classes text[],                 -- 等价类，展开存储便于 && 查询
  camera_motion_type camera_motion_type,
  camera_motion_dir text,
  scene text,
  subjects text[],
  actions text[],
  mood text[],

  -- affordance
  is_loopable boolean, clean_in boolean, clean_out boolean,
  has_speech boolean, has_lipsync boolean,
  negative_space text[],                    -- ['TOP','LEFT']
  safe_crop_9x16_ok boolean,
  subject_bbox_track jsonb,
  motion_energy numeric,

  -- technical
  sharpness numeric, niqe numeric, shake_score numeric,
  flicker_score numeric, audio_lufs numeric, audio_snr numeric,
  quality_tier smallint,                    -- 1(best)..4

  -- compliance
  risk_flags text[] DEFAULT '{}',
  third_party_faces int DEFAULT 0,
  third_party_logos text[] DEFAULT '{}',
  ocr_text text,

  -- lifecycle
  shot_date date, seasons text[] DEFAULT '{ALL}',
  ttl_at date, linked_skus text[], linked_campaigns text[],
  use_count int DEFAULT 0, last_used_at date,

  -- search
  embedding vector(1024),
  tsv tsvector GENERATED ALWAYS AS (
    to_tsvector('simple', coalesce(scene,'') || ' ' ||
      coalesce(array_to_string(subjects,' '),'') || ' ' ||
      coalesce(array_to_string(actions,' '),'') || ' ' ||
      coalesce(ocr_text,''))) STORED,

  tag_provenance jsonb NOT NULL,
  created_at timestamptz DEFAULT now()
);

-- 索引：覆盖 90% 的结构化过滤路径
CREATE INDEX shots_pick_idx ON shots (tenant_id, state, shot_type, camera_motion_type)
  WHERE state IN ('AVAILABLE','COOLING');
CREATE INDEX shots_classes_gin ON shots USING GIN (shot_type_classes);
CREATE INDEX shots_subjects_gin ON shots USING GIN (subjects);
CREATE INDEX shots_tsv_gin ON shots USING GIN (tsv);
CREATE INDEX shots_emb_hnsw ON shots USING hnsw (embedding vector_cosine_ops);
CREATE INDEX shots_lru_idx ON shots (tenant_id, use_count ASC, last_used_at ASC NULLS FIRST);

-- ============ 范式层（全局共享，非租户级） ============
CREATE TABLE beat_schemas (
  id text PRIMARY KEY,                      -- 'bs.food.origin_story'
  version int NOT NULL,
  verticals text[] NOT NULL,
  pillar_affinity text[] NOT NULL,
  doc jsonb NOT NULL,
  structural_signature text NOT NULL,
  status text NOT NULL DEFAULT 'DRAFT',      -- DRAFT|ACTIVE|DEPRECATED
  perf_stats jsonb DEFAULT '{}',             -- 飞轮回填
  UNIQUE (id, version)
);

CREATE TABLE style_themes (
  id text PRIMARY KEY, version int NOT NULL,
  doc jsonb NOT NULL, status text DEFAULT 'DRAFT',
  UNIQUE (id, version)
);

CREATE TABLE audio_tracks (
  id text PRIMARY KEY,
  license_kind text NOT NULL,               -- PLATFORM_LIBRARY|COMMERCIAL|CC0
  license_proof_uri text,                   -- 必填非空的授权凭证
  platform text[],                           -- 哪些平台可用
  bpm numeric, beat_grid jsonb,
  mood text[], energy numeric,
  CONSTRAINT license_proof_required CHECK (license_kind='PLATFORM_LIBRARY' OR license_proof_uri IS NOT NULL)
);

-- ============ 产物层 ============
CREATE TABLE monthly_schedules (
  id uuid PRIMARY KEY, tenant_id uuid NOT NULL,
  month date NOT NULL, doc jsonb NOT NULL,
  solver_stats jsonb, is_active boolean DEFAULT true,
  UNIQUE (tenant_id, month)
);

CREATE TABLE video_plans (
  id uuid PRIMARY KEY,
  tenant_id uuid NOT NULL,
  scheduled_date date NOT NULL,
  state text NOT NULL,
  ir jsonb NOT NULL,
  ir_digest text NOT NULL,                  -- 内容寻址：渲染缓存 key
  diversity_signature jsonb NOT NULL,
  beat_schema_id text, beat_schema_version int,
  style_theme_id text,
  shot_ids uuid[] NOT NULL,
  planned_cost_cents int NOT NULL,
  created_at timestamptz DEFAULT now(),
  UNIQUE (tenant_id, ir_digest)
);
CREATE INDEX ON video_plans (tenant_id, scheduled_date);
CREATE INDEX plans_sig_idx ON video_plans USING GIN (diversity_signature);

CREATE TABLE render_artifacts (
  id uuid PRIMARY KEY,
  plan_id uuid NOT NULL REFERENCES video_plans(id),
  ir_digest text NOT NULL,
  renderer_version text NOT NULL,           -- ffmpeg+remotion+font 指纹
  storage_uri text NOT NULL,
  content_hash text NOT NULL,
  first_frame_phash bytea NOT NULL,
  audio_fingerprint bytea,
  duration_frames int, cost_cents int,
  created_at timestamptz DEFAULT now(),
  UNIQUE (ir_digest, renderer_version)      -- 确定性渲染的表达
);

CREATE TABLE qc_reports (
  id uuid PRIMARY KEY,
  subject_kind text NOT NULL,               -- SHOT|ARTIFACT|ORDER_ITEM
  subject_id uuid NOT NULL,
  assertion_set_version text NOT NULL,
  results jsonb NOT NULL,                   -- [{assertion_id,pass,measured,evidence_uri}]
  verdict text NOT NULL,                    -- PASS|REWORK|FAIL
  remedy_sheet jsonb,
  human_override jsonb,
  created_at timestamptz DEFAULT now()
);

CREATE TABLE events (
  id bigserial PRIMARY KEY,
  event_id uuid UNIQUE NOT NULL,
  tenant_id uuid NOT NULL,
  plan_id uuid,
  kind text NOT NULL,
  actor text NOT NULL,
  payload jsonb NOT NULL,
  ts timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON events (tenant_id, kind, ts DESC);

-- ============ 预算 ============
CREATE TABLE budget_ledger (
  id bigserial PRIMARY KEY,
  tenant_id uuid NOT NULL, month date NOT NULL,
  category text NOT NULL,                   -- LLM|VLM|TTS|GEN|RENDER|STORAGE
  cost_cents int NOT NULL,
  ref_kind text, ref_id text,
  ts timestamptz DEFAULT now()
);
CREATE MATERIALIZED VIEW budget_usage AS
  SELECT tenant_id, month, sum(cost_cents) AS used_cents
  FROM budget_ledger GROUP BY 1,2;
```

## 2.4 版本与命名规则（写进 CONTRIBUTING.md，CI 强制）

| 规则 | 内容 |
|---|---|
| Schema 版本 | `<entity>/<major>`（如 `video_plan/1`）。**major 只在破坏性变更时递增**；minor 变更靠"新增可选字段"完成。 |
| 破坏性定义 | 删字段 / 改类型 / 加 required / 收窄 enum / 改语义 |
| 词表演进 | **enum 只允许追加**。废弃用 `deprecated: true` + `replaced_by`，不删。DB enum 用 `ALTER TYPE ... ADD VALUE`。 |
| 引用 | 所有跨实体引用用 `VersionedRef {id, version}`，禁止裸 id 引用可变实体 |
| 时间 | 全系统整数帧 + `fps`。DB 存 `_frame` 后缀。禁止 `float seconds` 出现在任何 schema。 |
| ID | 实体用 UUIDv7（时间有序）；范式/词表用可读稳定 slug（`bs.food.origin_story`） |
| Digest | `ir_digest = sha256(JCS(ir 去除 provenance/budget))` —— **RFC 8785 JSON 规范化**，否则 key 顺序会导致缓存失效 |

> **`ir_digest` 用 JCS 规范化这条极易被忽略。** Go 的 `encoding/json` 和 TS 的 `JSON.stringify` 输出的 key 顺序不同，如果不规范化，Go 端和 TS 端算出的 digest 不一致，渲染缓存永久失效。

---

# §3 契约设计（Contract Design）

## 3.1 模块边界与契约清单

```
┌──────────────── Control Plane (Go) ────────────────────────┐
│ S1 Interview  S4 ParadigmLib  S5 Planner  S7 QCOrchestrator│
│ S3 AssetQuery S8 ComplianceGate S9 Delivery S10 CostGovernor│
└────┬──────────┬───────────┬───────────┬────────────────────┘
     │ C1       │ C2        │ C3        │ C4
     ▼          ▼           ▼           ▼
  BAML      Operator     Renderer    Vendor
  (LLM)     (Python)     (TS)        (外部)
```

**七个契约，逐一定义：**

| ID | 边界 | 形式 | 冻结优先级 |
|---|---|---|---|
| **C0** | 词表 + 实体 Schema | codegen（编译期） | **P0 最先冻** |
| **C1** | 控制面 ↔ LLM | BAML function 签名 | P0 |
| **C2** | 控制面 ↔ Python 算子 | CLI: JSON stdin/stdout + 文件路径 | P0 |
| **C3** | 控制面 ↔ 渲染器 | `RenderRequest`（含 VideoPlan IR）→ `RenderResult` | P0 |
| **C4** | 控制面 ↔ 外部供应商 | ProductionOrder + QCReport（人可读 + 机器可读） | P1 |
| **C5** | 控制面 ↔ 商家端 | 交付 API + 事件上报 | P1 |
| **C6** | 服务间内部 | gRPC / 单体内接口 | P2（可后置） |

## 3.2 各契约的具体形式

### C2：Python 算子契约（最关键的隔离设计）

**契约原则**：每个 Python 算子是一个**纯函数 CLI**。它不知道数据库、不知道租户、不知道业务。

```
# 契约：算子调用协议 v1
$ operator <op_name> --contract-version 1 < request.json > response.json

request.json:
{
  "contract_version": 1,
  "op": "shot_segment",
  "inputs": { "media_path": "/mnt/work/abc.mp4" },
  "params": { "threshold": 27.0, "min_scene_len_frames": 15 },
  "workdir": "/mnt/work/job-123",
  "determinism": { "seed": 42 }
}

response.json:
{
  "contract_version": 1,
  "op": "shot_segment",
  "status": "OK",                    // OK | INPUT_ERROR | RUNTIME_ERROR | TIMEOUT
  "outputs": { "shots": [ { "in_frame": 0, "out_frame": 120, "confidence": 0.9 } ] },
  "artifacts": [ { "role":"keyframes", "paths": ["/mnt/work/job-123/kf_0.jpg"] } ],
  "metrics": { "wall_ms": 3412, "gpu_seconds": 0.0 },
  "operator_version": "shot_segment@1.2.0",
  "model_versions": { "pyscenedetect": "0.6.4" }
}
```

必须实现的 12 个算子（每个都是独立 Docker image，独立版本）：

| 算子 | 职责 | 底层 |
|---|---|---|
| `probe` | 媒体元信息 | ffprobe |
| `shot_segment` | 镜头切分 + 关键帧 | PySceneDetect / TransNetV2 |
| `tech_metrics` | 清晰度/抖动/曝光/闪烁 | OpenCV + RAFT |
| `audio_metrics` | LUFS/峰值/静音/SNR/VAD | pyloudnorm + Silero VAD |
| `vlm_tag` | 语义打标（受控词表输出） | Qwen3-VL via BAML（**注意：这个走 C1 不走 C2**） |
| `detect_open_vocab` | 开放词表检测 | YOLO-World / GroundingDINO |
| `face_logo_ocr` | 人脸/logo/文字 | InsightFace + PaddleOCR |
| `crop_analyze` | 9:16 可裁性 + 主体轨迹 + 负空间 | YOLO + 轨迹平滑 |
| `align_words` | 词级强制对齐 | WhisperX / FunASR |
| `beat_track` | BPM + 节拍网格 | librosa / madmom |
| `nr_quality` | 无参考视频质量 | DOVER / FastVQA |
| `lipsync_score` | 口型同步 | SyncNet |

**为什么 `vlm_tag` 走 BAML 而不是 Python 算子**：因为它的输出必须严格符合受控词表 schema，BAML 的 SAP（schema-aligned parsing）+ enum 约束正是为此设计的。让 Python 去做 JSON 解析和重试是重复劳动。

**Go 侧调用封装**：

```go
// internal/operator/client.go
package operator

type Request struct {
    ContractVersion int               `json:"contract_version"`
    Op              string            `json:"op"`
    Inputs          map[string]any    `json:"inputs"`
    Params          map[string]any    `json:"params"`
    Workdir         string            `json:"workdir"`
    Determinism     Determinism       `json:"determinism"`
}

type Response struct {
    ContractVersion int                `json:"contract_version"`
    Op              string             `json:"op"`
    Status          Status             `json:"status"`
    Outputs         json.RawMessage    `json:"outputs"`
    Artifacts       []Artifact         `json:"artifacts"`
    Metrics         Metrics            `json:"metrics"`
    OperatorVersion string             `json:"operator_version"`
    ModelVersions   map[string]string  `json:"model_versions"`
    Error           *OpError           `json:"error,omitempty"`
}

type Runner interface {
    Run(ctx context.Context, req Request) (Response, error)
}

// 三个实现：
//  - LocalRunner   : exec.Command，开发用
//  - DockerRunner  : 每算子独立镜像，生产用
//  - FakeRunner    : 读 testdata/golden/<op>/<digest>.json，测试用 ← 关键
```

`FakeRunner` 是让你的 Go 业务逻辑能被完全单元测试的前提。**每个算子都必须提供一套 golden fixtures**，这是算子作者的交付义务，写进 C2 契约。

### C3：渲染契约

```jsonc
// RenderRequest v1
{
  "contract_version": 1,
  "plan": { /* VideoPlan IR，schema_version: video_plan/1 */ },
  "resolved_media": [                     // 控制面预解析，渲染器不查库
    { "ref":"shot:uuid", "local_path":"/mnt/media/x.mp4",
      "content_hash":"sha256:...", "fps":30 }
  ],
  "fonts": [ {"family":"XX", "path":"/fonts/xx.ttf", "hash":"sha256:..."} ],
  "output": { "path":"/out/plan-123.mp4", "codec":"h264",
              "crf":20, "preset":"medium", "audio_bitrate":"128k" },
  "modes": {
    "overlay_only": false,                 // true 时只渲 Remotion alpha 层
    "preview": false,                      // 低码率快出
    "deterministic": true                  // 禁用一切时间/随机依赖
  },
  "renderer_expect": {                     // 版本钉死，防漂移
    "ffmpeg":"7.1", "remotion":"4.0.x", "node":"22.x"
  }
}
```

```jsonc
// RenderResult v1
{
  "contract_version": 1,
  "status":"OK",
  "output": { "path":"...", "content_hash":"sha256:...",
              "duration_frames": 900, "size_bytes": 12345678 },
  "verification": {
    "first_frame_phash":"...",
    "audio_fingerprint":"...",
    "measured_lufs": -14.2,
    "frame_count_matches_ir": true,
    "safe_area_violations": []           // ← 渲染器自检，见下
  },
  "stages": [ {"name":"remotion_overlay","ms":8200,"renders":1},
              {"name":"ffmpeg_compose","ms":4100} ],
  "cost": { "render_count": 1, "cents": 1 },
  "renderer_version":"renderer@1.4.2+ffmpeg7.1+remotion4.0.230+fonts.abc123"
}
```

**关键契约条款（写进文档，测试验证）：**

- **R-1 确定性**：同一 `plan` + 同一 `renderer_version` + `deterministic:true` → 输出 `content_hash` 必须逐字节一致。
- **R-2 帧数守恒**：`output.duration_frames == max(clip.tl_end)`。
- **R-3 安全区自检**：渲染器必须自己检查所有 overlay 的 `layout_box` 与 caption 实际渲出的 bbox 是否越过 `canvas.safe_area`，越界则返回 `safe_area_violations` 并**拒绝**（不是警告）。
- **R-4 无隐式回退**：找不到字体/媒体 → 报错，禁止 fallback 到默认字体（默认字体会导致排版全乱且悄无声息）。
- **R-5 renderer_version 必须包含字体哈希**。字体升级会改变文字宽度 → 换行位置变 → 帧不一致。这一点非常隐蔽。

### C4：供应商契约（双形态）

同一个 `ProductionOrder` 必须能渲染成两种视图：

1. **机器可读 JSON**（走 API 的生成商）
2. **人类可执行 PDF/网页 shot list**（摄影师、商家自拍）—— 由 Remotion/HTML 模板从同一 JSON 生成，**含 AI 生成的构图示意图**

`QCReport` 同理：JSON + 人类可读返修单（带截帧证据、红框标注、具体动作）。

> **这个"同一真源双视图"是你外包协同效率的关键**。任何"人工写一份给摄影师，另写一份给系统"的做法都会漂移。

### C5：商家端契约（含发布链路）

```jsonc
// GET /api/v1/deliveries/today
{
  "date":"2026-06-01",
  "items":[{
    "plan_id":"...",
    "video_url":"...",              // 带签名，短期有效
    "cover_url":"...",
    "post_text":"...",
    "hashtags":["..."],
    "aigc_disclosure_required": true,
    "publish_hints": {
      "recommended_window":["18:00","20:00"],
      "platform_schemes": {           // 唤起而非代发
        "douyin": {"scheme":"snssdk1128://...", "h5_share_url":"..."},
        "kuaishou": {"scheme":"..."}
      }
    },
    "why_this": {                     // 给商家看的解释，提升信任与续订
      "pillar":"食材溯源",
      "hook_strategy":"反常识提问",
      "one_line":"今天用清晨进货镜头讲新鲜度"
    }
  }]
}

// POST /api/v1/deliveries/{plan_id}/feedback   ← 飞轮入口
{
  "action":"PUBLISHED|IGNORED|REJECTED|EDITED_PUBLISHED",
  "reason_code":"TOO_SALESY|WRONG_FACT|UGLY|NOT_MY_STYLE|SEASON_MISMATCH|OTHER",
  "reason_text":"...",
  "edits":[ {"op":"replace","path":"/copy/caption_blocks/0/text","value":"..."} ]
}
```

`reason_code` **必须是受控枚举**。自由文本反馈的信噪比太低，无法驱动飞轮。

## 3.3 兼容性规则

```
生产者（Producer）规则：
  P1  只增不减：新增字段必须 optional 且有默认语义
  P2  不改语义：字段含义变化 = major bump
  P3  广播 major bump 需要 2 周并行期

消费者（Consumer）规则：
  C1  未知字段必须忽略（禁止 strict parse 于入口层）
  C2  未知 enum 值 → 走 fallback 分支 + 告警，禁止 panic
  C3  必须校验 schema_version 前缀，major 不匹配则拒绝处理
```

`C2` 特别重要：词表在追加，旧版渲染器会遇到新的 `shot_type`。它必须降级为等价类处理而不是崩掉。

## 3.4 BAML 契约（C1）

BAML 支持原生 Go client（`baml init --client-type go`）以及 TypeScript 原生，这正好覆盖你的栈。契约设计要点：

**契约条款 B-1：BAML function 的返回类型必须引用 codegen 出的 enum**，不允许 `string`。

```baml
// baml_src/vocab.baml —— 由 codegen 从 YAML 生成，禁止手改
enum ShotType {
  EXTREME_CLOSEUP  @description("主体占画面>70%，质感/细节")
  CLOSEUP
  MEDIUM
  WIDE
  OTS
  TABLETOP
  POV
  INSERT
}
enum CameraMotionType { STATIC PUSH PULL PAN TILT TRACK HANDHELD ORBIT WHIP }
enum NegativeSpace { TOP BOTTOM LEFT RIGHT NONE }
// ... 14 张词表全部生成
```

```baml
// baml_src/tagging.baml
class ShotTags {
  shot_type ShotType
  shot_type_confidence float
  camera_motion CameraMotionType
  camera_motion_dir string?          @description("LEFT/RIGHT/IN/OUT/UP/DOWN，STATIC 时为 null")
  scene Scene
  subjects Subject[]                 @description("最多 5 个，按显著性降序")
  actions Action[]
  mood Mood[]

  clean_in bool                      @description("首帧是否处于稳定构图，无入场残影/手抖")
  clean_out bool
  is_loopable bool
  negative_space NegativeSpace[]     @description("可安放字幕的空白区域")

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
```

**契约条款 B-2：确定性字段禁止让 LLM 判断。** 上面 prompt 里明确把 tech_metrics 作为"可信输入"给它，并禁止它重新判断。这是控制 LLM 幻觉的最有效手段：**把可测量的东西测出来，只让 LLM 做不可测量的语义判断。**

**契约条款 B-3：`uncertain_fields` 是强制字段。** 它是你的主动学习入口 —— 只抽检 LLM 自己说不确定的样本，标注成本降一个数量级。

```baml
// baml_src/interview.baml  —— 苏格拉底问答
class InterviewTurn {
  next_question string?             @description("null 表示信息已充分")
  question_targets string[]         @description("这个问题要填的槽位路径，如 identity.differentiators[0].proof_types")
  why_asking string                 @description("给商家看的一句话理由，建立信任")
  quick_options string[]            @description("2-5 个可点选项，降低回答成本")
  extracted patch_ops              @description("从上一轮回答中提取的 BrandKernel 更新")
  completeness_estimate float
  stop_reason string?               @description("SUFFICIENT | DIMINISHING_RETURNS | MAX_TURNS")
}

class patch_ops {
  ops PatchOp[]
}
class PatchOp {
  op "replace" | "add"
  path string
  value string
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
```

**注意"苏格拉底式问答"的工程化关键**：不是让 LLM 自由聊，而是 **slot_coverage 驱动 + 硬上限 + 递减收益停止**。而且必须给 `quick_options` —— 小商家不会打字。

**契约条款 B-4：每个 BAML function 必须有 ≥5 个 test block，含 2 个对抗样本。** 见 §4.5。

---

# §4 冻结前的契约测试

这是你问得最好的一个问题。我给一个七层测试策略 + 明确的 Freeze Gate 清单。

## 4.1 测试金字塔

```
L7  影子交付      真实客户，人工并行审核，不外发 —— 冻结后仍持续
L6  端到端场景    30 天全流程模拟（种子租户）
L5  确定性/复现    同输入同输出，跨机器跨时间
L4  变形测试      Metamorphic：扰动输入，验证不变式
L3  属性测试      Property-based：随机生成输入，验证不变式
L2  契约测试      Provider/Consumer 双向 + golden fixtures
L1  Schema 测试   合法/非法样本 + 跨语言一致性
```

## 4.2 L1 — Schema 测试

对每个 schema 准备三类样本，放在 `schema/testdata/<entity>/`：

```
valid/          ≥5 个，覆盖 minimal / typical / maximal / 边界
invalid/        ≥15 个，每个对应一条 required/enum/range/不变式违反
                文件名即断言：missing_pillars.json, enum_bad_shot_type.json,
                             invariant_bs2_duration_unsat.json
evolution/      上一 major 版本的真实样本，验证向后兼容
```

**跨语言一致性测试**（这是最容易漏掉、后果最严重的一项）：

```go
// schema/crosslang_test.go
func TestCrossLanguageValidationParity(t *testing.T) {
    for _, f := range allTestdataFiles() {
        goResult := govalidator.Validate(f)
        tsResult := runNodeValidator(f)     // 调 TS 侧 zod
        bamlResult := runBamlParse(f)       // 若该实体是 BAML 输出类型
        require.Equal(t, goResult.Valid, tsResult.Valid, f)
        require.Equal(t, goResult.Valid, bamlResult.Valid, f)
        // 错误路径也要一致
        require.ElementsMatch(t, goResult.ErrorPaths(), tsResult.ErrorPaths(), f)
    }
}
```

**JCS digest 一致性测试**（对应 §2.4 的坑）：

```go
func TestIRDigestCrossLanguage(t *testing.T) {
    for _, plan := range validPlans() {
        goDigest := ir.Digest(plan)
        tsDigest := runNodeDigest(plan)
        require.Equal(t, goDigest, tsDigest)
        // 且对 key 顺序不敏感
        shuffled := shuffleJSONKeys(plan)
        require.Equal(t, goDigest, ir.Digest(shuffled))
    }
}
```

## 4.3 L2 — 契约测试

### C2 算子契约：双向

**Provider 侧（Python 算子作者的义务）**：

```
operators/shot_segment/
├── contract/
│   ├── request.schema.json        ← 从公共 schema 引用
│   ├── response.schema.json
│   └── invariants.md
├── testdata/golden/
│   ├── case_01_static_indoor/{request.json, media.mp4(小), response.json}
│   ├── case_02_handheld_pan/...
│   ├── case_03_single_long_take/...
│   ├── case_04_corrupt_media/     ← 必须返回 INPUT_ERROR 而非崩溃
│   └── case_05_zero_length/
└── test_contract.py               ← CI 强制：跑 golden，比对 response
```

**Consumer 侧（Go 业务逻辑）**：用 `FakeRunner` 读同一批 golden。

```go
func TestIngestPipeline_UsesGoldenOperators(t *testing.T) {
    runner := operator.NewFakeRunner("../../operators/*/testdata/golden")
    svc := ingest.New(runner, memStore())
    shots, err := svc.Process(ctx, "case_02_handheld_pan")
    require.NoError(t, err)
    require.Len(t, shots, 4)
    require.Equal(t, "HANDHELD", shots[0].CameraMotionType)
}
```

**关键规则：Consumer 只能使用 Provider golden 中出现过的字段。** CI 检查：扫描 Go 代码对 `Outputs` 的字段访问，与 golden 的字段集做交集校验。这就是 Consumer-Driven Contract 的轻量实现（不需要 Pact broker）。

### C3 渲染契约：帧级快照

```
renderer/contract/
├── cases/
│   ├── 01_minimal_single_clip/plan.json
│   ├── 02_five_beats_full/plan.json
│   ├── 03_all_overlay_components/plan.json    ← 覆盖每个注册组件
│   ├── 04_safe_area_violation/plan.json       ← 期望 REJECT
│   ├── 05_missing_font/plan.json              ← 期望 ERROR，不 fallback
│   ├── 06_cjk_long_caption_wrap/plan.json     ← 中文换行
│   ├── 07_speed_ramp_and_transition/plan.json
│   └── 08_beat_synced_cuts/plan.json
└── golden/
    └── 01_minimal_single_clip/
        ├── result.json                        (去掉 timing 字段)
        ├── frames/{0,1,29,30,449,899}.png     ← 采样帧
        └── audio.wav.fingerprint
```

```go
func TestRenderContract(t *testing.T) {
    for _, c := range cases() {
        res := renderer.Render(c.Request)
        // 1) 结果结构
        assertJSONMatch(t, c.Golden.Result, redact(res))
        // 2) 采样帧：SSIM > 0.995 且 pHash 汉明距离 == 0
        for _, fi := range c.Golden.FrameIndices {
            got := extractFrame(res.Output.Path, fi)
            require.Greater(t, ssim(got, c.Golden.Frame(fi)), 0.995)
            require.Equal(t, 0, phashDistance(got, c.Golden.Frame(fi)))
        }
        // 3) 音频指纹
        require.Equal(t, c.Golden.AudioFP, fingerprint(res.Output.Path))
        // 4) 帧数守恒
        require.Equal(t, maxTLEnd(c.Request.Plan), res.Output.DurationFrames)
    }
}
```

> **为什么用采样帧而非全片哈希**：GPU/CPU 编码器版本差异会导致字节不同但视觉相同。采样帧 + SSIM 是可移植的。全片字节哈希只在**完全相同的容器镜像**内才成立 —— 所以再加一条 CI-only 的严格测试：在固定 image 内跑，要求字节一致。

### C1 BAML 契约

BAML 的 test block 直接写在 `.baml` 里，在 VSCode 与 CLI 都能跑：

```baml
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
}

// 对抗样本：模糊 + 抖动，期望它老实承认不可用
test tag_shot_adversarial_blurry {
  functions [TagShot]
  args { /* ... */ tech_metrics_json #"{"sharpness":45,"shake_score":0.41}"# }
  @@assert(admits_unusable, {{ this.usable_reason != null }})
  @@assert(flags_uncertainty, {{ this.uncertain_fields|length > 0 }})
}
```

**BAML 契约测试的三条纪律：**

1. **只断言结构与不变式，绝不断言具体文本。** 断言"`shot_type` 属于 DETAIL 等价类"，不是断言"`shot_type == CLOSEUP`"。
2. **必须有对抗样本**（模糊、空画面、多主体、误导性 OCR 文字、跨类目素材）。
3. **跑多模型**：同一 test 在 `QwenVLLocal` / `Sonnet` / `GPT` 三个 client 下都要过。这保证你的 prompt 不是过拟合到某个模型，也让你日后能自由换模型降本。

### 补充：Golden Set 的构建（无需人工标注）

冷启动 golden set 用**共识法**：

```
1. 取 300 个真实素材片段
2. 用 3 个不同 VLM（Qwen3-VL / InternVL / 云端 API）各打一次标
3. 三者一致的字段 → 直接进 golden（约 60–70%）
4. 三者不一致的字段 → 进人工队列（只需标 30–40%，且是最有信息量的部分）
5. 每个 golden 样本记录 agreement 分数，作为该字段的可信度基线
```

这个"分歧驱动标注"能把你的标注量压到 1/3，且标的都是难例。**后续飞轮阶段，`uncertain_fields` + 商家 `CLIP_REMOVED` 事件自动补充 golden set。**

## 4.4 L3 — 属性测试（Property-based）

约束求解器和谓词编译器是最该用属性测试的地方。Go 用 `pgregory.net/rapid`。

```go
func TestPredicateCompiler_Properties(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        pred := genPredicate(rt)            // 随机生成合法 Predicate AST
        shots := genShots(rt, 200)          // 随机生成 Shot 集合

        // 性质1：SQL 编译结果 == 内存中直接 eval
        sqlRes := queryDB(t, compile(pred), shots)
        memRes := evalInMemory(pred, shots)
        require.ElementsMatch(rt, ids(sqlRes), ids(memRes))

        // 性质2：must 越多，结果集单调不增
        extra := genPredicate(rt)
        andRes := queryDB(t, compile(and(pred, extra)), shots)
        require.Subset(rt, ids(sqlRes), ids(andRes))

        // 性质3：编译结果永远参数化，不含字面量拼接
        require.NotContains(rt, compile(pred).SQL, "'")
    })
}
```

```go
func TestPlanner_Invariants(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        pool := genShotPool(rt, rapid.IntRange(20, 400).Draw(rt, "n"))
        schemas := genBeatSchemas(rt, 30)
        days := 30

        sched, err := planner.SolveMonth(pool, schemas, cfg)

        // P1: 永不无解（fallback_chain 末端保证）
        require.NoError(rt, err)
        require.Len(rt, sched.Days, days)

        // P2: 冷却约束
        for _, v := range violations(sched, CooldownRule) { rt.Fatalf("cooldown: %v", v) }

        // P3: 无连续 3 天同构
        for i := 2; i < days; i++ {
            sigs := []string{sched.Days[i-2].Sig, sched.Days[i-1].Sig, sched.Days[i].Sig}
            require.False(rt, allEqual(sigs))
        }

        // P4: 素材消耗均衡 —— 前 10 天不得消耗 >45% 的 tier-1 素材
        require.LessOrEqual(rt, tier1Consumed(sched, 0, 10), 0.45*tier1Total(pool))

        // P5: 预算不超
        require.LessOrEqual(rt, sched.TotalCostCents, cfg.MonthlyBudgetCents)

        // P6: 素材池极小时优雅降级而非崩溃
        if len(pool) < 30 { require.Greater(rt, sched.FallbackUsageRate, 0.0) }
    })
}
```

**P4 和 P6 是你最容易翻车的两条**，属性测试能在开发期就抓到。

## 4.5 L4 — 变形测试（Metamorphic）

对 LLM 环节和 Planner，你没有"正确答案"，但有**不变关系**：

| 变形 | 期望不变式 |
|---|---|
| Shot 关键帧做轻微色彩抖动/JPEG 重压 | `shot_type` / `camera_motion` 不变 |
| Shot 水平镜像 | `camera_motion_dir` 应翻转（LEFT↔RIGHT），`shot_type` 不变 |
| Shot 时间反放 | `camera_motion` PUSH↔PULL 翻转 |
| BrandKernel 中 pillar 顺序打乱 | 生成的 30 天排期的 pillar 配比分布不变（KS 检验） |
| 素材池增加 10 个无关素材 | 已有 plan 的 hard 约束满足性不变 |
| 同一 plan 用 seed=1 vs seed=2 | 结构合法性不变，但 `diversity_signature` 不同 |
| BeatSchema 库从 30 减到 15 | 多样性指标下降但不违反硬约束（优雅降级） |

```go
func TestMetamorphic_MirrorFlipsDirection(t *testing.T) {
    for _, s := range shotFixturesWithMotion() {
        a := bamlTagShot(s.Keyframes, s.Metrics)
        b := bamlTagShot(mirror(s.Keyframes), s.Metrics)
        require.Equal(t, a.ShotType, b.ShotType)
        require.Equal(t, flipDir(a.CameraMotionDir), b.CameraMotionDir)
    }
}
```

## 4.6 L5 — 确定性与复现测试

```go
func TestDeterminism_SamePlanSameBytes(t *testing.T) {
    plan := loadPlan("02_five_beats_full")
    h1 := render(plan).ContentHash
    h2 := render(plan).ContentHash              // 立刻重跑
    time.Sleep(2 * time.Second)                  // 排除时间依赖
    h3 := render(plan).ContentHash
    require.Equal(t, h1, h2); require.Equal(t, h1, h3)
}

func TestDeterminism_PlannerIsPureFunction(t *testing.T) {
    // 把所有 LLM/算子输出预先固定为 artifact，Planner 应完全确定
    fx := loadFrozenArtifacts("tenant_seed_01")
    p1 := planner.PlanDay(fx, date, seed(7))
    p2 := planner.PlanDay(fx, date, seed(7))
    require.Equal(t, ir.Digest(p1), ir.Digest(p2))
}
```

**同时必须有一个"反确定性"测试**，防止过度确定化导致每天一样：

```go
func TestNonDegenerate_ThirtyDaysAreDiverse(t *testing.T) {
    sched := planner.SolveMonth(seedPool(), allSchemas(), cfg)
    sigs := structuralSigs(sched)
    require.GreaterOrEqual(t, len(unique(sigs)), 18)       // 30 天 ≥18 种结构
    require.LessOrEqual(t, maxNgramOverlap(sched), 0.15)
    require.GreaterOrEqual(t, minPHashDistance(sched, 14), 8)
    require.GreaterOrEqual(t, len(unique(musicIDs(sched))), 10)
}
```

**这个测试就是你"平台判重风险"的自动化防线**，也是我认为整套测试里商业价值最高的一个。

## 4.7 L6 — 端到端场景测试

建立 3 个**种子租户**（seed tenant）作为永久回归资产：

| 种子租户 | 特征 | 测什么 |
|---|---|---|
| `seed_food_rich` | 餐饮，素材充足（300 shots），无合规风险 | Happy path，多样性上限 |
| `seed_beauty_poor` | 美容，素材贫瘠（45 shots），有肖像与功效宣称风险 | Fallback 链、合规门、降级质量 |
| `seed_pet_messy` | 宠物店，素材脏（含第三方人脸、竞品 logo、过季） | 隔离、TTL、补拍触发 |

E2E 测试跑完整 30 天，断言：

```go
func TestE2E_ThirtyDayDelivery(t *testing.T) {
    for _, tenant := range seedTenants() {
        run := e2e.Run(tenant, days(30))
        require.Equal(t, 30, run.DeliveredCount)          // 每天都有
        require.Zero(t, run.BlockerQCFailures)
        require.Zero(t, run.ComplianceViolations)
        require.LessOrEqual(t, run.TotalCostCents, tenant.MonthlyBudgetCents)
        require.GreaterOrEqual(t, run.DiversityScore, 0.7)
        require.LessOrEqual(t, run.HumanInterventionRate, 0.15)  // 关键 UE 指标
        require.Zero(t, run.PlansUsingRevokedVoice)
        require.Zero(t, run.PlansUsingExpiredShots)
    }
}
```

`HumanInterventionRate` 是你的**单位经济学健康度指标**。如果它 >30%，$1000/月的定价撑不住。让它成为一个测试断言，而不是月底才发现的问题。

## 4.8 Freeze Gate 检查表

**冻结不是"觉得差不多了"，是一个有明确准入条件的评审。** 每个契约冻结需过以下 12 项：

```
[ ] G1  Schema 有 ≥5 valid + ≥15 invalid 样本，全部按预期通过/失败
[ ] G2  跨语言（Go/TS/BAML）校验结果一致性测试通过
[ ] G3  JCS digest 跨语言一致，且对 key 顺序不敏感
[ ] G4  所有跨字段不变式（IV-*）有对应实现 + 对应 invalid 样本
[ ] G5  向后兼容测试：上一 major 的样本仍可被消费（或明确声明 breaking + 迁移脚本）
[ ] G6  Provider golden fixtures ≥5 组，含 ≥2 组错误/边界路径
[ ] G7  Consumer 只访问 golden 中出现过的字段（CI 静态检查）
[ ] G8  未知字段忽略 + 未知 enum 降级，两条各有测试
[ ] G9  属性测试跑 ≥1000 次无反例
[ ] G10 至少 3 条变形不变式有测试
[ ] G11 确定性测试通过 + 非退化多样性测试通过
[ ] G12 在 3 个种子租户上跑通 30 天 E2E
```

## 4.9 冻结顺序（这个顺序很重要）

```
第 1 波（Week 1–3）  ★ 必须最先，一切依赖它
  ├── 受控词表 v1（14 张）
  ├── Shot / Asset schema
  ├── C2 算子调用协议
  └── ShotSlotQuery 谓词 AST

第 2 波（Week 4–6）
  ├── BrandKernel schema
  ├── VideoPlan IR v1        ← 最重要的单个决策
  ├── QCAssertion DSL
  └── C3 渲染契约

第 3 波（Week 7–9）
  ├── BeatSchema / StyleTheme
  ├── ProductionOrder
  └── C1 BAML function 签名

第 4 波（Week 10–12）
  ├── Event schema
  ├── C5 商家端 API
  └── ComplianceProfile
```

**为什么词表和 IR 最先冻**：它们的下游依赖最多，改动成本随时间指数上升。BeatSchema 反而可以晚冻，因为它是"内容资产"，会一直演进 —— 只要它的**容器 schema** 稳定。

**为什么 Event schema 可以最后**：它是 append-only 的，加字段成本最低。但**必须在第一个真实客户之前冻结**，因为丢失的事件无法追溯。

---

# §5 各服务开发 Spec

## S1 InterviewService（Onboarding）

**技术栈**：Go + BAML（Sonnet 级模型）+ TS 前端（微信小程序 / H5）

**输入**：`tenant_id`, `category`, 商家逐轮回答
**输出**：`BrandKernel`（v1, active）+ `AssetDemandPlan`

**核心算法**：槽位覆盖驱动的有限轮次访谈

```go
type SlotSpec struct {
    Path        string   // "identity.differentiators[*].proof_types"
    Weight      float64  // 对 completeness 的贡献
    MinFill     int
    Prerequisite []string // 依赖其他槽位先填
}

func (s *InterviewService) Next(ctx context.Context, st *State) (*Turn, error) {
    cov := s.coverage.Compute(st.Kernel)          // 确定性计算，不用 LLM
    if cov.Score >= 0.75 || st.Turn >= 18 ||
       cov.DeltaStagnant(3, 0.02) {
        return s.finalize(ctx, st)
    }
    // LLM 只负责"怎么问"，不负责"问不问、问哪个槽"
    target := cov.HighestValueMissingSlot()
    return baml.NextInterviewQuestion(ctx, st.Kernel, cov.JSON(), st.History,
                                      st.Vertical, st.Turn, target)
}
```

> **设计要点**：停止条件与槽位选择是**确定性代码**，只有"如何措辞提问"交给 LLM。这让访谈过程可测试、可预算、可复现。这是 §0 A3（重推理前置）与 A2（非确定性显式化）的具体落地。

**验收标准**：
- 18 轮内 `completeness ≥ 0.75` 的比例 ≥85%（在 20 个模拟商家 persona 上测）
- 平均轮次 ≤12
- 生成的 `differentiators` 中 `visual_provable=true` 的比例 ≥80%
- 单次 onboarding LLM 成本 ≤ ¥8

**开源/资源**：BAML；前端用小程序原生或 Taro（TS）。模拟商家 persona 用 LLM 生成 20 个 + 让它扮演回答，作为回归测试集。

## S2 IngestService（素材入库与打标）

**技术栈**：Go 编排 + Python 算子（Docker）+ BAML(VLM)

**流水线（DAG，每步幂等，中间产物内容寻址）**：

```
upload → probe → dedup(content_hash) 
       → shot_segment → [每 shot 并行]
            ├── extract_keyframes (3–5 帧，含首末帧)
            ├── tech_metrics
            ├── audio_metrics
            ├── crop_analyze        (9:16 可裁性 + 主体轨迹 + 负空间)
            ├── face_logo_ocr       (合规扫描)
            └── vlm_tag (BAML)      ← 唯一 LLM 步骤
       → merge → invariant_check → QC_L0 → state transition
```

**关键实现细节**：

1. **关键帧选取必须包含首末帧**，因为 `clean_in/clean_out` 判断依赖它们。中间帧用运动峰值采样，不要均匀采样（均匀采样会漏掉动作要点）。

2. **`negative_space` 计算用确定性方法而非 VLM**：
```python
# operators/crop_analyze
# 1) 对每个采样帧算局部方差图（低方差 = 平坦）
# 2) 排除主体 bbox
# 3) 在 9 宫格上聚合，返回连续可用区域（面积 >15% 画面）
# 4) 跨帧取交集 —— 只有全程都空的区域才算可用
```
   跨帧取交集这一点很关键，否则字幕会在某些帧压到主体上。

3. **`safe_crop_9x16` 判定**：主体 bbox 轨迹在裁切窗口内的 IoU 全程 >0.9 且无断头（bbox 顶部到裁切上边界 ≥ 头部高度 ×0.15）。

4. **成本控制**：`vlm_tag` 是主要成本。策略：
   - 先跑 `tech_metrics`，`sharpness < 阈值` 或 `shake > 阈值` 的 shot 直接 REJECTED，不调 VLM（能省 20–30%）
   - 关键帧压到 768px 长边
   - batch 调用（一次 8 个 shot）
   - `uncertain_fields` 非空的走一次 VLM 复核（第二个模型），其余不复核

**验收标准**：
- 每分钟素材端到端处理成本 ≤ ¥0.35
- 在 golden set 上：`shot_type` 准确率 ≥85%，`camera_motion` ≥90%，`clean_in/out` ≥80%
- `safe_crop_9x16` 假阳性率 ≤5%（假阳性会直接产出断头视频，比假阴性严重得多，阈值要偏保守）
- 合规扫描召回率 ≥95%（`third_party_faces`）

**开源清单**：
| 用途 | 项目 |
|---|---|
| 镜头切分 | PySceneDetect（content detector）；边界更准用 TransNetV2 |
| VLM | Qwen3-VL（本地 vLLM）；冷启动直接用云 API 更省事 |
| 开放词表检测 | YOLO-World、GroundingDINO |
| 人脸 | InsightFace |
| OCR | PaddleOCR（中文最稳） |
| 光流 | RAFT（精度）/ Farnebäck（速度） |
| 音频 | pyloudnorm、Silero VAD、librosa |
| 分割（打码用） | SAM2（Segment Anything 2，视频掩码） |

## S3 AssetQueryEngine

**技术栈**：纯 Go + PostgreSQL/pgvector

**职责**：`ShotSlotQuery` → 候选 shot 列表（带分数与解释）

**实现**：谓词 AST → 参数化 SQL 编译器 + 混合排序

```go
type CompiledQuery struct {
    SQL     string
    Args    []any
    NeedsVector bool
    Explain []string          // 每个谓词对应的人可读说明
}

func Compile(q *ShotSlotQuery, tenantID uuid.UUID, ctx PlanContext) (*CompiledQuery, error)

// 排序分数（确定性，可解释）
// score = w1*softMatch + w2*qualityTier + w3*freshness
//       - w4*useCount   - w5*recencyPenalty  + w6*semanticSim
```

**必须实现的三件事**：

1. **降级链执行器**：`must` 无结果 → 逐级 fallback，记录 `degrade_note` 到 `constraints_report`。
2. **`explain()` 接口**：任何空结果必须能回答"是哪个谓词把候选集清空了"（逐谓词递增执行，找出归零点）。这是运营排障的唯一手段。
3. **混合检索**：结构化 WHERE 先过滤 → 若候选 >K 则用 `should` 打分 → 若候选 <M 则加语义召回补充。**永远不要让语义检索决定 must**。

**验收标准**：
- P99 延迟 <80ms（10 万 shot 规模）
- `explain()` 对 100% 空结果给出归因谓词
- 属性测试：SQL 结果 == 内存 eval（§4.4 P1）

## S4 ParadigmLibrary（范式库）

**技术栈**：Go（服务）+ Python（离线蒸馏流水线）+ BAML

**离线蒸馏流水线**（这是冷启动核心，无人工标注）：

```
1. 采集：目标垂类头部账号公开视频（仅内部分析，不复用素材）
   —— 明确写进合规文档：分析用途、不存储原片超过 N 天、不再分发
2. shot_segment → 每 shot VLM 结构化描述
3. BAML: InferBeatSequence(shots) -> BeatRole[]     ← 推断叙事结构
4. 生成 structural_signature
5. 聚类（signature 的编辑距离 + beat 时长分布）→ 范式家族
6. BAML: SynthesizeBeatSchema(cluster) -> BeatSchema  ← 归纳成可复用骨架
7. 静态校验：IV-BS-1/2/3 + slot_query 可满足性（在种子素材池上试解）
8. 人工只做 accept/reject（每个 schema 30 秒），不做编写
```

```baml
class InferredBeat {
  role BeatRole
  start_sec float
  end_sec float
  shot_type ShotType
  copy_function CopyFunction
  rationale string
}
function InferBeatSequence(shot_descriptions: string, transcript: string) -> InferredBeat[] {
  client QwenVLLocal
  prompt #"
    下面是一条短视频的逐镜头描述与字幕转写。判断它的叙事结构：
    每个镜头承担什么叙事功能（HOOK/CONTEXT/PROOF/CONTRAST/OFFER/CTA/BUMPER）。
    规则：HOOK 只能出现在开头 3 秒内；CTA 通常在末尾；
    PROOF 必须有可见的证据画面（演示/对比/证言/资质）。
    ...
  "#
}
```

**关键：`slot_query` 的可满足性预检**。生成的 BeatSchema 必须在"典型素材池"（40 个 shot 的最小假想池）上能解出方案，否则拒收。这防止范式库里塞满"理论上很美但没素材能满足"的骨架。

**验收标准**：
- 每垂类产出 ≥25 个 ACTIVE BeatSchema
- `structural_signature` 去重后 ≥20 种
- 每个 schema 在最小素材池（40 shots）上可解率 100%
- 蒸馏 100 条参考视频的成本 ≤ ¥150

## S5 PlannerService ★ 核心

**技术栈**：纯 Go（这是你最该自己写的部分）

分两个阶段，这个拆分很重要：

### 阶段 A：月度全局排期（每月一次，可以慢）

**问题形式化**：

```
决策变量：x[d][s] ∈ {0,1}   第 d 天是否使用 BeatSchema s
         y[d][k][i] ∈ {0,1} 第 d 天第 k 个 slot 是否用 shot i

目标：max  Σ pillar_balance_score
         + Σ diversity_score
         + Σ quality_score
         - Σ λ·fallback_penalty
         - Σ μ·scarcity_cost      ← 稀缺素材的影子价格

硬约束：
  H1  Σ_s x[d][s] = 1                        每天恰好一个 schema
  H2  Σ_i y[d][k][i] = 1                     每 slot 恰好一个 shot（或 graphic 兜底）
  H3  y[d][k][i] ≤ eligible[d][k][i]         谓词可行性（预计算）
  H4  Σ_{d'∈[d-cd, d]} used(i,d') ≤ 1        冷却窗口
  H5  Σ_{d∈30d} used(i,d) ≤ max_uses_30d     复用上限
  H6  sig(d) ≠ sig(d-1) ∨ sig(d-1) ≠ sig(d-2) 不连续 3 天同构
  H7  Σ cost(d) ≤ monthly_budget
  H8  |pillar_count(p) - target_ratio(p)·30| ≤ 2   支柱配比
  H9  music(d) ∉ music(d-9..d-1)             音轨 10 日内不重
  H10 tier1_consumed(first_10d) ≤ 0.45·tier1_total  ← 素材消耗均衡
```

**求解策略（务实版，避免过早引入 CP-SAT）**：

问题规模：30 天 × 5–7 slot ≈ 200 个 slot 决策，候选池 ~300 shot。**这个规模不需要 CP-SAT。**

```go
// 三段式：构造 → 局部搜索 → 兜底
func SolveMonth(pool ShotPool, schemas []BeatSchema, cfg Config) (*MonthlySchedule, error) {
    // 阶段 1：GRASP 构造（贪心 + 随机化候选列表），seed 固定 → 确定性
    best := grasp.Construct(pool, schemas, cfg, cfg.Seed)

    // 阶段 2：模拟退火局部搜索，邻域算子：
    //   - swapDaySchema(d1, d2)
    //   - replaceShot(d, k, alt)
    //   - shiftSchema(d, newSchema)
    //   - swapMusic(d1, d2)
    best = anneal.Improve(best, cfg.AnnealIters /*=20000*/, cfg.Seed)

    // 阶段 3：硬约束若仍有违反 → 逐条修复（fallback 降级）
    best = repair.FixHardViolations(best, pool, cfg)

    if v := validate.Hard(best); len(v) > 0 {
        return nil, fmt.Errorf("infeasible after repair: %v", v)
    }
    return best, nil
}
```

**只有当出现"repair 后仍不可行"的真实案例时，才升级到 CP-SAT。** 那时的选项：
- OR-Tools 有 Go 绑定（`ortools/sat/go/cpmodel`，官方 repo 有 Go samples），但官方安装文档只列 C++/Java/.NET/Python —— **Go 绑定不是一等支持，需要 CGO，你自己评估**。
- 更稳的路径：**把 CP-SAT 包成一个 Python 算子**（走 C2 契约），输入 CpModel 的 JSON 描述，输出解。这样 Go 侧零 CGO 负担，且求解器可替换。

> **我的建议是先写模拟退火。** 理由：(1) 纯 Go 零依赖；(2) 天然支持"次优但可行"，比 CP-SAT 的 INFEASIBLE 对业务更友好；(3) 可以随时中断取当前最优，符合"每天必须出片"的可用性要求。

### 阶段 B：日度组装（每天一次，必须快 + 纯确定性）

```go
func PlanDay(sched *MonthlySchedule, d date, ctx Context) (*VideoPlan, error) {
    slot := sched.Day(d)                       // 月度已定 schema + shot 分配
    // 1) 重新校验（素材可能已过期/被隔离）
    if drift := revalidate(slot, ctx); drift.HasBlockers() {
        slot = repatch(slot, ctx)              // 局部重解，不动整月
    }
    // 2) 文案：唯一允许的日度 LLM 调用
    copy := baml.GenerateCopy(ctx.Kernel, slot.Schema, slot.Shots, ctx.RecentNgrams)
    // 3) TTS（可缓存：相同文本 + 相同 voice → 复用）
    vo := tts.Synthesize(copy, ctx.Voice)
    // 4) 强制对齐 → 词级时间戳
    timings := operator.AlignWords(vo, copy)
    // 5) 时长求解：把 beat 时长约束 + VO 实际时长 → 具体帧数
    frames := solveTiming(slot.Schema, timings, slot.Shots)
    // 6) 节拍对齐：把切点吸附到最近的 beat_grid（容差 ±3 帧）
    frames = snapToBeats(frames, ctx.Music.BeatGrid, 3)
    // 7) 布局：negative_space + safe_area → overlay layout_box
    overlays := layout.Solve(slot, frames, ctx.Style, ctx.Canvas.SafeArea)
    // 8) 组装 IR + digest + diversity_signature
    return assemble(...)
}
```

**第 5 步"时长求解"是最容易出 bug 的地方**，单独说：

```
输入：beat 的 duration_range[]、VO 各段实际时长、shot 的可用时长
问题：把 VO 段落映射到 beat，并给每个 clip 定 in/out
约束：Σ clip_duration == VO_total（或 VO 更短则用 B-roll 补）
      每个 clip 在 [beat.min, beat.max] 内
      clip 不超过 shot 的可用长度（含 handles）
方法：这是一个小规模 LP。用简单的比例分配 + 逐个 clamp + 余量再分配即可，
      但必须有"不可满足则降级"路径（缩短 VO / 加速播放 1.0–1.15x / 换更长 shot）
```

**"加速播放最多 1.15x"这个上限要写死**，超过就有明显不自然感。这类"工艺参数"应集中在一个 `craft_params.yaml`，可调可测。

**验收标准**：
- `SolveMonth` P95 <30s（300 shot 池）
- `PlanDay` P95 <5s（不含 TTS/对齐）
- 属性测试 §4.4 全通过
- 非退化多样性测试 §4.6 通过
- `fallback_usage_rate` 在 seed_food_rich 上 <10%，在 seed_beauty_poor 上 <40%

## S6 CompilerService + Renderer

**技术栈**：Go（编译器）+ TypeScript/Remotion（overlay）+ FFmpeg（合成）

**三阶段渲染，这个拆分是成本与确定性的关键**：

```
Stage 1  Remotion → overlay.mov (ProRes 4444 带 alpha, 或 VP9+alpha)
         只渲字幕/角标/动效/AIGC 标识，透明背景
         ★ 1 次 render 计费

Stage 2  FFmpeg → base.mp4
         主视频轨：trim / scale / crop / ken burns / transition / color
         用 filter_complex 一次完成，不落中间文件

Stage 3  FFmpeg → final.mp4
         overlay 合成 + 音频混流（VO + music + ducking）
         + loudnorm(-14 LUFS) + AIGC 隐式元数据注入
```

**为什么不让 Remotion 干全部**：
- Remotion 按 render 计费（$0.01/render，$100/mo 起，Automators 档）
- Remotion 处理长视频轨的性能与内存不如 FFmpeg
- FFmpeg 的确定性更可控
- 分离后 Stage 1 可独立缓存（同 style + 同文案 → 复用）

**成本估算**：100 客户 × 30 条/月 = 3000 renders × $0.01 = $30 → 触底 $100/月。1000 客户 = 30000 renders = $300/月。**可接受，但要提前确认失败重试是否计费**（去问 Remotion，别猜）。

> **注意**：Remotion ≤3 人公司免费。如果你团队 ≤3 人，前期零成本。但要预判到 4 人时的切换。

**Remotion 组件注册表（关键设计）**：

```typescript
// renderer/src/registry.ts —— 白名单 + 每组件独立 zod schema
export const OverlayRegistry = {
  'caption.karaoke': {
    schema: z.object({
      text: z.string().max(60),
      wordTimings: z.array(z.object({ w: z.string(), s: z.number().int(), e: z.number().int() })),
      emphasis: z.array(z.number().int()).default([]),
      style: CaptionStyleSchema,
    }),
    component: KaraokeCaption,
    // 声明式 bbox 预测：布局引擎用它做安全区校验，无需先渲染
    measure: (props, canvas) => measureCaptionBox(props, canvas),
  },
  'badge.price': { /* ... */ },
  'pointer.annotate': { /* ... */ },
  'progress.steps': { /* ... */ },
  'card.terminal_fallback': { /* ... */ },   // ← slot 兜底图形卡
  'aigc.disclosure': { /* ... */ },          // ← 合规显式标识，不可省略
} as const;

export type OverlayComponentId = keyof typeof OverlayRegistry;
```

**`measure()` 函数是核心创新点**：布局引擎（Go 侧）需要在渲染前知道文字会占多大空间，才能做安全区校验。方案：
- TS 侧暴露一个 `POST /measure` 端点（headless，无需 render）
- 或把 measure 逻辑用 opentype.js 实现并编译成 WASM，Go 直接调

后者更好（省一次网络往返，且能在 Planner 内部循环里用）。字体度量必须与实际渲染一致 —— 这也是 R-5（renderer_version 含字体哈希）的原因。

**受约束 TypeScript 的具体约束**（写进 eslint config）：

```jsonc
{
  "rules": {
    "@typescript-eslint/no-explicit-any": "error",
    "@typescript-eslint/no-non-null-assertion": "error",
    "no-restricted-syntax": ["error",
      { "selector": "NewExpression[callee.name='Date']",
        "message": "禁止 Date：破坏渲染确定性。用 IR 中的 frame 号。" },
      { "selector": "CallExpression[callee.object.name='Math'][callee.property.name='random']",
        "message": "禁止 Math.random：用 IR 中的 seed 与确定性 PRNG。" }
    ],
    "no-restricted-imports": ["error", { "patterns": ["*fetch*", "axios"] }]
    // 渲染器不许联网：所有资源必须由 RenderRequest 显式提供
  }
}
```

**这三条禁令（Date / random / network）是渲染确定性的全部来源。** 加进 CI。

**FFmpeg 命令生成**：不要拼字符串，用结构化 filter graph builder：

```go
type FilterGraph struct { nodes []Node; edges []Edge }
func (g *FilterGraph) Build() (args []string, err error)
// 好处：可单元测试（断言生成的 graph 结构）、可可视化、无引号地狱
```

**Golden 测试**：对每个 contract case，断言生成的 FFmpeg args 数组完全一致（这比断言视频输出快 1000 倍，作为快速反馈层）。

## S7 QCService

**技术栈**：Go（断言引擎编排）+ Python 算子（probes）

**架构**：断言引擎 = probe 执行器 + 表达式求值器

```go
type Probe interface {
    ID() string
    Measure(ctx context.Context, subj Subject, args map[string]any) (Measurement, error)
    Cost() CostEstimate
}

type Engine struct { probes map[string]Probe }

func (e *Engine) Run(ctx context.Context, subj Subject, set AssertionSet) (*QCReport, error) {
    // 1) 按 applies_when 过滤适用断言
    // 2) 按 probe 分组、去重（多个断言共用一次 measure）
    // 3) 按 cost 排序：先跑便宜的 L0，遇 BLOCKER 立即短路
    // 4) 每个断言产出 {pass, measured, evidence_uri}
    // 5) 生成 remedy_sheet（模板渲染）
}
```

**"按 probe 分组去重 + 成本排序 + BLOCKER 短路"是 QC 成本控制的核心。** 一个 L0 BLOCKER 失败就不该再跑 L2 的 SyncNet。

**断言集的组织**：

```
assertions/
├── L0_universal.yaml           # 所有素材都跑
├── L1_by_shot_type/*.yaml      # 按制作令 spec 动态生成
├── L2_generated_only.yaml      # applies_when: source_kind in [GEN_*]
├── L3_compliance_base.yaml
└── L3_by_category/*.yaml       # 餐饮/美容/教育... 各一套
```

**示例断言**：

```yaml
- assertion_id: L0.SHARPNESS.min
  level: L0
  severity: BLOCKER
  probe: { op: laplacian_var, args: { sample: N_UNIFORM, n: 8, agg: median } }
  expect: { op: gte, value: 120 }
  remedy:
    action: RESHOOT
    instruction_template: "画面模糊（清晰度 {{measured}}，要求 ≥{{expected}}）。请对焦后重拍，避免逆光与手持晃动。"
    auto_fixable: false

- assertion_id: L1.SUBJECT_PRESENT
  level: L1
  severity: BLOCKER
  probe: { op: object_present, args: { queries: "{{spec.subject}}", detector: yolo_world, conf: 0.35 } }
  expect: { op: is_true }
  remedy:
    action: RESHOOT
    instruction_template: "未检出要求的主体「{{spec.subject}}」。请确保 {{spec.subject}} 在画面中清晰可见且占比 {{spec.framing.subject_area_ratio}}。"

- assertion_id: L1.SAFE_CROP_9x16
  level: L1
  severity: MAJOR
  probe: { op: subject_bbox_within_safe, args: { target_ar: "9:16", margin_ratio: 0.08 } }
  expect: { op: is_true }
  remedy:
    action: AUTO_CROP
    instruction_template: "主体在 9:16 裁切后越界，已自动重定位裁切窗口。"
    auto_fixable: true
    auto_fix_op: auto_crop_recenter

- assertion_id: L2.LIPSYNC
  level: L2
  severity: MAJOR
  applies_when: { op: eq, field: has_lipsync, value: true }
  probe: { op: lipsync_lse_c, args: {} }
  expect: { op: gte, value: 6.0 }        # ← 阈值需在你的真实数据上校准
  remedy:
    action: REGENERATE
    instruction_template: "口型同步不达标（LSE-C {{measured}}）。请重新生成，或降低语速。"

- assertion_id: L3.BANNED_TERMS
  level: L3
  severity: BLOCKER
  probe: { op: banned_terms, args: { sources: [post_text, caption_blocks, ocr_text],
                                     dict: "{{compliance_profile.banned_dict}}" } }
  expect: { op: contains_none }
  remedy:
    action: REWRITE_COPY
    instruction_template: "文案含违禁表达：{{measured.hits}}。已触发改写。"
    auto_fixable: true
    auto_fix_op: rewrite_avoiding_terms
```

**阈值校准方法（无需人工标注）**：

```
1. 收集 200 个"确定可用"和 200 个"确定不可用"的素材
   —— 来源：商家实际发布了 vs 商家删掉了（飞轮！）
2. 对每个 probe 画两组的分布直方图
3. 阈值取 = 使假阳性率 <5% 的分位点（宁可漏检也别错杀，因为错杀会触发无谓返修，
   而返修的边际成本比一条略差的视频高得多）
4. 每月用新数据重算，阈值变更走版本化（assertion_set_version）
```

**假阳性 vs 假阴性的权衡这里要反直觉一次**：素材 QC 上宁可漏检，因为错杀导致返修成本高；但**合规 QC（L3）上必须宁可错杀**，因为合规事故不可逆。所以 L0/L1/L2 与 L3 用相反的阈值策略。

**验收标准**：
- L0 断言集在 200 个已知坏样本上召回 ≥90%，在 200 个已知好样本上假阳性 ≤5%
- L3 合规断言召回 ≥99%（宁可误报）
- 单条视频 QC 成本 ≤ ¥0.15（L0+L1+L3），生成物加 L2 ≤ ¥0.4
- 每个断言的 `remedy.instruction_template` 经人工评审"外包能看懂并执行"

## S8 ComplianceGate

**这是唯一的强制串行门禁**，不允许旁路。放在 QC 之后、Delivery 之前。

```go
type Gate interface {
    Check(ctx context.Context, plan *VideoPlan, art *RenderArtifact) (*GateResult, error)
}

// 顺序执行，任一 BLOCK 即停
var gates = []Gate{
    CategoryAdmissionGate{},   // 类目准入：医疗/医美/金融/招商 → 强制人审或拒接
    BannedTermsGate{},          // 广告法极限词 + 类目资质词
    RequiredDisclaimerGate{},   // 必需声明（如"效果因人而异"）
    ThirdPartyRightsGate{},     // 他人肖像/logo/招牌/车牌
    MusicLicenseGate{},         // 音轨授权凭证存在且未过期
    VoiceAuthorizationGate{},   // 声音克隆授权 ACTIVE
    AIGCLabelGate{},            // 显式 + 隐式标识
    PortraitAuthorizationGate{},// 出镜人授权 ACTIVE
}
```

### AIGC 标识实现

法规事实（已核实）：
- 《人工智能生成合成内容标识办法》**2025-09-01 施行**
- 强制性国标 **GB 45438-2025《网络安全技术 人工智能生成合成内容标识方法》**，同日施行
- 双轨：**显式标识**（文字/声音/图形，用户可明显感知）+ **隐式标识**（文件元数据）
- 全国网安标委（TC260）网站发布了配套**实践指南**，含图片 XMP 的 Python 示例；音视频用 ffmpeg 写元数据

实现方案：

```
显式标识：Remotion 组件 'aigc.disclosure'
  - 位置：画面内、安全区内、非首帧即出现且持续可见（或按国标要求的呈现方式）
  - 同时在 post_text 末尾追加声明文案
  - 该 overlay 由 ComplianceGate 强制注入，不允许 StyleTheme 覆盖或隐藏
  → 写一条测试：任何 aigc_disclosure.required=true 的 plan，
     渲出的视频在指定帧区间必须能 OCR 出声明文字

隐式标识：FFmpeg metadata 注入
  ffmpeg -i final.mp4 -c copy -movflags use_metadata_tags \
    -metadata AIGC='{...}' out.mp4
  → 写一条测试：ffprobe 能读回，且 JSON 可解析、字段完整
```

> ⚠️ **必须由你自己核实的部分**：GB 45438-2025 规定的隐式标识**具体字段名、字段语义、编码格式**，以及"服务提供者 vs 内容传播平台"的责任划分。我在检索中看到一些字段名的二手线索（如 ContentProducer / ContentPropagator / PropagateID / ReservedCode 一类），但**我无法确认其准确性与完整性，不要采信我的转述**。请去：(1) TC260 官网的实践指南原文（含官方示例代码）；(2) 国标正式文本；(3) 让法务过一遍。**你的产品是"工业化批量代产 AI 内容"，这个责任会集中落在你身上而不是商家身上，值得花钱做一次专项合规意见。**

**建议的架构预留**：把隐式标识写成一个独立的 `AIGCLabelWriter` 接口 + 版本化的 field mapping 配置文件。这样标准细则变化或平台加要求时，改配置不改代码。同时预留 C2PA 的接口位（学界与国际标准在往这走，未来可能被要求）。

### 违禁词库冷启动（无人工）

```
1. 爬取《广告法》《反不正当竞争法》条文 + 各地市监局公开处罚决定书
2. LLM 抽取被处罚的具体表述 → 结构化为 {term, category, law_ref, case_ref}
3. 扩展：同义词、谐音变体、拼音首字母、繁体、emoji 替代、拆字
4. 分级：BLOCKER（绝对禁止：最/第一/国家级/无效退款）
        MAJOR（需资质：治疗/抗菌/减肥/投资回报）
        MINOR（建议规避）
5. 检测双保险：AC 自动机（快，处理变体）+ LLM 语义判断（慢，处理隐含宣称）
   —— LLM 只在 AC 未命中但类目高危时才跑，控成本
```

「隐含宣称」是纯词库抓不到的："喝了三个月，我的体检报告变好了" 没有违禁词，但构成功效宣称。这就是需要 LLM 那一层的原因。

## S9 DeliveryService（交付 + 飞轮）

**技术栈**：Go（API）+ TS（小程序）

**核心决策：绝不代发布。** 理由（重申）：抖音开放平台的发布类 scope 需逐项审核、有白名单与企业认证门槛；"第三方批量代发布"是平台风控的高风险特征；小红书基本无对外发布 API；任何群控/自动化脚本会导致封号，商家账号资产归零，你赔不起。

```
交付形态：微信小程序 / 企微机器人
  每日推送"今日视频"
    ├── 视频预览 + 封面
    ├── 文案（可一键复制、可编辑）
    ├── why_this（为什么给你这条）← 提升信任与续订
    ├── 【一键发布】→ 唤起抖音/快手（scheme 或 H5 share）
    ├── 【不发，因为...】→ 受控 reason_code   ★飞轮
    └── 【改一下】→ 局部重生成（记录 JSON Patch）★飞轮
```

**飞轮数据的四层**（按信噪比排序，你的第一飞轮是前三层）：

| 层 | 信号 | 用途 |
|---|---|---|
| 1 | 发布率 / 忽略率（按 BeatSchema、pillar、style 分组） | 范式库的 `perf_stats` 回填；淘汰低发布率 schema |
| 2 | `reason_code` 分布 | 定位系统性缺陷（如 TOO_SALESY 高 → OFFER beat 权重过大） |
| 3 | JSON Patch 编辑轨迹 | 最高密度：商家改了哪句、删了哪个镜头 → 直接监督信号 |
| 4 | 平台 metrics（T+1，粗颗粒） | 弱监督，只做长周期校准，**不做日常决策** |

**必须实现的三个物化视图**：

```sql
-- 1) 范式效能
CREATE MATERIALIZED VIEW schema_performance AS
SELECT p.beat_schema_id, p.beat_schema_version,
       count(*) AS delivered,
       sum((e.kind='PLAN_PUBLISHED')::int)::float / count(*) AS publish_rate,
       sum((e.kind='COPY_EDITED')::int)::float / count(*) AS edit_rate,
       avg(jsonb_array_length(e.payload->'edits')) AS avg_edit_ops
FROM video_plans p LEFT JOIN events e ON e.plan_id = p.id
GROUP BY 1,2;

-- 2) 素材效能（哪些 shot 总被删）
CREATE MATERIALIZED VIEW shot_rejection AS
SELECT s.id, s.shot_type, s.scene,
       count(*) FILTER (WHERE e.kind='CLIP_REMOVED') AS removed_count,
       s.use_count,
       count(*) FILTER (WHERE e.kind='CLIP_REMOVED')::float / greatest(s.use_count,1) AS reject_rate
FROM shots s LEFT JOIN events e ON e.payload->>'shot_id' = s.id::text
GROUP BY 1,2,3;

-- 3) 文案编辑模式（喂给 BAML few-shot）
CREATE MATERIALIZED VIEW copy_edit_patterns AS
SELECT p.tenant_id, jsonb_array_elements(e.payload->'edits') AS patch,
       p.ir->'copy'->'caption_blocks' AS original
FROM events e JOIN video_plans p ON p.id = e.plan_id
WHERE e.kind='COPY_EDITED';
```

**视图 3 是飞轮的最高价值产物**：商家的真实改写就是最好的 few-shot 示例。做法：定期把高频编辑模式提炼成 BAML 的 few-shot examples 或负例约束（"避免这类表达"），实现 prompt 的数据驱动迭代。

**关于 `reject_rate` 的一个反直觉洞察**：某个 shot 反复被删，不一定是 shot 差，可能是**它被用在了错误的 beat_role 上**。所以视图要按 `(shot_id, beat_role)` 联合统计，不只按 shot_id。这个区分能让你从"删掉素材"升级到"修正取材规则"。

## S10 CostGovernor + Observability

**这是你 UE 的守门人，必须是一等服务而不是监控面板。**

```go
type Governor interface {
    // 事前：预留额度。拿不到就必须降级或拒绝
    Reserve(ctx context.Context, tenantID uuid.UUID, est CostEstimate) (*Reservation, error)
    // 事后：实际计费
    Settle(ctx context.Context, res *Reservation, actual CostActual) error
    // 决策：当前该用哪一档
    Tier(ctx context.Context, tenantID uuid.UUID) (Tier, error)
}

type Tier int
const (
    TierFull      Tier = iota  // 允许 VLM 复核、LLM 改写、L2 QC
    TierEconomy                 // 关闭 VLM 复核，降级到小模型，L2 抽检
    TierMinimal                 // 纯确定性：模板文案 + 已有素材 + 无 LLM
)
```

**关键设计：Planner 必须接受 `Tier` 作为输入并改变行为。** 不是"超预算就报警"，是"超预算就走 TierMinimal 仍然交付"。因为"每天都发布"是你的核心承诺，不能因为预算问题断更。

`TierMinimal` 必须被 E2E 测试覆盖：**在 TierMinimal 下跑 30 天，仍然要通过所有硬约束和多样性测试。** 这条测试会逼你把确定性能力做扎实 —— 而这恰好是你的成本护城河。

**成本模型（每客户每月，需用真实数据校准）**：

| 项 | Onboarding（一次性） | 日常（月） |
|---|---|---|
| 访谈 LLM | ¥8 | 0 |
| 素材打标（VLM） | ¥60–120（200–400 shot） | ¥10（补拍增量） |
| 范式匹配 | ¥5 | 0 |
| 文案生成 | 0 | ¥15（30 条） |
| TTS | ¥5 | ¥20 |
| 强制对齐 | ¥2 | ¥6 |
| 渲染（Remotion+FFmpeg） | 0 | ¥25 |
| QC | ¥25 | ¥12 |
| 存储/CDN | ¥5 | ¥15 |
| **小计** | **¥110–170** | **¥103** |

3 个月总成本 ≈ ¥110–170 + ¥309 ≈ **¥420–480**（不含外包拍摄与人工兜底）。收入 ¥3000–4500。

**剩下的空间全部用于**：摄影师上门（这是最大单项，¥800–1500）+ 人工兜底质检 + 客服。所以 `HumanInterventionRate` 才是生死线，比算力成本重要得多 —— 这也是为什么它要成为 E2E 测试断言。

**必须打点的 12 个指标**：

```
业务：publish_rate, edit_rate, reject_rate, reason_code 分布, 续订率
质量：qc_pass_rate(by level), fallback_usage_rate, diversity_score, 
      merchant_shot_reject_rate
成本：cost_per_video, cost_per_tenant_month, human_intervention_minutes
可靠：daily_delivery_success_rate（必须 >99.5%，这是承诺）
```

## S11 ReshootService（UGC 反向采集）★ 我认为这是你最该优先建的技术资产

因为它同时解决**成本、时效、续订**三件事，而且它复用了你已有的 L0/L1 质检器。

```
触发条件（自动）：
  - shot.ttl_at 到期
  - season 不匹配当前月
  - linked_campaign 结束
  - reject_rate > 阈值（商家反复删这个镜头）
  - 某 pillar 的可用 shot 数 < 阈值（导致 fallback 率上升）

产出：MerchantShootTask
  ├── shot_list（3–8 个，一次拍完，不超过 15 分钟）
  ├── 每个 item 含：
  │     ├── 构图示意图（AI 生成的线稿/示意图，不是文字描述）★
  │     ├── 一句话拍法（"手机竖着，离菜 30 公分，从左往右慢慢平移，数 5 秒"）
  │     ├── 时长要求 + handles 提示（"动作开始前先拍 1 秒静止"）
  │     └── 反例图（"不要这样：手抖、逆光、拍到隔壁桌客人"）
  └── 上传后自动跑 L0 + L1（用 ProductionOrder 的同一套断言）
        ├── PASS → 直接入库，当天可用
        └── FAIL → 返回具体、可执行的返修指令（不是"质量不合格"）
```

**返修指令的质量决定这个功能成败。** 对比：

- ❌ "视频清晰度不达标，请重新拍摄"
- ✅ "第 2 个镜头（切牛肉特写）有点模糊。原因可能是离得太近手机对不上焦。**试试退后一点点，点一下屏幕上的牛肉让它对上焦，再拍一次**。"

后者需要：`defect_type` → `probable_cause` → `layman_action` 的映射表。**这张表是纯人工经验，但只需写一次（30–40 条），是极高杠杆的资产。** 建议在冷启动的 5 个真实客户上专门磨这张表。

**验收标准**：
- 商家自拍的一次通过率 ≥60%（第一版），≥75%（迭代后）
- 从触发到入库的中位时长 ≤48h
- 补拍任务的商家完成率 ≥70%（这个数字直接反映指令是否可执行）

---

# §6 开源栈与资源清单

## 6.1 分层技术栈

| 层 | 选型 | 备注 |
|---|---|---|
| **控制面** | Go 1.23+ | 单体优先（modular monolith），别过早微服务 |
| HTTP/RPC | `net/http` + chi；内部 gRPC（connect-go 更简洁） | |
| 队列 | River（Postgres 原生，Go） 或 Asynq（Redis） | River 更省一个组件 |
| DB | PostgreSQL 16 + pgvector | 向量、JSONB、队列一把梭；别一开始上 Qdrant |
| 对象存储 | MinIO（自托管）/ S3 兼容 | |
| Schema/校验 | JSON Schema + `santhosh-tekuri/jsonschema`（Go） + zod（TS） | |
| **LLM 层** | BAML | Go 原生 client（`baml init --client-type go`）；TS 原生 |
| VLM | Qwen3-VL（vLLM 本地）+ 云 API 兜底 | 冷启动直接用云 API |
| **算子层** | Python 3.11 + Docker，每算子独立镜像 | 严格遵守 C2 契约，无状态 |
| **渲染层** | Remotion 4.x（overlay）+ FFmpeg 7.x（合成） | 见 §5.6 三阶段 |
| Node | 22 LTS，pnpm，严格 TS | |
| **求解器** | 自研 GRASP + 模拟退火（Go） | 需要时再上 OR-Tools（Python 算子形态） |
| **观测** | OpenTelemetry + Prometheus + Grafana；Loki | |
| **CI** | GitHub Actions + 固定容器镜像（渲染确定性依赖它） | |

## 6.2 关键开源项目

**视频/CV**
| 项目 | 用途 | 注意 |
|---|---|---|
| PySceneDetect | 镜头切分 | content detector，阈值需按垂类调 |
| TransNetV2 | 更准的镜头边界 | 需 GPU；PySceneDetect 不够时才上 |
| Qwen3-VL | 视觉理解与打标 | 权重开放；vLLM 部署 |
| InternVL | VLM 交叉验证 | 用于 golden set 共识法 |
| YOLO-World | 开放词表检测 | 快，适合 L1 断言 |
| GroundingDINO | 开放词表检测（更准） | 慢，用于抽检 |
| SAM2 | 视频分割 | 打码/去 logo |
| InsightFace | 人脸检测与识别 | 第三方人脸合规 |
| PaddleOCR | 中文 OCR | 招牌/字幕/违禁词检测 |
| RAFT | 光流 | 抖动、时序闪烁 |
| DOVER / FastVQA | 无参考视频质量 | L2 "塑料感"检测 |
| SyncNet | 口型同步评分 | LSE-C / LSE-D |

**音频**
| 项目 | 用途 |
|---|---|
| WhisperX | 词级强制对齐（多语言） |
| FunASR | 中文 ASR + 对齐（中文更稳，优先） |
| pyloudnorm | LUFS 测量 |
| Silero VAD | 语音活动检测 |
| librosa / madmom | BPM + 节拍网格（madmom 更准） |
| ffmpeg loudnorm | 响度归一化 |

**基础设施与工程**
| 项目 | 用途 |
|---|---|
| Remotion | 动效/字幕渲染（注意授权） |
| FFmpeg 7.x | 一切转码/合成/滤镜 |
| OpenTimelineIO | **仅作导出**（给人类剪辑师）；Python 侧转换 |
| pgvector | 向量检索 |
| River | Go 原生 Postgres 队列 |
| pgregory.net/rapid | Go property-based testing |
| santhosh-tekuri/jsonschema | Go JSON Schema 校验 |
| opentype.js | 字体度量（WASM，给布局引擎用） |
| go-cmp | golden 测试的结构化 diff |

**替代方案备注**
- Remotion 的 MIT 替代是 Motion Canvas，但生态与 AI 可编程性差很多。**建议直接付费用 Remotion**，$100–300/月在你的成本结构里不是问题。
- 不要自己写视频合成引擎。FFmpeg filter_complex 能覆盖你 95% 的需求。

## 6.3 音乐版权（隐藏地雷，单独说）

**优先级**：
1. **平台官方免费曲库**（抖音"音乐库-可商用"）—— 最安全，且有流量加成。**但需要商家账号内操作，无法在你的渲染管线里预混。**
2. 解法：**IR 支持"静音音轨 + 卡点标记"模式**。视频渲出时不带 BGM，商家在抖音发布器里选官方音乐，你的卡点已经按标准 BPM（如 120）对齐。这个设计既合规又拿到流量加成。
3. 需要预混 BGM 时，用商业授权曲库（Artlist / Epidemic Sound 的商用授权，注意授权范围是否覆盖"为客户制作"）或 CC0。
4. `audio_tracks.license_proof_uri` 的 DB 约束（见 §2.3）强制每条音轨有授权凭证。

> **"静音 + 卡点标记"这个方案我认为你应该作为默认路径。** 它同时解决版权、平台流量加成、以及"商家参与感"三个问题。代价是失去部分渲染控制力（ducking 做不了），但对 ¥1000/月的产品完全可接受。

---

# §7 实施路线图

## Phase 0：契约冻结（Week 1–6，2 人）

**产出全是文档与测试，一行业务代码都不写。**

```
Week 1-2  受控词表 v1（14 张 YAML）+ codegen 流水线
          Shot/Asset schema + testdata（valid/invalid/evolution）
          C2 算子协议 + FakeRunner + 第一批 golden（用 20 个真实素材手工产出）
Week 3-4  VideoPlan IR v1 ★ 最重要
          ShotSlotQuery 谓词 AST + 编译器 + property test
          C3 渲染契约 + 8 个 contract case（先只有 plan.json，无 golden）
Week 5-6  QCAssertion DSL + 前 30 条 L0/L1 断言
          BrandKernel schema
          Freeze Gate 评审（§4.8 的 12 项）
```

**Phase 0 的 Definitions of Done**：`make gen && make test` 全绿，且能用手写的 `plan.json` 渲出一条视频（哪怕素材是假的）。

## Phase 1：垂直切片（Week 7–14，2–3 人）

**选 1 个垂类。我建议社区餐饮**：素材形态最稳定、合规风险低、门店密度高（摄影师效率高）、内容支柱清晰。

```
Week 7-8    Ingest 流水线（S2）+ 12 个 Python 算子（先只要 6 个：probe/
            shot_segment/tech_metrics/audio_metrics/crop_analyze/face_logo_ocr）
Week 9-10   AssetQueryEngine（S3）+ 手写 10 个 BeatSchema（不做蒸馏，先手写）
            Planner 阶段 B（PlanDay）—— 先不做月度优化，每天贪心
Week 11-12  Compiler + Renderer（S6）三阶段 + Remotion 组件（先 5 个）
            QC L0 + L1（S7）
Week 13-14  ComplianceGate（S8）+ Delivery 小程序（S9）
            端到端：1 个真实商家，人工全程审核，跑 14 天
```

**Phase 1 目标不是自动化率，是"跑通一次完整闭环并且不出事故"。** 允许 50% 人工干预。

## Phase 2：5 客户 × 3 个月（Week 15–26）

**这一轮的产出不是收入，是三样资产**（我在前面强调过，这里给具体交付物）：

1. **BeatSchema 库**：从 10 个手写扩到 30 个（此时上蒸馏流水线 S4）
2. **QC 断言库**：从 30 条扩到 120 条，**全部从真实翻车案例里长出来**。建立纪律：每次人工发现问题，必须新增一条断言 + 一个 golden 反例。
3. **外包验收协议**：ProductionOrder 模板 + `defect→cause→action` 映射表（S11 的 30–40 条）

同时上线：
- Planner 阶段 A（月度全局排期 + 模拟退火）
- ReshootService（S11）★ 优先级高于你想象
- CostGovernor（S10）+ TierMinimal 路径
- 飞轮三个物化视图

**Phase 2 的关键指标**：`HumanInterventionRate` 从 50% 降到 <20%。降不下来就不要进 Phase 3。

## Phase 3：规模化（Week 27+）

- 第 2、3 个垂类（复用架构，只新增词表分表 + BeatSchema）
- 平台数据接入（弱监督校准）
- 正式 ISV 资质申请（走官方发布链路）
- 类目分层定价（高合规成本类目加价）

---

# §8 我认为你还需要处理的问题

## 8.1 必须由你自己核实（我不能替你决定）

| 事项 | 为什么重要 | 去哪核实 |
|---|---|---|
| **GB 45438-2025 隐式标识的具体字段规范** | 直接决定 ComplianceGate 实现；我看到的字段名是二手信息，不可信 | TC260 官网实践指南原文 + 国标正式文本 + 法务 |
| **"批量代产 AI 内容的服务提供者"的法律定位** | 责任划分决定你的风险敞口和保险需求 | 专项法律意见（值得花钱） |
| **Remotion 失败重试是否计费** | 影响成本模型 | 直接问 Remotion（他们有 20 分钟评估通话） |
| **抖音开放平台发布类 scope 的实际审核门槛** | 决定 Phase 3 能否走官方链路 | 开放平台文档 + 申请一次试试 |
| **商业音乐授权是否覆盖"为第三方客户制作"** | 曲库授权常排除"代理服务" | 逐个曲库看条款 |
| **OTIO 是否有可用的 Go 绑定** | 影响 S6 导出实现（不影响主链路） | OTIO GitHub |
| **OR-Tools Go 绑定的成熟度** | 影响 Planner 升级路径 | or-tools repo 的 `ortools/sat/go` |

## 8.2 我认为还有三个你和我都还没充分处理的难点

**① 商家的"事实漂移"没有检测机制。**
菜单改了、涨价了、活动结束了、店长换人了 —— BrandKernel 里的事实会静默过期，导致你产出**事实错误**的视频。这比"不好看"严重得多（可能构成虚假宣传）。

建议：给 BrandKernel 的每个事实性字段加 `verified_at` + `verify_interval`，到期在交付卡片里做**轻量确认**（"这个价格还对吗？对/改"一键）。把事实核验融入日常交付，而不是单独做一次月度回访。这几乎零成本，且是防止事故的唯一实用手段。

**② 30 天连续发布的"叙事疲劳"没有建模。**
你的多样性约束都是**形式层面**的（结构、pHash、n-gram）。但商家账号还有**语义层面**的疲劳：连续 10 天都在讲"食材新鲜"，即使形式各异，观众也会腻。

建议：在 `diversity_signature` 里加一维 `semantic_topic_embedding`，约束"近 7 天的话题 embedding 平均余弦相似度 < T"。这是对现有约束系统的小扩展，但覆盖了一个真实的失效模式。

**③ 你的第一个真实事故必然发生在"人工兜底路径"上。**
所有自动化系统的事故都在人工介入的边界发生：运营手动改了个 plan 但绕过了 ComplianceGate。

建议：**人工干预必须走同一条管线**。运营的编辑操作只能产生新的 VideoPlan（走完整 Gate 链），不允许直接改 RenderArtifact 或直接交付。写成架构约束 + 一条 E2E 测试（"模拟运营手动改稿，断言 ComplianceGate 仍被执行"）。

---

如果你要继续往下推进，我建议下一步选一个具体展开：

- **A. 把受控词表 v1 的 14 张 YAML 全部写出来**（餐饮垂类，可直接用）
- **B. 把 VideoPlan IR 的 Go 类型 + 校验器 + JCS digest 实现写出来**
- **C. 把 QC 断言库的前 40 条写出来**（L0 + L1，含阈值初值与 remedy 模板）
- **D. 把 Renderer 的三阶段实现写出来**（Remotion 组件 + FFmpeg filter graph builder + contract test 骨架）
- **E. 把 Planner 的模拟退火求解器写出来**（含邻域算子与 property test）

我的建议顺序是 **A → B → E → C → D**：A 和 B 是所有东西的地基，E 是你的核心壁垒且最需要设计力，C 和 D 更接近"有耐心就能做完"的工程量。

