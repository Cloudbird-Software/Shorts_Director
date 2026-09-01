// consistency_test.go —— Freeze Gate G2：同一样本集、双语言判定一致。
//
// TS 侧（ajv，tests/testdata.test.ts）已断言：valid 全过 / invalid 全拒。
// 本测试是 Go 侧锚点（IR-0007 退役-2/3 自 internal/entity 迁入，现仅覆盖
// video_plan——唯一有 Go 实体实现的存留实体），消费同一批样本：
//   - valid 样本：Go Validate 必须通过（与 ajv 一致）
//   - invalid 样本：Go Validate 必须拒绝——**除非**该样本的失败是纯结构性的
//     （additionalProperties/长度/pattern/格式等 schema 领域），逐条登记在
//     testdata/g2_go_pass_invalid.json（含登记理由）。登记表任何变化都是
//     评审焦点：双侧领域差异必须显式化，禁止静默漂移。
package videoplan_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Cloudbird-Software/Shorts_Director/internal/videoplan"
)

// checkVideoPlan 是 video_plan 的校验入口（与 schema/testdata 共样本）。
func checkVideoPlan(raw []byte) error {
	var p videoplan.Plan
	if err := json.Unmarshal(raw, &p); err != nil {
		return err
	}
	return p.Validate()
}

// TestG2ValidSamplesBothLanguagesAccept：valid 样本 Go Validate 必须通过
// （TS ajv 侧同批样本由 G1 harness 断言通过）。
func TestG2ValidSamplesBothLanguagesAccept(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "schema", "testdata", "video_plan", "valid", "*.json"))
	if err != nil || len(files) < 5 {
		t.Fatalf("video_plan valid 样本异常: %v (%d)", err, len(files))
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if err := checkVideoPlan(raw); err != nil {
			t.Errorf("video_plan/valid/%s: Go Validate 未通过（与 ajv 判定不一致）: %v",
				filepath.Base(f), err)
		}
	}
}

// TestG2InvalidSamplesVerdictsConsistent：invalid 样本 Go 必须拒绝，
// 纯结构性失败按登记表豁免（登记表含理由；TS 侧另有测试防登记表腐烂）。
func TestG2InvalidSamplesVerdictsConsistent(t *testing.T) {
	allow := loadAllowlist(t)
	files, err := filepath.Glob(filepath.Join("..", "..", "schema", "testdata", "video_plan", "invalid", "*.json"))
	if err != nil || len(files) < 15 {
		t.Fatalf("video_plan invalid 样本异常: %v (%d)", err, len(files))
	}
	for _, f := range files {
		base := filepath.Base(f)
		raw, _ := os.ReadFile(f)
		err := checkVideoPlan(raw)
		exempted, known := allow[base]
		if err == nil && !known {
			t.Errorf("video_plan/invalid/%s: Go Validate 放行但未登记结构性豁免——请补登记或修校验",
				base)
		}
		if err != nil && known {
			t.Errorf("video_plan/invalid/%s: 已登记豁免（%s）但 Go 实际拒绝——登记过期，请删除",
				base, exempted)
		}
	}
}

// loadAllowlist 读 testdata/g2_go_pass_invalid.json 的 video_plan 段：
// {样本文件名 → 登记理由（纯结构性失败的说明）}。_comment 说明键跳过。
func loadAllowlist(t *testing.T) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "g2_go_pass_invalid.json"))
	if err != nil {
		t.Fatalf("豁免登记表不可读: %v", err)
	}
	var top struct {
		Comment   json.RawMessage   `json:"_comment"`
		VideoPlan map[string]string `json:"video_plan"`
	}
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatal(err)
	}
	if top.VideoPlan == nil {
		t.Fatal("登记表缺少 video_plan 段")
	}
	return top.VideoPlan
}
