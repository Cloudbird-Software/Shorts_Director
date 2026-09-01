// merchant.go 加载 mock 商家信息表（evals/merchants/<slug>/merchant.json，
// IFACE-4 版本化数据集）。INV-6：全部 fictional，字段为占位虚构信息。
package form1

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// MerchantInfo 是信息层五要素（gen_form 词表 I2V_AMBIENCE bindings
// info_layer 的确定性子集）。
type MerchantInfo struct {
	ShopName      string `json:"shop_name"`
	SignatureItem string `json:"signature_item"`
	Price         string `json:"price"`
	Address       string `json:"address"`
	Phone         string `json:"phone"`
}

// Merchant 是一个 mock 商家。
type Merchant struct {
	SchemaVersion  int          `json:"schema_version"`
	ID             string       `json:"id"`
	Vertical       string       `json:"vertical"`
	Fictional      bool         `json:"fictional"`
	Info           MerchantInfo `json:"info"`
	AIGCDisclosure string       `json:"aigc_disclosure"`
	// Path 是信息表文件路径（加载时回填；信息层断言的证据 URI）。
	Path string `json:"-"`
}

// LoadMerchant 从 merchant.json 读取并校验（INV-6 硬门禁：非 fictional 拒收）。
func LoadMerchant(path string) (*Merchant, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("form1: 读商家信息表失败: %w", err)
	}
	var m Merchant
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("form1: %s 不是合法 JSON: %w", path, err)
	}
	m.Path = path
	if m.SchemaVersion != 1 || m.ID == "" {
		return nil, fmt.Errorf("form1: %s 缺 schema_version/id", path)
	}
	if !m.Fictional {
		return nil, fmt.Errorf("form1: %s 非 fictional（INV-6 禁真实商家信息）", path)
	}
	if m.Info.ShopName == "" || m.Info.SignatureItem == "" || m.Info.Price == "" ||
		m.Info.Address == "" || m.Info.Phone == "" || m.AIGCDisclosure == "" {
		return nil, fmt.Errorf("form1: %s 信息表字段不齐（五要素 + AIGC 文案）", path)
	}
	return &m, nil
}

// LoadMerchantsDir 从目录批量加载：<dir>/<id>/merchant.json（按 id 索引）。
func LoadMerchantsDir(dir string) (map[string]*Merchant, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("form1: 读商家目录失败: %w", err)
	}
	out := map[string]*Merchant{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		m, err := LoadMerchant(filepath.Join(dir, e.Name(), "merchant.json"))
		if err != nil {
			return nil, err
		}
		if m.ID != e.Name() {
			return nil, fmt.Errorf("form1: 目录名 %s 与商家 id %q 不符", e.Name(), m.ID)
		}
		out[m.ID] = m
	}
	return out, nil
}
