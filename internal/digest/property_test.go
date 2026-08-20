package digest

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// ────────────────────────────────────────────────────────────────────
// G9 属性测试 + M1 变形不变式（Freeze Gate，issue #44）
//
// 零第三方依赖：确定性 LCG 自造随机 JSON 树（含 CJK/emoji 键、浮点数、
// 嵌套数组），驱动 ≥1000 次属性检查。pgregory.net/rapid 依赖报批期间，
// 属性框架以最小自研生成器替代（AGENTS.md 硬规则 3：标准库优先）。
// ────────────────────────────────────────────────────────────────────

// lcg 是可复现的伪随机源（确定性：同种子同序列——失败可重放）。
type lcg struct{ state uint64 }

func (r *lcg) next() uint64 {
	r.state = r.state*6364136223846793005 + 1442695040888963407
	return r.state >> 33
}

func (r *lcg) intn(n int) int { return int(r.next() % uint64(n)) }

// randomValue 递归生成随机 JSON 值（深度 ≤3、宽度 ≤4、键含 Unicode）。
func (r *lcg) randomValue(depth int) any {
	switch r.intn(6) {
	case 0:
		return r.intn(2) == 1
	case 1:
		return nil
	case 2:
		return float64(r.next()%1e9) / 1e3 // 有小数位的 ES 数字规范化路径
	case 3:
		return float64(r.next() % 1000) // 整数值 float（ES6 短格式）
	case 4:
		return []any{r.randomValue(depth - 1), r.randomValue(depth - 1)}
	default:
		if depth <= 0 {
			return r.randomKey()
		}
		m := map[string]any{}
		for i := 0; i < 1+r.intn(3); i++ {
			m[r.randomKey()] = r.randomValue(depth - 1)
		}
		return m
	}
}

// randomKey 覆盖 UTF-16 码元排序的难点：CJK、拉丁混排、数字前缀。
func (r *lcg) randomKey() string {
	pools := []string{"键", "値", "აბ", "é", "zebra", "Zebra", "0key", "1key", "😀", "á"}
	k := pools[r.intn(len(pools))]
	if r.intn(3) == 0 {
		k += pools[r.intn(len(pools))]
	}
	return k
}

// shuffleKeyOrder 深度重排所有 map 的键序（M1 的输入变换）。
// map 本身无序，这里通过序列化→乱序重序列化制造显式键序差异。
func shuffleKeyOrder(r *lcg, v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		// 乱序回填不能改变 map（无序）——真正的键序差异在序列化阶段：
		// 用不同插入序的原始 JSON 文本验证 Canonicalize 对键序不敏感。
		for i := len(keys) - 1; i > 0; i-- {
			j := r.intn(i + 1)
			keys[i], keys[j] = keys[j], keys[i]
		}
		for _, k := range keys {
			out[k] = shuffleKeyOrder(r, x[k])
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = shuffleKeyOrder(r, e)
		}
		return out
	default:
		return v
	}
}

// marshalInKeyOrder 按给定键序逐键序列化（构造显式键序差异的 JSON 文本）。
func marshalInKeyOrder(v any, reverse bool) (string, error) {
	var b strings.Builder
	var walk func(any) error
	walk = func(x any) error {
		switch t := x.(type) {
		case map[string]any:
			keys := make([]string, 0, len(t))
			for k := range t {
				keys = append(keys, k)
			}
			// 插入序 = Go map 随机序；reverse 制造第二种确定序
			if reverse {
				for i, j := 0, len(keys)-1; i < j; i, j = i+1, j-1 {
					keys[i], keys[j] = keys[j], keys[i]
				}
			}
			b.WriteByte('{')
			for i, k := range keys {
				if i > 0 {
					b.WriteByte(',')
				}
				kb, err := json.Marshal(k)
				if err != nil {
					return err
				}
				b.Write(kb)
				b.WriteByte(':')
				if err := walk(t[k]); err != nil {
					return err
				}
			}
			b.WriteByte('}')
		case []any:
			b.WriteByte('[')
			for i, e := range t {
				if i > 0 {
					b.WriteByte(',')
				}
				if err := walk(e); err != nil {
					return err
				}
			}
			b.WriteByte(']')
		default:
			eb, err := json.Marshal(t)
			if err != nil {
				return err
			}
			b.Write(eb)
		}
		return nil
	}
	if err := walk(v); err != nil {
		return "", err
	}
	return b.String(), nil
}

