// data_test.go —— 卡 #120c：已提交形态4 数据集的完整性判定题
// （evals/suites/form4_digital_human.json × evals/merchants）。
//
// 数据契约（fake 后端全链可跑通的充分条件）：
//  1. 每条目有商家信息表 + form4 三要素脚本 + 人像照文件；
//  2. 口播文案 rune 数 × 0.24s（fake TTS 时长公式）≤ 断言包上限 6.0s；
//  3. 三要素（品牌/卖点/CTA）各自是口播文案的子串——fake 转写透传
//     文案，L1.transcribe.* 判定题必须能过（否则数据集自带恒失败条目）。
package form4_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Cloudbird-Software/Shorts_Director/internal/eval"
	"github.com/Cloudbird-Software/Shorts_Director/internal/form1"
	"github.com/Cloudbird-Software/Shorts_Director/internal/form4"
)

const (
	committedSuite    = "../../evals/suites/form4_digital_human.json"
	committedMerchant = "../../evals/merchants"
	// fakeTTSRate 是 operators/gen_tts fake 后端的时长公式系数（秒/rune）。
	fakeTTSRate = 0.24
	// durationUpperBound 与 internal/form4 assertionPack L0 钉死同值。
	durationUpperBound = 6.0
)

// TestForm4CommittedDatasetIntegrity：数据集自洽——fake 全链每条必出片。
func TestForm4CommittedDatasetIntegrity(t *testing.T) {
	suite, err := eval.LoadSuite(committedSuite)
	if err != nil {
		t.Fatal(err)
	}
	if suite.Op != "gen_lipsync" {
		t.Fatalf("形态4 套件 op 必须 gen_lipsync: %q", suite.Op)
	}
	merchants, err := form1.LoadMerchantsDir(committedMerchant)
	if err != nil {
		t.Fatal(err)
	}
	scripts, err := form4.LoadScriptsDir(committedMerchant)
	if err != nil {
		t.Fatal(err)
	}
	if len(suite.Entries) < 3 {
		t.Fatalf("形态4 套件应覆盖 ≥3 个 mock 商家（AC-7），得到 %d", len(suite.Entries))
	}
	for _, e := range suite.Entries {
		m, ok := merchants[e.ID]
		if !ok {
			t.Errorf("条目 %s 缺商家信息表", e.ID)
			continue
		}
		s, ok := scripts[e.ID]
		if !ok {
			t.Errorf("条目 %s 缺 form4.json 三要素脚本", e.ID)
			continue
		}
		// 1) 人像照落盘存在
		if _, err := os.Stat(filepath.Join(committedMerchant, e.ID,
			filepath.Base(e.ImagePath))); err != nil {
			t.Errorf("条目 %s 人像照缺失: %v", e.ID, err)
		}
		// 2) fake TTS 时长 ≤ 断言上限
		dur := float64(utf8.RuneCountInString(e.Prompt)) * fakeTTSRate
		if dur > durationUpperBound {
			t.Errorf("条目 %s 口播 %d rune → fake TTS %.2fs 超断言上限 %.1fs（文案须精简）",
				e.ID, utf8.RuneCountInString(e.Prompt), dur, durationUpperBound)
		}
		// 3) 三要素是口播文案子串（fake 转写透传 → 判定题可过）
		for field, want := range map[string]string{
			"brand": s.Brand, "selling_point": s.SellingPoint, "cta": s.CTA,
		} {
			if !strings.Contains(e.Prompt, want) {
				t.Errorf("条目 %s 三要素 %s=%q 不是口播文案子串： %q",
					e.ID, field, want, e.Prompt)
			}
		}
		// 4) AIGC 披露文案非空（信息层 overlay 与隐式元数据的数据前提）
		if strings.TrimSpace(m.AIGCDisclosure) == "" {
			t.Errorf("条目 %s 商家信息表缺 aigc_disclosure", e.ID)
		}
	}
}
