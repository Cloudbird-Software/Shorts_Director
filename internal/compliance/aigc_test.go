package compliance

import (
	"strings"
	"testing"

	"github.com/Cloudbird-Software/Shorts_Director/internal/videoplan"
)

// TestDisclosureRequired：GENERATED clip 或 TTS VO 触发义务。
func TestDisclosureRequired(t *testing.T) {
	in := passingInput(t) // with_vo_and_speed：TTS VO ⇒ true
	if !DisclosureRequired(in.Plan) {
		t.Error("含 TTS VO 应触发标识义务")
	}

	in.Plan.Audio.VORef = nil
	if DisclosureRequired(in.Plan) {
		t.Error("纯实拍+库音乐不应触发义务")
	}

	in.Plan.Tracks[0].Clips[0].Source.Kind = "GENERATED"
	if !DisclosureRequired(in.Plan) {
		t.Error("GENERATED clip 应触发义务")
	}
}

// TestEnsureAIGCDisclosureIdempotent：注入幂等，且 post_text 追加一次。
func TestEnsureAIGCDisclosureIdempotent(t *testing.T) {
	in := passingInput(t)
	EnsureAIGCDisclosureCleanup(in.Plan)
	base := in.Plan.Copy.PostText

	id1 := EnsureAIGCDisclosure(in.Plan, "AI 生成内容")
	id2 := EnsureAIGCDisclosure(in.Plan, "AI 生成内容")
	if id1 == "" || id1 != id2 {
		t.Errorf("幂等注入应返回同一 id: %q vs %q", id1, id2)
	}
	want := base + " AI 生成内容"
	if in.Plan.Copy.PostText != want {
		t.Errorf("post_text = %q, want %q", in.Plan.Copy.PostText, want)
	}
	if FindDisclosureOverlay(in.Plan) != id1 {
		t.Error("注入后应能找到显式标识 overlay")
	}
}

// TestImplicitLabelRoundTrip：构造 ⇄ 判定闭环；缺字段/坏 JSON 拒绝。
func TestImplicitLabelRoundTrip(t *testing.T) {
	meta := BuildImplicitLabel("producer-a", "2026-08-19T00:00:00Z", "plan-1")
	if !HasImplicitLabel(meta) {
		t.Fatal("完整字段应通过")
	}
	if v, ok := meta["AIGC_LABEL_VERSION"]; !ok || v != ImplicitLabelVersion {
		t.Errorf("版本标签缺失: %v", meta)
	}

	// 缺 AIGC 键。
	if HasImplicitLabel(map[string]string{}) {
		t.Error("无 AIGC 键应拒绝")
	}
	// 坏 JSON。
	if HasImplicitLabel(map[string]string{"AIGC": "{oops"}) {
		t.Error("坏 JSON 应拒绝")
	}
	// 字段不齐（手工构造缺 Identifier）。
	if HasImplicitLabel(map[string]string{"AIGC": `{"ContentProducer":"p","ProduceTime":"t"}`}) {
		t.Error("缺字段应拒绝")
	}
	// 空值字段。
	if HasImplicitLabel(map[string]string{"AIGC": `{"ContentProducer":"p","ProduceTime":"","Identifier":"i"}`}) {
		t.Error("空值字段应拒绝")
	}
}

// TestEnsureDisclosureLayoutInSafeArea：注入的 layout_box 落在安全区内。
func TestEnsureDisclosureLayoutInSafeArea(t *testing.T) {
	in := passingInput(t)
	EnsureAIGCDisclosureCleanup(in.Plan)
	id := EnsureAIGCDisclosure(in.Plan, "AI 生成内容")
	var box videoplan.LayoutBox
	for _, o := range in.Plan.Overlays {
		if o.OverlayID == id {
			box = o.LayoutBox
		}
	}
	sa := in.Plan.Canvas.SafeArea
	if box.X < sa.Left || box.X+box.W > in.Plan.Canvas.W-sa.Right {
		t.Errorf("overlay 越左右安全区: %+v", box)
	}
	if box.Y < sa.Top || box.Y+box.H > in.Plan.Canvas.H-sa.Bottom {
		t.Errorf("overlay 越上下安全区: %+v", box)
	}
	if err := in.Plan.Validate(); err != nil {
		t.Errorf("注入后 plan 应整体合法: %v", err)
	}
}

// TestGateResultSummary：摘要可读且确定。
func TestGateResultSummary(t *testing.T) {
	in := passingInput(t)
	in.Plan.Copy.CaptionBlocks[0].Text = "全网第一"
	res := Chain(nil, StandardGates(), in)
	s := res.Summary()
	if !strings.Contains(s, "decision=BLOCK") || !strings.Contains(s, "skipped=6") {
		t.Errorf("summary = %q", s)
	}
}