const propertyRuns = 1000

// TestPropertyDigestKeyOrderInsensitive 是 M1 变形不变式：
// 同一 JSON 值任意键序序列化 → ContentDigest 全等（RFC 8785 A2 内容寻址）。
func TestPropertyDigestKeyOrderInsensitive(t *testing.T) {
	r := &lcg{state: 20260820}
	for i := 0; i < propertyRuns; i++ {
		v := r.randomValue(3)
		a, err := marshalInKeyOrder(v, false)
		if err != nil {
			t.Fatalf("marshal a: %v", err)
		}
		b, err := marshalInKeyOrder(v, true)
		if err != nil {
			t.Fatalf("marshal b: %v", err)
		}
		da, err := ContentDigest([]byte(a))
		if err != nil {
			t.Fatalf("digest a: %v", err)
		}
		db, err := ContentDigest([]byte(b))
		if err != nil {
			t.Fatalf("digest b: %v", err)
		}
		if da != db {
			t.Fatalf("M1 违例（第 %d 例）：键序不同 digest 不等\n a=%s\n b=%s", i, a, b)
		}
	}
}

// TestPropertyCanonicalizeIdempotent：规范化幂等——
// canonicalize(canonicalize(x)) == canonicalize(x)（G9 基础属性）。
func TestPropertyCanonicalizeIdempotent(t *testing.T) {
	r := &lcg{state: 87654321}
	for i := 0; i < propertyRuns; i++ {
		v := r.randomValue(3)
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		c1, err := CanonicalizeJSON(raw)
		if err != nil {
			t.Fatalf("canonicalize 1st: %v", err)
		}
		c2, err := CanonicalizeJSON(c1)
		if err != nil {
			t.Fatalf("canonicalize 2nd: %v", err)
		}
		if string(c1) != string(c2) {
			t.Fatalf("幂等性违例（第 %d 例）：\n c1=%s\n c2=%s", i, c1, c2)
		}
	}
}

// TestPropertyDigestFormat：digest 恒为 sha256:<64 hex>（契约 pattern）。
func TestPropertyDigestFormat(t *testing.T) {
	r := &lcg{state: 13579}
	for i := 0; i < propertyRuns; i++ {
		raw, err := json.Marshal(r.randomValue(2))
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		d, err := ContentDigest(raw)
		if err != nil {
			t.Fatalf("digest: %v", err)
		}
		if !strings.HasPrefix(d, "sha256:") || len(d) != 7+64 {
			t.Fatalf("digest 形态违例（第 %d 例）：%q", i, d)
		}
		for _, c := range d[7:] {
			if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
				t.Fatalf("digest 非小写 hex（第 %d 例）：%q", i, d)
			}
		}
	}
}

// TestPropertyDigestMutationSensitive 是 M4 变形不变式（抗碰撞）：
// 语义内容变异（注入唯一键）→ ContentDigest 必变。M1 保证键序无关，
// 本不变式保证反方向——不同内容必不同 digest；若破坏，A2 公理下的一切
// 内容寻址判重/去重（GoldenKey、content_hash、飞轮样本池）全部失效。
func TestPropertyDigestMutationSensitive(t *testing.T) {
	r := &lcg{state: 44443}
	for i := 0; i < propertyRuns; i++ {
		base := r.randomValue(3)
		obj, ok := base.(map[string]any)
		if !ok {
			obj = map[string]any{"wrap": base}
		}
		raw, err := json.Marshal(obj)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		d1, err := ContentDigest(raw)
		if err != nil {
			t.Fatalf("digest: %v", err)
		}
		mutated := make(map[string]any, len(obj)+1)
		for k, v := range obj {
			mutated[k] = v
		}
		mutated["__mut_"+fmt.Sprint(i)] = i
		raw2, err := json.Marshal(mutated)
		if err != nil {
			t.Fatalf("marshal2: %v", err)
		}
		d2, err := ContentDigest(raw2)
		if err != nil {
			t.Fatalf("digest2: %v", err)
		}
		if d1 == d2 {
			t.Fatalf("run %d: 语义变异未改变 digest（内容寻址判重失效）", i)
		}
	}
}
