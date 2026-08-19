package brandkernel

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func testdataDir() string {
	return filepath.Join("..", "..", "schema", "testdata", "brand_kernel")
}

func loadKernel(t *testing.T, sub, name string) BrandKernel {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(testdataDir(), sub, name))
	if err != nil {
		t.Fatalf("读样本失败: %v", err)
	}
	var k BrandKernel
	if err := json.Unmarshal(raw, &k); err != nil {
		t.Fatalf("解析 %s/%s: %v", sub, name, err)
	}
	return k
}

// TestValidSamplesPassValidate：G1 valid 样本（≥5）必须全部通过 IV-BK 校验。
func TestValidSamplesPassValidate(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join(testdataDir(), "valid"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) < 5 {
		t.Fatalf("valid 样本少于 5 个：%d", len(entries))
	}
	for _, e := range entries {
		if k := loadKernel(t, "valid", e.Name()); k.Validate() != nil {
			t.Errorf("valid 样本 %s 未通过 Validate: %v", e.Name(), k.Validate())
		}
	}
}

// TestRoundTripNoDrift：schema 字段 ↔ Go 结构体防漂移——样本里的未知字段
// （Go 侧漏映射）必须报错；重序列化再解析必须等值。
func TestRoundTripNoDrift(t *testing.T) {
	entries, _ := os.ReadDir(filepath.Join(testdataDir(), "valid"))
	for _, e := range entries {
		raw, err := os.ReadFile(filepath.Join(testdataDir(), "valid", e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		var k BrandKernel
		if err := dec.Decode(&k); err != nil {
			t.Errorf("%s 存在 Go 结构体未映射的字段（漂移）: %v", e.Name(), err)
			continue
		}
		out, err := json.Marshal(k)
		if err != nil {
			t.Fatalf("序列化 %s: %v", e.Name(), err)
		}
		var k2 BrandKernel
		if err := json.Unmarshal(out, &k2); err != nil {
			t.Fatalf("重解析 %s: %v", e.Name(), err)
		}
		if !reflect.DeepEqual(k, k2) {
			t.Errorf("%s round-trip 后不等值", e.Name())
		}
	}
}

func baseKernel(t *testing.T) BrandKernel {
	t.Helper()
	return loadKernel(t, "valid", "minimal.json")
}

// TestIVBK1：pillars <3 或某 pillar proof_types <2 必须被拒。
func TestIVBK1(t *testing.T) {
	k := baseKernel(t)
	k.Pillars = k.Pillars[:2]
	if err := k.Validate(); err == nil || !strings.Contains(err.Error(), "IV-BK-1") {
		t.Errorf("pillars=2 未触发 IV-BK-1: %v", err)
	}

	k = baseKernel(t)
	k.Pillars[0].ProofTypes = k.Pillars[0].ProofTypes[:1]
	if err := k.Validate(); err == nil || !strings.Contains(err.Error(), "IV-BK-1") {
		t.Errorf("pillar proof_types=1 未触发 IV-BK-1: %v", err)
	}

	k = baseKernel(t)
	k.Pillars[0].ProofTypes = nil
	if err := k.Validate(); err == nil || !strings.Contains(err.Error(), "IV-BK-1") {
		t.Errorf("pillar proof_types=nil 未触发 IV-BK-1: %v", err)
	}
}

// TestIVBK2：score<0.75 时访谈停止条件未满足，不得进入 L3 匹配。
func TestIVBK2(t *testing.T) {
	k := baseKernel(t) // minimal score=0.82
	if err := k.ReadyForL3Matching(); err != nil {
		t.Errorf("score=0.82 应可进入 L3: %v", err)
	}

	k.Completeness.Score = 0.7499
	if err := k.ReadyForL3Matching(); err == nil || !strings.Contains(err.Error(), "IV-BK-2") {
		t.Errorf("score=0.7499 未触发 IV-BK-2: %v", err)
	}
	k.Completeness.Score = 0.75
	if err := k.ReadyForL3Matching(); err != nil {
		t.Errorf("score=0.75（边界含）应可进入 L3: %v", err)
	}

	// 停止条件是覆盖度而非轮次：轮次耗尽但未达标仍拒绝。
	k.Completeness.Score = 0.6
	k.Completeness.InterviewTurns = 18
	if err := k.ReadyForL3Matching(); err == nil {
		t.Error("interview_turns=18 但 score=0.6 不得放行（覆盖度才是停止条件）")
	}
}

// TestIVBK3：category 必须来自受控枚举且决定 compliance_profile_id。
func TestIVBK3(t *testing.T) {
	k := baseKernel(t)
	k.Category = "随便写的类目" // 模拟 LLM 自由生成
	if err := k.Validate(); err == nil || !strings.Contains(err.Error(), "IV-BK-3") {
		t.Errorf("自由生成 category 未触发 IV-BK-3: %v", err)
	}

	k = baseKernel(t)
	k.ComplianceProfileID = ""
	if err := k.Validate(); err == nil || !strings.Contains(err.Error(), "IV-BK-3") {
		t.Errorf("compliance_profile_id 缺失未触发 IV-BK-3: %v", err)
	}
}
