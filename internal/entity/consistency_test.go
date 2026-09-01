// consistency_test.go —— Freeze Gate G2：同一样本集、双语言判定一致。
//
// TS 侧（ajv，tests/testdata.test.ts）已断言：valid 全过 / invalid 全拒。
// 本测试是 Go 侧锚点，消费同一批样本：
//   - valid 样本：Go Validate 必须通过（与 ajv 一致）
//   - invalid 样本：Go Validate 必须拒绝——**除非**该样本的失败是纯结构性的
//     （additionalProperties/长度/pattern/格式等 schema 领域），逐条登记在
//     testdata/g2_go_pass_invalid.json（含登记理由）。登记表任何变化都是
//     评审焦点：双侧领域差异必须显式化，禁止静默漂移。
package entity_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Cloudbird-Software/Shorts_Director/internal/entity"
	"github.com/Cloudbird-Software/Shorts_Director/internal/videoplan"
)

// goEntities 是有 Go 实体实现的实体 → 校验入口（与 schema/testdata 共样本）。
var goEntities = []struct {
	name  string
	check func(raw []byte) error
}{
	{"shot", func(raw []byte) error {
		var s entity.Shot
		if err := json.Unmarshal(raw, &s); err != nil {
			return err
		}
		return s.Validate()
	}},
	{"video_plan", func(raw []byte) error {
		var p videoplan.Plan
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		return p.Validate()
	}},
}

// TestG2ValidSamplesBothLanguagesAccept：valid 样本 Go Validate 必须通过
// （TS ajv 侧同批样本由 G1 harness 断言通过）。
func TestG2ValidSamplesBothLanguagesAccept(t *testing.T) {
	for _, e := range goEntities {
		files, err := filepath.Glob(filepath.Join("..", "..", "schema", "testdata", e.name, "valid", "*.json"))
		if err != nil || len(files) < 5 {
			t.Fatalf("%s valid 样本异常: %v (%d)", e.name, err, len(files))
		}
		for _, f := range files {
			raw, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			if err := e.check(raw); err != nil {
				t.Errorf("%s/valid/%s: Go Validate 未通过（与 ajv 判定不一致）: %v",
					e.name, filepath.Base(f), err)
			}
		}
	}
}

// TestG2InvalidSamplesVerdictsConsistent：invalid 样本 Go 必须拒绝，
// 纯结构性失败按登记表豁免（登记表含理由；TS 侧另有测试防登记表腐烂）。
func TestG2InvalidSamplesVerdictsConsistent(t *testing.T) {
	allow := loadAllowlist(t)
	for _, e := range goEntities {
		files, err := filepath.Glob(filepath.Join("..", "..", "schema", "testdata", e.name, "invalid", "*.json"))
		if err != nil || len(files) < 15 {
			t.Fatalf("%s invalid 样本异常: %v (%d)", e.name, err, len(files))
		}
		for _, f := range files {
			base := filepath.Base(f)
			raw, _ := os.ReadFile(f)
			err := e.check(raw)
			exempted, known := allow[e.name][base]
			if err == nil && !known {
				t.Errorf("%s/invalid/%s: Go Validate 放行但未登记结构性豁免——请补登记或修校验",
					e.name, base)
			}
			if err != nil && known {
				t.Errorf("%s/invalid/%s: 已登记豁免（%s）但 Go 实际拒绝——登记过期，请删除",
					e.name, base, exempted)
			}
		}
	}
}

// loadAllowlist 读 testdata/g2_go_pass_invalid.json：
// 实体 → {样本文件名 → 登记理由（纯结构性失败的说明）}。_comment 说明键跳过。
func loadAllowlist(t *testing.T) map[string]map[string]string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "g2_go_pass_invalid.json"))
	if err != nil {
		t.Fatalf("豁免登记表不可读: %v", err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatal(err)
	}
	m := map[string]map[string]string{}
	for k, v := range top {
		if k == "_comment" {
			continue
		}
		var inner map[string]string
		if err := json.Unmarshal(v, &inner); err != nil {
			t.Fatalf("登记表 %s 段格式错误: %v", k, err)
		}
		m[k] = inner
	}
	return m
}
