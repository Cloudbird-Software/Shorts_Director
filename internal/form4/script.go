// script.go 加载形态4 口播脚本（evals/merchants/<slug>/form4.json）：
// 三要素（品牌名/卖点/行动号召）的显式期望锚——转写断言的判定是
// 「转写文本包含三要素」这一判定题，期望值必须钉死在数据里，
// 不从文案倒推（防实现向验收口径漂移）。
package form4

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Script 是一个 mock 商家的形态4 口播脚本（三要素期望锚）。
type Script struct {
	SchemaVersion int    `json:"schema_version"`
	MerchantID    string `json:"merchant_id"`
	Brand         string `json:"brand"`         // 三要素1：品牌名（转写包含 + 信息层 overlay）
	SellingPoint  string `json:"selling_point"` // 三要素2：卖点
	CTA           string `json:"cta"`           // 三要素3：行动号召
	// Path 是脚本文件路径（加载时回填；信息层断言的证据 URI）。
	Path string `json:"-"`
}

// LoadScript 从 form4.json 读取并校验。
func LoadScript(path string) (*Script, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("form4: 读口播脚本失败: %w", err)
	}
	var s Script
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("form4: %s 不是合法 JSON: %w", path, err)
	}
	s.Path = path
	if s.SchemaVersion != 1 || s.MerchantID == "" {
		return nil, fmt.Errorf("form4: %s 缺 schema_version/merchant_id", path)
	}
	if s.Brand == "" || s.SellingPoint == "" || s.CTA == "" {
		return nil, fmt.Errorf("form4: %s 三要素不齐（brand/selling_point/cta）", path)
	}
	return &s, nil
}

// LoadScriptsDir 从商家目录批量加载：<dir>/<id>/form4.json（按 id 索引）。
// 无 form4.json 的商家跳过（形态4 是子集——只有配了口播脚本的商家进管线）。
func LoadScriptsDir(dir string) (map[string]*Script, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("form4: 读商家目录失败: %w", err)
	}
	out := map[string]*Script{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(dir, e.Name(), "form4.json")
		if _, err := os.Stat(path); err != nil {
			continue
		}
		s, err := LoadScript(path)
		if err != nil {
			return nil, err
		}
		if s.MerchantID != e.Name() {
			return nil, fmt.Errorf("form4: 目录名 %s 与脚本 merchant_id %q 不符", e.Name(), s.MerchantID)
		}
		out[s.MerchantID] = s
	}
	return out, nil
}
