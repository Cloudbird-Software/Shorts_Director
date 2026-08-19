package digest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Freeze Gate G3：共享向量集（testdata/digest/jcs_vectors.json）由 Go 与 TS
// 双侧消费，canonical 与 sha256 必须逐字节一致——本测试是 Go 侧锚点，
// TS 侧在 tests/digest.test.ts。改任何一侧实现而向量失配 ⇒ 双侧测试同时失败。
func TestSharedJCSVectors(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "digest", "jcs_vectors.json"))
	if err != nil {
		t.Fatal(err)
	}
	var suite struct {
		Comment string `json:"comment"`
		Vectors []struct {
			Name      string `json:"name"`
			Input     string `json:"input"`
			Canonical string `json:"canonical"`
			SHA256    string `json:"sha256"`
		} `json:"vectors"`
	}
	if err := json.Unmarshal(raw, &suite); err != nil {
		t.Fatal(err)
	}
	if len(suite.Vectors) < 5 {
		t.Fatalf("向量集少于 5 条：%d", len(suite.Vectors))
	}
	for _, v := range suite.Vectors {
		canon, err := CanonicalizeJSON([]byte(v.Input))
		if err != nil {
			t.Errorf("%s: %v", v.Name, err)
			continue
		}
		if string(canon) != v.Canonical {
			t.Errorf("%s: canonical 不一致\n got %s\nwant %s", v.Name, canon, v.Canonical)
		}
		sum := sha256.Sum256(canon)
		if got := hex.EncodeToString(sum[:]); got != v.SHA256 {
			t.Errorf("%s: sha256 不一致 got %s want %s", v.Name, got, v.SHA256)
		}
	}
}
