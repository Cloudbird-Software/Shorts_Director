package compiler

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Cloudbird-Software/Shorts_Director/internal/videoplan"
)

// ────────────────────────────────────────────────────────────────────
// M3 变形不变式（Freeze Gate G10，issue #44）：编译确定性
//
// 输入变换：plan JSON 中所有对象的键序深度重排（含 caption payload 的
// map 字段）。输出不变式：编译产物序列化逐字节一致——compiler 对
// map 遍历序不敏感（docs/ARCHITECTURE.md 渲染确定性三禁令的前置）。
// 复用 valid 样本（schema/testdata 单一事实源），确定性 LCG 驱动键序。
// ────────────────────────────────────────────────────────────────────

type klcg struct{ state uint64 }

func (r *klcg) next() uint64 {
	r.state = r.state*6364136223846793005 + 1442695040888963407
	return r.state >> 33
}

func (r *klcg) intn(n int) int { return int(r.next() % uint64(n)) }

// reshuffleKeyOrder 深度乱序序列化任意 JSON 值的键序。
func reshuffleKeyOrder(r *klcg, v any, buf *bytes.Buffer) error {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		for i := len(keys) - 1; i > 0; i-- {
			j := r.intn(i + 1)
			keys[i], keys[j] = keys[j], keys[i]
		}
		buf.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			kb, err := json.Marshal(k)
			if err != nil {
				return err
			}
			buf.Write(kb)
			buf.WriteByte(':')
			if err := reshuffleKeyOrder(r, t[k], buf); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
	case []any:
		buf.WriteByte('[')
		for i, e := range t {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := reshuffleKeyOrder(r, e, buf); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
	default:
		eb, err := json.Marshal(t)
		if err != nil {
			return err
		}
		buf.Write(eb)
	}
	return nil
}

const m3Runs = 200 // 单例样本键序排列空间有限，200 轮充分覆盖

// TestMetamorphicKeyOrderCompileStable：键序重排 → 编译产物逐字节一致。
func TestMetamorphicKeyOrderCompileStable(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "schema", "testdata", "video_plan", "valid", "with_vo_and_speed.json"))
	if err != nil {
		t.Fatalf("读样本失败: %v", err)
	}
	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		t.Fatalf("解析样本: %v", err)
	}

	compileFrom := func(tree any) []byte {
		var buf bytes.Buffer
		if err := reshuffleKeyOrder(&klcg{state: 1}, tree, &buf); err != nil {
			t.Fatalf("重排序列化: %v", err)
		}
		var p videoplan.Plan
		if err := json.Unmarshal(buf.Bytes(), &p); err != nil {
			t.Fatalf("解析重排 plan: %v", err)
		}
		req, err := Compile(p, fixtureIndex(), fixtureFonts(), fixtureOutput(), fixtureModes(), fixtureExpect())
		if err != nil {
			t.Fatalf("Compile: %v", err)
		}
		out, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		return out
	}

	baseline := compileFrom(tree)
	r := &klcg{state: 20260820}
	for i := 0; i < m3Runs; i++ {
		var buf bytes.Buffer
		if err := reshuffleKeyOrder(r, tree, &buf); err != nil {
			t.Fatalf("重排序列化（第 %d 例）: %v", i, err)
		}
		var p videoplan.Plan
		if err := json.Unmarshal(buf.Bytes(), &p); err != nil {
			t.Fatalf("解析重排 plan（第 %d 例）: %v", i, err)
		}
		req, err := Compile(p, fixtureIndex(), fixtureFonts(), fixtureOutput(), fixtureModes(), fixtureExpect())
		if err != nil {
			t.Fatalf("Compile（第 %d 例）: %v", i, err)
		}
		out, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("Marshal（第 %d 例）: %v", i, err)
		}
		if !bytes.Equal(baseline, out) {
			t.Fatalf("M3 违例（第 %d 例）：键序重排改变编译产物\n baseline=%s\n shuffled=%s", i, baseline, out)
		}
	}
}
