package compliance

import (
	"encoding/json"
	"fmt"

	"github.com/Cloudbird-Software/Shorts_Director/internal/videoplan"
)

// ImplicitLabelVersion 是隐式标识字段映射的版本（GB 45438-2025）。
// 具体字段名以 TC260 官方实践指南为准并在此版本化：
// 标准细则变化时改 mapping 配置（加版本号），不改 Gate 代码。
const ImplicitLabelVersion = "2025.gb45438.v1"

// implicitLabelFields 是 v1 字段集：元数据 AIGC 块必须全部出现。
// ⚠ 字段语义待 TC260 指南原文核验（Engineering_plan §S8 风险清单），
// 版本常量 + 映射表的存在就是为那天准备的不改代码切换路径。
var implicitLabelFields = []string{
	"ContentProducer", // 生成服务提供者
	"ProduceTime",     // 生成时间
	"Identifier",      // 内容标识
}

// DisclosureRequired 判定 AIGC 标识义务是否成立：
// 含生成素材（GENERATED clip）或生成声音（TTS VO）即触发（§S8 双轨）。
func DisclosureRequired(p *videoplan.Plan) bool {
	if p.Audio.VORef != nil {
		return true
	}
	for _, t := range p.Tracks {
		for _, c := range t.Clips {
			if c.Source.Kind == "GENERATED" {
				return true
			}
		}
	}
	return false
}

// FindDisclosureOverlay 返回可见 aigc.disclosure overlay 的 id；
// "" 表示缺失。可见 = 帧跨度 ≥1（IV-VP-3 已保证不越总时长）。
func FindDisclosureOverlay(p *videoplan.Plan) string {
	for _, o := range p.Overlays {
		if o.Component == "aigc.disclosure" && o.EndFrame > o.StartFrame {
			return o.OverlayID
		}
	}
	return ""
}

// EnsureAIGCDisclosure 强制注入显式标识（ComplianceGate 前置步骤）：
// 缺则补 aigc.disclosure overlay 并在 post_text 末尾追加声明文案。
// 返回注入的 overlay id（已存在时返回既有 id，幂等）。
// 该 overlay 由 Gate 强制注入，不允许 StyleTheme 覆盖或隐藏。
func EnsureAIGCDisclosure(p *videoplan.Plan, disclosureText string) string {
	if id := FindDisclosureOverlay(p); id != "" {
		return id
	}
	total := p.TotalFrames()
	if total <= 0 {
		total = 1
	}
	// 全程右下角常显：安全区内（safe_area 右/下留白），BC 锚点。
	id := fmt.Sprintf("aigc_auto_%s", p.PlanID)
	p.Overlays = append(p.Overlays, videoplan.Overlay{
		OverlayID:  id,
		Intent:     "COMPLIANCE",
		Component:  "aigc.disclosure",
		Props:      map[string]any{"text": disclosureText, "font": "HarmonyOS_Sans_Regular"},
		StartFrame: 0,
		EndFrame:   total,
		LayoutBox: videoplan.LayoutBox{
			X: p.Canvas.W - p.Canvas.SafeArea.Right - 320,
			Y: p.Canvas.H - p.Canvas.SafeArea.Bottom - 60,
			W: 320, H: 60, Anchor: "BC",
		},
	})
	if p.Copy.PostText != "" {
		p.Copy.PostText += " "
	}
	p.Copy.PostText += disclosureText
	return id
}

// HasImplicitLabel 判定产物元数据是否携带完整的 AIGC 隐式标识块。
// metadata 键为 ffprobe 读回的全量标签；AIGC 块须为可解析 JSON 且字段齐备。
func HasImplicitLabel(metadata map[string]string) bool {
	raw, ok := metadata["AIGC"]
	if !ok || raw == "" {
		return false
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return false
	}
	for _, f := range implicitLabelFields {
		v, ok := m[f]
		if !ok || v == nil || v == "" {
			return false
		}
	}
	return true
}

// BuildImplicitLabel 构造隐式标识元数据块（渲染管线 Stage 3 注入用）。
func BuildImplicitLabel(producer, produceTime, identifier string) map[string]string {
	m := map[string]any{}
	for i, f := range implicitLabelFields {
		switch i {
		case 0:
			m[f] = producer
		case 1:
			m[f] = produceTime
		case 2:
			m[f] = identifier
		}
	}
	raw, _ := json.Marshal(m)
	return map[string]string{"AIGC": string(raw), "AIGC_LABEL_VERSION": ImplicitLabelVersion}
}

// WriteBack 生成 plan.compliance 回写块（Gate 通过后由调用方写入）。
func WriteBack(p *videoplan.Plan, res *GateResult) *videoplan.ComplianceResult {
	c := &videoplan.ComplianceResult{ChecksPassed: res.ChecksPassed}
	if DisclosureRequired(p) {
		c.AIGCDisclosure.Required = true
		if id := FindDisclosureOverlay(p); id != "" {
			c.AIGCDisclosure.ExplicitOverlayID = &id
		}
	}
	p.Compliance = c
	return c
}
