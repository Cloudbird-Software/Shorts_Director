package compat

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/Cloudbird-Software/Shorts_Director/internal/entity"
)

// golden fixture：假想的未来版本生产者数据（未知字段 + 未来枚举值）。
const forwardFixture = "testdata/forward/shot_from_future.json"

// TestDecodeTolerantIgnoresUnknownFields（G8 路径 1）：未来字段不阻解码，
// 已知部分完整落结构体，且 Validate 仍通过（未知字段不是错误）。
func TestDecodeTolerantIgnoresUnknownFields(t *testing.T) {
	raw, err := os.ReadFile(forwardFixture)
	if err != nil {
		t.Fatal(err)
	}
	var s entity.Shot
	if err := DecodeTolerant(raw, &s); err != nil {
		t.Fatalf("未知字段导致解码失败（违反 G8 路径 1）: %v", err)
	}
	if s.ID == "" || s.Identity.OutFrame == 0 {
		t.Fatal("已知字段未被解码")
	}
	if err := s.Validate(); err != nil {
		// fixture 的 state 是未来枚举，Validate 必须拒——写侧护栏不受消费边界影响；
		// 路径 1 的断言是"未知字段不产生错误"，枚举拒绝由路径 2 的降级处理（见下）。
		t.Logf("Validate 拒绝未来枚举（预期，写侧护栏）: %v", err)
	}
	// 摘除未来枚举后，仅含未知字段的数据必须整体可消费。
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	m["state"] = "SEGMENTED"
	cleaned, _ := json.Marshal(m)
	var s2 entity.Shot
	if err := DecodeTolerant(cleaned, &s2); err != nil {
		t.Fatalf("仅未知字段仍解码失败: %v", err)
	}
	if err := s2.Validate(); err != nil {
		t.Errorf("仅未知字段的数据未通过 Validate（G8 路径 1 破坏）: %v", err)
	}
}

// TestScanUnknownEnums（G8 路径 2 前半）：扫描出全部未知枚举位置与原值。
func TestScanUnknownEnums(t *testing.T) {
	raw, err := os.ReadFile(forwardFixture)
	if err != nil {
		t.Fatal(err)
	}
	legalStates := func(v string) bool {
		for _, s := range []entity.ShotState{
			entity.ShotUploaded, entity.ShotSegmented, entity.ShotTagged,
			entity.ShotRejected, entity.ShotAvailable, entity.ShotCooling,
			entity.ShotExpired, entity.ShotQuarantine,
		} {
			if v == string(s) {
				return true
			}
		}
		return false
	}
	got, err := ScanUnknownEnums(raw, map[string]func(string) bool{
		"/state": legalStates,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []UnknownEnum{{Pointer: "/state", Raw: "AI_REMASTERED"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("未知枚举扫描结果 %+v，期望 %+v", got, want)
	}
}

// TestDegradeEnum（G8 路径 2 后半）：已知原样、未知落 UNKNOWN+Raw，
// 降级后的状态 fail-safe——未知 ShotState 不可消费。
func TestDegradeEnum(t *testing.T) {
	legal := func(v string) bool { return v == "AVAILABLE" || v == "SEGMENTED" }

	if e := DegradeEnum("AVAILABLE", legal); e.Value != "AVAILABLE" || e.Unknown() {
		t.Errorf("已知值必须原样: %+v", e)
	}
	e := DegradeEnum("AI_REMASTERED", legal)
	if e.Value != UnknownEnumValue || e.Raw != "AI_REMASTERED" || !e.Unknown() {
		t.Errorf("未知值必须落 UNKNOWN+Raw: %+v", e)
	}

	// fail-safe 语义示例：降级状态导致 Shot 不再可消费（而非误入候选池）。
	s := entity.Shot{
		ID:       "018f6c01-aaaa-7aaa-8aaa-000000000001",
		AssetID:  "018f6b2e-9c4a-7b3e-a1d2-3f4e5d6c7b8a",
		TenantID: "018f6b10-0000-7000-8000-000000000001",
		Identity: entity.ShotIdentity{InFrame: 0, OutFrame: 100, FPS: 25},
	}
	s.State = entity.ShotState(DegradeEnum("AI_REMASTERED", legal).Value)
	if s.IsConsumable(time.Now()) {
		t.Error("降级为 UNKNOWN 的状态不得进入候选池（fail-safe 破坏）")
	}
}

// TestScanUnknownEnumsJSONError：非法 JSON 报错而非静默通过。
func TestScanUnknownEnumsJSONError(t *testing.T) {
	if _, err := ScanUnknownEnums([]byte("{not json"), nil); err == nil {
		t.Error("非法 JSON 必须报错")
	}
}
