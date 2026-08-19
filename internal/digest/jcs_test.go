package digest

import (
	"math"
	"regexp"
	"testing"
)

// RFC 8785 Appendix B 数值向量（ECMAScript Number::toString 规则）。
func TestEsNumber(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0, "0"},
		{-0, "0"},
		{1, "1"},
		{-1, "-1"},
		{4.50, "4.5"},
		{2e-3, "0.002"},
		{1e-6, "0.000001"},
		{1e-7, "1e-7"},
		{1e21, "1e+21"},
		{9.999999999999997e+22, "9.999999999999997e+22"},
		{1e28, "1e+28"},
		{333333333.33333329, "333333333.3333333"},
		{5e-20, "5e-20"},
		{1.0000000000000002, "1.0000000000000002"},
		{9007199254740992, "9007199254740992"}, // 2^53
	}
	for _, c := range cases {
		got, err := esNumber(c.in)
		if err != nil {
			t.Fatalf("esNumber(%v) err = %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("esNumber(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestEsNumberRejectsSpecial(t *testing.T) {
	for _, x := range []float64{nan(), inf(1), inf(-1)} {
		if _, err := esNumber(x); err == nil {
			t.Errorf("esNumber(%v) 应当报错", x)
		}
	}
}

// RFC 8785 §3.2.3：key 按 UTF-16 码元排序——星体平面的代理对(0xD83D)
// 小于 BMP 高区字符(U+E000)，与 UTF-8 字节序相反。
func TestKeyOrderUTF16(t *testing.T) {
	raw := []byte("{\"\ue000\":1,\"\U0001F600\":0}") // 私有区 vs 星体平面 emoji
	canon, err := CanonicalizeJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\"\\U0001F600\":0,\"\ue000\":1}"
	want = "{\"\U0001F600\":0,\"\ue000\":1}"
	if string(canon) != want {
		t.Errorf("canonical = %q, want %q", canon, want)
	}
}

func TestCanonicalizeBasics(t *testing.T) {
	cases := []struct{ in, want string }{
		// 空白剥离 + key 排序
		{`{ "b" : 2 , "a" : 1 }`, `{"a":1,"b":2}`},
		// 嵌套排序 + 数组保序
		{`{"z":{"y":2,"x":1},"a":[3,1,2]}`, `{"a":[3,1,2],"z":{"x":1,"y":2}}`},
		// 字符串最小转义（控制字符 \u 小写 hex，其余原样）
		{`["\"\\` + `\b\f\n\r\t` + `\u0001"]`, `["\"\\` + `\b\f\n\r\t` + `\u0001"]`},
		// 字面量
		{`[null,true,false]`, `[null,true,false]`},
		// 数值规范化：多余尾零/指数形式 → ES 形式
		{`{"n":4.50}`, `{"n":4.5}`},
		{`{"n":2e-3}`, `{"n":0.002}`},
		{`{"n":1E+2}`, `{"n":100}`},
	}
	for _, c := range cases {
		if c.want == "no" {
			continue
		}
		got, err := CanonicalizeJSON([]byte(c.in))
		if err != nil {
			t.Fatalf("CanonicalizeJSON(%q) err = %v", c.in, err)
		}
		if string(got) != c.want {
			t.Errorf("CanonicalizeJSON(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCanonicalizeErrors(t *testing.T) {
	if _, err := CanonicalizeJSON([]byte(`{bad`)); err == nil {
		t.Error("非法 JSON 应当报错")
	}
	if _, err := CanonicalizeJSON([]byte(`{"a":1}{"b":2}`)); err == nil {
		t.Error("尾随内容应当报错")
	}
}

func TestContentDigestFormat(t *testing.T) {
	// 规范化等价输入 → 同一摘要（内容寻址的根基）。
	re := regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	a, err := ContentDigest([]byte(`{ "a" : 1 , "b" : 2 }`))
	if err != nil {
		t.Fatal(err)
	}
	b, err := ContentDigest([]byte(`{"b":2,"a":1}`))
	if err != nil {
		t.Fatal(err)
	}
	if !re.MatchString(a) {
		t.Errorf("摘要格式非法: %q", a)
	}
	if a != b {
		t.Errorf("规范化等价输入摘要不同: %q vs %q", a, b)
	}
	// 一字节之差必须变摘要。
	c, _ := ContentDigest([]byte(`{"b":2,"a":2}`))
	if c == a {
		t.Error("不同内容产生相同摘要")
	}
}

func nan() float64 { return math.NaN() }

func inf(s int) float64 {
	if s > 0 {
		return math.Inf(1)
	}
	return math.Inf(-1)
}
