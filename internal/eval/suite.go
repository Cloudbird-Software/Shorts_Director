// Package eval 实现镜头制式评估编排（IR-0007 AC-4/AC-5，实验 E2 的仪器）：
// 套件定义（形态×模型×seed 集×断言包×预算上限）→ 逐条生成（C2 Runner）
// → 逐条断言（复用 qc 引擎）→ 聚合出片率 → 内容寻址 run artifact。
//
// 出片率口径全系统唯一（IFACE-5）：「可用」= 通过该制式断言包全部断言；
// 聚合口径 = K 次抽卡中至少 1 条可用的条目（entry）比例。
package eval

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/Cloudbird-Software/Shorts_Director/internal/qc"
	"github.com/Cloudbird-Software/Shorts_Director/internal/vocab"
)

// SchemaVersion 是套件/run artifact 的结构版本。
const SchemaVersion = 1

// genOps 是生成算子 op 受控枚举（IFACE-1 首批四类）；
// 新增 op 走本清单变更，禁止散落命名。
var genOps = map[string]bool{
	"gen_i2v": true, "gen_tts": true, "gen_lipsync": true, "vlm_boolean": true,
}

// Budget 是套件执行的预算上限（BUDGET-3：超限中止 + 部分结果标注落盘）。
type Budget struct {
	WallSeconds float64 `json:"wall_seconds"` // >0
	GpuSeconds  float64 `json:"gpu_seconds"`  // ≥0；fake 后端为 0
}

// Entry 是一次"K 抽"的单元：一个商家场景（种子图+prompt）× 全部 seed。
type Entry struct {
	ID          string  `json:"id"`
	ImagePath   string  `json:"image_path"`
	Prompt      string  `json:"prompt"`
	DurationSec float64 `json:"duration_sec"`
}

// Suite 是制式评估套件定义（硬件无关：不含机器/路径之外的执行环境）。
type Suite struct {
	SchemaVersion int            `json:"schema_version"`
	SuiteID       string         `json:"suite_id"`
	GenForm       string         `json:"gen_form"` // vocab gen_form
	Op            string         `json:"op"`       // IFACE-1 受控算子枚举
	Model         string         `json:"model"`    // 算子后端注册表键
	Seeds         []int64        `json:"seeds"`
	FPS           int            `json:"fps"`
	Params        map[string]any `json:"params,omitempty"`
	Entries       []Entry        `json:"entries"`
	AssertionPack []qc.Assertion `json:"assertion_pack"`
	Budget        Budget         `json:"budget"`
}

// LoadSuite 从 JSON 文件读取并校验套件定义。
func LoadSuite(path string) (*Suite, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("eval: 读套件失败: %w", err)
	}
	var s Suite
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("eval: 套件 %s 不是合法 JSON: %w", path, err)
	}
	if err := s.Validate(); err != nil {
		return nil, fmt.Errorf("eval: 套件 %s 非法: %w", path, err)
	}
	return &s, nil
}

// Validate 校验套件定义的受控域。
func (s *Suite) Validate() error {
	if s.SchemaVersion != SchemaVersion {
		return fmt.Errorf("schema_version 必须 %d", SchemaVersion)
	}
	if s.SuiteID == "" {
		return fmt.Errorf("suite_id 必填")
	}
	if !vocab.IsVocabID("gen_form", s.GenForm) {
		return fmt.Errorf("gen_form %q 不在词表 gen_form", s.GenForm)
	}
	if !genOps[s.Op] {
		return fmt.Errorf("op %q 不在 IFACE-1 受控算子枚举", s.Op)
	}
	if s.Model == "" {
		return fmt.Errorf("model 必填（算子后端注册表键）")
	}
	if len(s.Seeds) == 0 {
		return fmt.Errorf("seeds 必填非空（K 次抽卡的 K）")
	}
	if s.FPS <= 0 {
		return fmt.Errorf("fps 必须为正整数")
	}
	if len(s.Entries) == 0 {
		return fmt.Errorf("entries 必填非空")
	}
	seen := map[string]bool{}
	for i, e := range s.Entries {
		if e.ID == "" || seen[e.ID] {
			return fmt.Errorf("entries[%d].id 必填且不得重复", i)
		}
		seen[e.ID] = true
		if e.ImagePath == "" || e.Prompt == "" || e.DurationSec <= 0 {
			return fmt.Errorf("entries[%d] 需要 image_path/prompt/duration_sec>0", i)
		}
	}
	for i := range s.AssertionPack {
		if err := s.AssertionPack[i].Validate(); err != nil {
			return fmt.Errorf("assertion_pack[%d]: %w", i, err)
		}
	}
	if s.Budget.WallSeconds <= 0 {
		return fmt.Errorf("budget.wall_seconds 必须 >0")
	}
	if s.Budget.GpuSeconds < 0 {
		return fmt.Errorf("budget.gpu_seconds 不得为负")
	}
	return nil
}
