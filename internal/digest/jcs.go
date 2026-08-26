// Package digest 实现 RFC 8785（JCS，JSON Canonicalization Scheme）与
// 内容寻址摘要。这是全系统 A2 公理（一切非确定性显式落盘为内容寻址
// artifact）的 Go 侧公共底座，TS/Go 跨语言摘要必须逐字节一致。
package digest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
)

// ErrBadJSON 表示输入不是合法 JSON。
var ErrBadJSON = errors.New("digest: invalid JSON")

// ErrUnsupportedType 表示规范化只支持 JSON 数据模型
// （nil/bool/float64/string/[]any/map[string]any）。
var ErrUnsupportedType = errors.New("digest: unsupported type for canonicalization")

// CanonicalizeJSON 规范化 JSON 字节流（RFC 8785）：
// 数值按 ECMAScript Number::toString 序列化、key 按 UTF-16 码元排序、
// 无空白、字符串最小化转义。
func CanonicalizeJSON(raw []byte) ([]byte, error) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadJSON, err)
	}
	var b strings.Builder
	if err := writeCanonical(&b, v); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

// ContentDigest 返回内容寻址摘要 "sha256:<64 hex 小写>"，
// 与 schema 中 content_hash 的 pattern ^sha256:[0-9a-f]{64}$ 一致。
func ContentDigest(raw []byte) (string, error) {
	canon, err := CanonicalizeJSON(raw)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canon)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// ValueDigest 对内存中的值一步求内容寻址摘要：
// json.Marshal（数值语义=IEEE 754 double，与 JCS 一致）+ 规范化 + sha256。
// GoldenKey、probe 去重键等"请求字段摘要"共用此入口。
func ValueDigest(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return ContentDigest(raw)
}

// writeCanonical 递归写出规范化 JSON。
func writeCanonical(b *strings.Builder, v any) error {
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		if x {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case float64:
		s, err := esNumber(x)
		if err != nil {
			return err
		}
		b.WriteString(s)
	case string:
		writeString(b, x)
	case []any:
		b.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			if err := writeCanonical(b, e); err != nil {
				return err
			}
		}
		b.WriteByte(']')
	case map[string]any:
		b.WriteByte('{')
		for i, k := range sortedKeysUTF16(x) {
			if i > 0 {
				b.WriteByte(',')
			}
			writeString(b, k)
			b.WriteByte(':')
			if err := writeCanonical(b, x[k]); err != nil {
				return err
			}
		}
		b.WriteByte('}')
	default:
		return fmt.Errorf("%w: %T", ErrUnsupportedType, v)
	}
	return nil
}

// sortedKeysUTF16 按 UTF-16 码元序排序 key（RFC 8785 §3.2.3）。
// 星体平面字符的代理对（0xD800–0xDFFF）小于 BMP 高区（如 U+E000），
// 与 UTF-8 字节序相反——必须显式编码为 []uint16 再比较。
func sortedKeysUTF16(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return utf16Less(keys[i], keys[j])
	})
	return keys
}

func utf16Less(a, b string) bool {
	ua, ub := utf16.Encode([]rune(a)), utf16.Encode([]rune(b))
	for i := 0; i < len(ua) && i < len(ub); i++ {
		if ua[i] != ub[i] {
			return ua[i] < ub[i]
		}
	}
	return len(ua) < len(ub)
}

// writeString 写出最小化转义的字符串（RFC 8785 §3.2.2.2）。
func writeString(b *strings.Builder, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
}

// esNumber 按 ECMAScript Number::toString(10) 序列化 double（RFC 8785 §3.2.2.3）。
func esNumber(x float64) (string, error) {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return "", fmt.Errorf("%w: NaN/Infinity 不是合法 JSON 数值", ErrUnsupportedType)
	}
	if x == 0 {
		return "0", nil // 含 -0：ES 规定 (-0).toString() === "0"
	}
	if x < 0 {
		s, err := esNumber(-x)
		return "-" + s, err
	}
	// 最短 round-trip 科学计数（保证与 ES 的"最短表示"一致），再重排为 ES 形式。
	sci := strconv.FormatFloat(x, 'e', -1, 64)
	mant, expStr, _ := strings.Cut(sci, "e")
	exp, _ := strconv.Atoi(expStr)
	mant = strings.TrimSuffix(mant, ".0")
	digits := strings.Replace(mant, ".", "", 1)
	point := strings.Index(mant, ".") // 小数点位置；无点则 = len(mant)
	if point < 0 {
		point = len(mant)
	}
	n := exp + point - 1 // 十进制指数：value = 0.digits × 10^(n+1)

	switch {
	case n >= 21 || n < -6:
		// 指数形式 d[.ddd]e±nn —— 指数符号必写、无前导零。
		sign := "+"
		if n < 0 {
			sign = "-"
			n = -n
		}
		head := digits[0]
		tail := digits[1:]
		if tail != "" {
			return fmt.Sprintf("%c.%se%s%d", head, tail, sign, n), nil
		}
		return fmt.Sprintf("%ce%s%d", head, sign, n), nil
	case n >= 0:
		// positional：digits 不足以填满整数位时右侧补零（如 1E+2 → "100"）。
		for len(digits) < n+1 {
			digits += "0"
		}
		intPart := digits[:n+1]
		frac := digits[n+1:]
		if frac == "" {
			return intPart, nil
		}
		return intPart + "." + frac, nil
	default: // -6 <= n < 0：0.000ddd
		return "0." + strings.Repeat("0", -n-1) + digits, nil
	}
}
