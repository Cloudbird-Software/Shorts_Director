package entity

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// shotTestdataDir 指向 schema/testdata/shot（G1 冻结样本，TS 侧 harness 同源）。
func shotTestdataDir() string {
	return filepath.Join("..", "..", "schema", "testdata", "shot")
}

func loadShot(t *testing.T, sub, name string) Shot {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(shotTestdataDir(), sub, name))
	if err != nil {
		t.Fatalf("读样本失败: %v", err)
	}
	var s Shot
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("解析 %s/%s: %v", sub, name, err)
	}
	return s
}

// TestShotValidSamples：全部 valid 样本必须通过不变式校验。
func TestShotValidSamples(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join(shotTestdataDir(), "valid"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) < 5 {
		t.Fatalf("valid 样本少于 5 个：%d", len(entries))
	}
	for _, e := range entries {
		if s := loadShot(t, "valid", e.Name()); s.Validate() != nil {
			t.Errorf("valid 样本 %s 未通过 Validate: %v", e.Name(), s.Validate())
		}
	}
}

// TestShotFSM：合法迁移全过，非法迁移全拒。
func TestShotFSM(t *testing.T) {
	legal := []struct{ from, to ShotState }{
		{ShotUploaded, ShotSegmented},
		{ShotSegmented, ShotTagged},
		{ShotTagged, ShotRejected},
		{ShotTagged, ShotAvailable},
		{ShotAvailable, ShotCooling},
		{ShotCooling, ShotAvailable},
		{ShotAvailable, ShotExpired},
		{ShotAvailable, ShotQuarantine},
		{ShotQuarantine, ShotAvailable}, // 人工放行
		{ShotRejected, ShotUploaded},    // 重拍回归
	}
	for _, c := range legal {
		if !CanTransition(c.from, c.to) {
			t.Errorf("CanTransition(%s→%s) = false, want true", c.from, c.to)
		}
	}
	illegal := []struct{ from, to ShotState }{
		{ShotUploaded, ShotAvailable}, // 必须先 SEGMENTED/TAGGED
		{ShotSegmented, ShotAvailable},
		{ShotCooling, ShotExpired},
		{ShotExpired, ShotAvailable}, // 终态
		{ShotRejected, ShotAvailable},
	}
	for _, c := range illegal {
		if CanTransition(c.from, c.to) {
			t.Errorf("CanTransition(%s→%s) = true, want false", c.from, c.to)
		}
	}
}

// TestIVSH1：safe_crop 不可裁且未声明 pillarbox ⇒ 不得进入 9:16 候选池。
func TestIVSH1(t *testing.T) {
	s := loadShot(t, "valid", "minimal.json")
	// ok=true：可入选。
	s.Affordance.SafeCrop9x16 = &SafeCrop{OK: true}
	if !s.EligibleForVertical() {
		t.Error("safe_crop.ok=true 应可入选")
	}
	// ok=false 未声明 pillarbox：不可入选。
	pillarbox := "PILLARBOX"
	s.Affordance.SafeCrop9x16 = &SafeCrop{OK: false}
	if s.EligibleForVertical() || s.PillarboxDeclared() {
		t.Error("ok=false 且未声明 method 时不可入选")
	}
	// ok=false 声明 PILLARBOX：豁免。
	s.Affordance.SafeCrop9x16 = &SafeCrop{OK: false, Method: &pillarbox}
	if !s.EligibleForVertical() || !s.PillarboxDeclared() {
		t.Error("声明 PILLARBOX 应豁免 IV-SH-1")
	}
	// pillarbox 声明样本必须可入选。
	pb := loadShot(t, "valid", "pillarbox_declared.json")
	if !pb.EligibleForVertical() {
		t.Error("pillarbox_declared 样本应可进入 9:16 候选池")
	}
}

// TestIVSH2：risk_flags 非空 ⇒ 不得 AVAILABLE。
func TestIVSH2(t *testing.T) {
	s := loadShot(t, "valid", "typical_tagged.json")
	s.State = ShotAvailable
	s.Compliance.RiskFlags = []string{"THIRD_PARTY_FACE"}
	if err := s.Validate(); err == nil ||
		!strings.Contains(err.Error(), "IV-SH-2") {
		t.Errorf("IV-SH-2 未触发: %v", err)
	}
	s.State = ShotQuarantine
	if err := s.Validate(); err != nil {
		t.Errorf("QUARANTINED 下应合法: %v", err)
	}
	if s.IsConsumable(time.Now()) {
		t.Error("risk_flags 非空的 shot 不可消费")
	}
}

// TestIVSH3：ttl 过期 ⇒ 出候选池。
func TestIVSH3(t *testing.T) {
	s := loadShot(t, "valid", "minimal.json")
	if s.State != ShotAvailable {
		s.State = ShotAvailable
	}
	now := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	old := "2020-01-01"
	future := "2099-01-01"
	s.Lifecycle.TTLAt = &old
	if s.IsConsumable(now) {
		t.Error("ttl_at < now 的 shot 必须出候选池")
	}
	s.Lifecycle.TTLAt = &future
	if !s.IsConsumable(now) {
		t.Error("ttl 未过期的 AVAILABLE shot 应可消费")
	}
}

// TestValidateIdentity：帧边界与 fps 约束。
func TestValidateIdentity(t *testing.T) {
	s := loadShot(t, "valid", "minimal.json")
	base := s.Identity.OutFrame - s.Identity.InFrame

	s.Identity.OutFrame = s.Identity.InFrame
	if err := s.Validate(); err == nil {
		t.Error("out_frame == in_frame 应报错")
	}
	s.Identity.OutFrame = s.Identity.InFrame + base
	s.Identity.FPS = 24
	if err := s.Validate(); err == nil {
		t.Error("fps=24 应报错")
	}
	s.Identity.FPS = 25
	s.Identity.DurationFrames = base + 1
	if err := s.Validate(); err == nil {
		t.Error("duration_frames 与 out-in 不一致应报错")
	}
}

// TestShotEvolutionSamples：G5 向后兼容——evolution/ 样本（钉死的上一 major
// 形态，文件名 v<major>_ 前缀）必须仍可被当前实体消费与校验。
func TestShotEvolutionSamples(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join(shotTestdataDir(), "evolution"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if s := loadShot(t, "evolution", e.Name()); s.Validate() != nil {
			t.Errorf("evolution 样本 %s 未通过 Validate: %v", e.Name(), s.Validate())
		}
	}
}
