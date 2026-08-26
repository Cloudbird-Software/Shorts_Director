// Package jsoncmp 提供 JSON 解码形态值的跨形态比较原语。
//
// JSON 反序列化后数值恒为 float64，但运行期合成字段也可能是 int（模板渲染、
// 计数列）。slotquery 谓词求值与 qc expect 比较需要完全一致的判定语义
// （"同内容同判定"是 G2 跨语言一致性测试的前置），故在此单点维护，
// 避免逐包复制出细微漂移。
package jsoncmp

// Float 把 JSON 数值统一为 float64（接受 float64 与 int 两种解码/构造形态）。
func Float(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int:
		return float64(x), true
	}
	return 0, false
}

// Equal 跨 JSON 解码形态比较 number/string/bool；类型不同即为不等。
func Equal(a, b any) bool {
	if ab, ok := a.(bool); ok {
		bb, ok2 := b.(bool)
		return ok2 && ab == bb
	}
	if af, ok := Float(a); ok {
		if bf, ok2 := Float(b); ok2 {
			return af == bf
		}
		return false
	}
	as, aok := a.(string)
	bs, bok := b.(string)
	return aok && bok && as == bs
}
