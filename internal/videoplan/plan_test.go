package videoplan

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadPlan(t *testing.T, dir, name string) Plan {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "schema", "testdata", "video_plan", dir, name))
	if err != nil {
		t.Fatalf("读样本失败: %v", err)
	}
	var p Plan
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("解析 %s: %v", name, err)
	}
	return p
}

// TestValidSamplesPassValidate：G1 valid 样本必须全部通过 IV-VP 校验。
func TestValidSamplesPassValidate(t *testing.T) {
	for _, name := range []string{
		"minimal_music_plan.json", "silent_mode.json", "with_vo_and_speed.json",
		"commercial_music_proof.json", "generated_clip.json",
	} {
		p := loadPlan(t, "valid", name)
		if err := p.Validate(); err != nil {
			t.Errorf("valid 样本 %s 未通过 Validate: %v", name, err)
		}
	}
}

func basePlan(t *testing.T) Plan {
	t.Helper()
	return loadPlan(t, "valid", "with_vo_and_speed.json")
}

func TestIVVP1MainTrackTiling(t *testing.T) {
	// 主轨插入重叠 clip ⇒ 不无缝。
	p := basePlan(t)
	p.Tracks[0].Clips = append(p.Tracks[0].Clips, Clip{
		ClipID: "dup", BeatRole: "PROOF", Source: ClipSource{Kind: "COLOR", Ref: "#000000"},
		SrcIn: 0, SrcOut: 10, TlStart: 50, TlEnd: 75,
		Transform:    Transform{Crop: &Crop{}, Scale: 1, Position: &Position{}},
		TransitionIn: TransitionIn{Kind: "CUT"},
	})
	err := p.Validate()
	if err == nil || !strings.Contains(err.Error(), "IV-VP-1") {
		t.Errorf("IV-VP-1 重叠未触发: %v", err)
	}

	// 主轨首段不从 0 开始 ⇒ 破坏铺满。
	p = basePlan(t)
	p.Tracks[0].Clips[0].TlStart = 10
	if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "IV-VP-1") {
		t.Errorf("IV-VP-1 起点未触发: %v", err)
	}

	// 非主轨 clip 超总时长。
	p = basePlan(t)
	p.Tracks[1].Clips = append(p.Tracks[1].Clips, Clip{
		ClipID: "late", BeatRole: "CTA", Source: ClipSource{Kind: "COLOR", Ref: "#000000"},
		SrcIn: 0, SrcOut: 10, TlStart: 70, TlEnd: 90,
		Transform:    Transform{Crop: &Crop{}, Scale: 1, Position: &Position{}},
		TransitionIn: TransitionIn{Kind: "CUT"},
	})
	if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "IV-VP-1") {
		t.Errorf("IV-VP-1 越总时长未触发: %v", err)
	}
}

func TestIVVP2NonPositiveSpan(t *testing.T) {
	p := basePlan(t)
	p.Tracks[0].Clips[0].SrcOut = p.Tracks[0].Clips[0].SrcIn // src 跨度 0
	err := p.Validate()
	if err == nil || !strings.Contains(err.Error(), "IV-VP-2") {
		t.Errorf("IV-VP-2 src 未触发: %v", err)
	}
	p = basePlan(t)
	p.Tracks[0].Clips[0].TlEnd = p.Tracks[0].Clips[0].TlStart // tl 跨度 0
	if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "IV-VP-2") {
		t.Errorf("IV-VP-2 tl 未触发: %v", err)
	}
}

func TestIVVP3CaptionBeyondTotal(t *testing.T) {
	p := basePlan(t)
	p.Copy.CaptionBlocks[0].EndFrame = p.TotalFrames() + 10
	if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "IV-VP-3") {
		t.Errorf("IV-VP-3 caption 未触发: %v", err)
	}
}

func TestIVVP3OverlayBeyondSafeArea(t *testing.T) {
	p := basePlan(t)
	// BC 锚点、y 贴到底部边界之外（safe_area.bottom=400）。
	p.Overlays[0].LayoutBox.Y = p.Canvas.H - 100
	if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "IV-VP-3") {
		t.Errorf("IV-VP-3 safe_area 未触发: %v", err)
	}
	// 越出总时长的 overlay。
	p = basePlan(t)
	p.Overlays[0].EndFrame = p.TotalFrames() + 5
	if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "IV-VP-3") {
		t.Errorf("IV-VP-3 区间未触发: %v", err)
	}
}

func TestIVVP4ComponentWhitelist(t *testing.T) {
	p := basePlan(t)
	p.Overlays[0].Component = "custom.unknown"
	if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "IV-VP-4") {
		t.Errorf("IV-VP-4 未触发: %v", err)
	}
}

func TestSpeedCraftCap(t *testing.T) {
	p := basePlan(t)
	p.Tracks[0].Clips[0].Speed = 1.5 // 超 1.15 工艺上限
	if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "speed") {
		t.Errorf("speed 上限未触发: %v", err)
	}
}

func TestTimebaseMustBeFrames(t *testing.T) {
	p := basePlan(t)
	p.Timebase.Unit = "seconds" // float 秒进入时基
	if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "frame") {
		t.Errorf("时基校验未触发: %v", err)
	}
	p = basePlan(t)
	p.Timebase.Rate = 30 // 与 canvas.fps=25 不一致
	if err := p.Validate(); err == nil {
		t.Error("rate/fps 不一致应拒绝")
	}
}

func TestLicensedRefProofRequired(t *testing.T) {
	p := basePlan(t)
	p.Audio.MusicRef.LicenseKind = "COMMERCIAL"
	p.Audio.MusicRef.LicenseProofURI = "" // 非平台曲库无凭证
	if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "license_proof_uri") {
		t.Errorf("授权凭证校验未触发: %v", err)
	}
}

func TestTotalFramesAndDescribe(t *testing.T) {
	p := basePlan(t)
	if got := p.TotalFrames(); got != 75 {
		t.Errorf("TotalFrames = %d, want 75", got)
	}
	d := p.Describe()
	if !strings.Contains(d, "total=75f") || !strings.Contains(d, "bs.food.origin_story") {
		t.Errorf("Describe = %q", d)
	}
}

// TestEvolutionSamplesConsumable：G5 向后兼容——evolution/ 样本必须仍能
// 反序列化并通过 IV-VP 校验（v2 破坏性变更时的回归基线，禁止改写样本）。
func TestEvolutionSamplesConsumable(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join("..", "..", "schema", "testdata", "video_plan", "evolution"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		p := loadPlan(t, "evolution", e.Name())
		if err := p.Validate(); err != nil {
			t.Errorf("evolution 样本 %s 未通过 Validate: %v", e.Name(), err)
		}
	}
}
