// merchant_test.go —— 卡 #118：mock 商家信息表加载与 INV-6 硬门禁。
package form1_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Cloudbird-Software/Shorts_Director/internal/form1"
)

func writeMerchantFile(t *testing.T, dir, name string, raw map[string]any) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
		t.Fatal(err)
	}
	bs, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, bs, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func validMerchant() map[string]any {
	return map[string]any{
		"schema_version": 1, "id": "m1", "vertical": "food", "fictional": true,
		"info": map[string]any{
			"shop_name": "S", "signature_item": "I", "price": "P",
			"address": "A", "phone": "T",
		},
		"aigc_disclosure": "AI generated demo",
	}
}

func TestLoadMerchantRejects(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]func(m map[string]any){
		"非 fictional":    func(m map[string]any) { m["fictional"] = false },
		"缺 fictional 字段": func(m map[string]any) { delete(m, "fictional") },
		"schema 版本错":     func(m map[string]any) { m["schema_version"] = 2 },
		"缺 id":           func(m map[string]any) { m["id"] = "" },
		"缺店名":            func(m map[string]any) { m["info"].(map[string]any)["shop_name"] = "" },
		"缺 AIGC 文案":      func(m map[string]any) { m["aigc_disclosure"] = "" },
	}
	for name, mutate := range cases {
		m := validMerchant()
		mutate(m)
		if _, err := form1.LoadMerchant(writeMerchantFile(t, dir, name+".json", m)); err == nil {
			t.Errorf("%s: 应被拒收", name)
		}
	}
	if _, err := form1.LoadMerchant(filepath.Join(dir, "nope.json")); err == nil {
		t.Error("文件不存在应报错")
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := form1.LoadMerchant(filepath.Join(dir, "bad.json")); err == nil {
		t.Error("非法 JSON 应报错")
	}
}

func TestLoadMerchantOK(t *testing.T) {
	dir := t.TempDir()
	path := writeMerchantFile(t, dir, "m1.json", validMerchant())
	m, err := form1.LoadMerchant(path)
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != "m1" || !m.Fictional || m.Path != path || m.Info.ShopName != "S" {
		t.Fatalf("加载结果异常: %+v", m)
	}
}

func TestLoadMerchantsDir(t *testing.T) {
	dir := t.TempDir()
	writeMerchantFile(t, dir, "a/merchant.json", func() map[string]any {
		m := validMerchant()
		m["id"] = "a"
		return m
	}())
	// 目录名与 id 不符 → 拒收
	writeMerchantFile(t, dir, "b/merchant.json", func() map[string]any {
		m := validMerchant()
		m["id"] = "c"
		return m
	}())
	if _, err := form1.LoadMerchantsDir(dir); err == nil {
		t.Fatal("目录名与 id 不符应被拒收")
	}
	// 空目录 → 空集（套件无条目时管线另有校验）
	empty := filepath.Join(dir, "empty")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	ms, err := form1.LoadMerchantsDir(empty)
	if err != nil || len(ms) != 0 {
		t.Fatalf("空目录应得空集: %v %+v", err, ms)
	}
}
