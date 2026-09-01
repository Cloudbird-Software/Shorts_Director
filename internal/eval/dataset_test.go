// dataset_test.go —— 卡 #116：evals/ 数据集与形态套件的结构守护
// （IFACE-4 版本化目录/禁运行时抓网；INV-6 无真实 PII）。
package eval_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Cloudbird-Software/Shorts_Director/internal/eval"
	"github.com/Cloudbird-Software/Shorts_Director/internal/operator"
)

const evalsRoot = "../../evals"

type merchantDoc struct {
	SchemaVersion int    `json:"schema_version"`
	ID            string `json:"id"`
	Vertical      string `json:"vertical"`
	Fictional     bool   `json:"fictional"`
	Info          struct {
		ShopName      string `json:"shop_name"`
		SignatureItem string `json:"signature_item"`
		Price         string `json:"price"`
		Address       string `json:"address"`
		Phone         string `json:"phone"`
	} `json:"info"`
	AIGCDisclosure string `json:"aigc_disclosure"`
	SeedImages     []struct {
		Role   string `json:"role"`
		Path   string `json:"path"`
		Source struct {
			Type      string `json:"type"`
			Generator string `json:"generator"`
		} `json:"source"`
		LicensePlaceholder string `json:"license_placeholder"`
	} `json:"seed_images"`
}

// TestEvalsMerchants：≥3 个 mock 商家（餐饮×2/美业×1），信息表字段齐全，
// fictional=true（INV-6），种子图实际存在且为 PNG，来源全为合成。
func TestEvalsMerchants(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join(evalsRoot, "merchants"))
	if err != nil {
		t.Fatal(err)
	}
	verticals := map[string]int{}
	total := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		total++
		raw, err := os.ReadFile(filepath.Join(evalsRoot, "merchants", e.Name(), "merchant.json"))
		if err != nil {
			t.Fatalf("%s: %v", e.Name(), err)
		}
		var m merchantDoc
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("%s: %v", e.Name(), err)
		}
		if m.SchemaVersion != 1 || m.ID == "" || !m.Fictional {
			t.Fatalf("%s: schema/fictional 声明缺失（INV-6）", e.Name())
		}
		if m.Info.ShopName == "" || m.Info.SignatureItem == "" || m.Info.Price == "" ||
			m.Info.Address == "" || m.Info.Phone == "" || m.AIGCDisclosure == "" {
			t.Fatalf("%s: 信息表字段不齐（店名/招牌项/价格/地址/电话/AIGC）", e.Name())
		}
		if len(m.SeedImages) == 0 {
			t.Fatalf("%s: 无种子图", e.Name())
		}
		for _, img := range m.SeedImages {
			raw, err := os.ReadFile(filepath.Join("../..", img.Path))
			if err != nil || len(raw) < 8 || string(raw[:4]) != "\x89PNG" {
				t.Fatalf("%s: 种子图 %s 缺失或非 PNG", e.Name(), img.Path)
			}
			if img.Source.Type != "synthetic" || img.Source.Generator == "" || img.LicensePlaceholder == "" {
				t.Fatalf("%s: 种子图 %s 来源/授权占位记录缺失（IFACE-4）", e.Name(), img.Path)
			}
		}
		verticals[m.Vertical]++
	}
	if total < 3 {
		t.Fatalf("mock 商家 %d < 3", total)
	}
	if verticals["food"] < 2 || verticals["beauty"] < 1 {
		t.Fatalf("垂类分布不符（餐饮×2/美业×1 起）: %v", verticals)
	}
}

// TestEvalsSuites：全部套件可经 eval.LoadSuite 受控校验；形态1/形态4 套件
// seed 集 K=5 且条目引用真实存在的商家种子图。
func TestEvalsSuites(t *testing.T) {
	suites := map[string]*eval.Suite{}
	for _, name := range []string{"form1_ambience.json", "form4_digital_human.json", "form1_smoke_fake.json"} {
		s, err := eval.LoadSuite(filepath.Join(evalsRoot, "suites", name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		suites[name] = s
	}
	if s := suites["form1_ambience.json"]; len(s.Seeds) != 5 || len(s.Entries) != 3 {
		t.Fatalf("形态1 套件须 3 商家 × K=5: seeds=%d entries=%d", len(s.Seeds), len(s.Entries))
	}
	if s := suites["form4_digital_human.json"]; len(s.Seeds) != 5 || len(s.Entries) != 1 {
		t.Fatalf("形态4 套件须 1 商家 × K=5: seeds=%d entries=%d", len(s.Seeds), len(s.Entries))
	}
	if s := suites["form1_ambience.json"]; len(s.AssertionPack) < 3 {
		t.Fatalf("形态1 断言包过薄: %d", len(s.AssertionPack))
	}
	for name, s := range suites {
		for _, e := range s.Entries {
			if _, err := os.Stat(filepath.Join("../..", e.ImagePath)); err != nil {
				t.Fatalf("%s: 条目 %s 种子图不存在: %s", name, e.ID, e.ImagePath)
			}
		}
	}
}

// TestEvalsSmokeSuiteRuns：冒烟套件经 FakeRunner 真实执行——数据集与
// golden fixtures、评估仪器三方咬合（CI 内无 Python/GPU 也能跑通 E2 骨架）。
func TestEvalsSmokeSuiteRuns(t *testing.T) {
	s, err := eval.LoadSuite(filepath.Join(evalsRoot, "suites", "form1_smoke_fake.json"))
	if err != nil {
		t.Fatal(err)
	}
	art, err := eval.Run(context.Background(), eval.RunOptions{
		Suite: s, Gen: &operator.FakeRunner{Dir: goldenRoot}, Engine: engineWith(),
		ProfileRef: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
		RunnerMode: "fake", WorkdirRoot: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if art.Yield.YieldRatio != 1 || art.Items[0].Status != eval.ItemOK {
		t.Fatalf("冒烟套件应全可用: %+v", art.Yield)
	}
}
