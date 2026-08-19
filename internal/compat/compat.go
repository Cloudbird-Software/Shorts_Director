// Package compat 是前向兼容的消费边界（Freeze Gate G8）。
// 两侧护栏分工：实体校验（Go Validate / TS ajv）是**写侧**——契约漂移必须拒；
// 消费边界（读取上游未来版本的数据）是**读侧**——未知内容必须降级而不是崩溃。
// 本包锁定两条读侧路径：
//
//   - 未知字段忽略：encoding/json 默认忽略未知字段；DecodeTolerant 是统一
//     入口，禁止在消费边界引入 DisallowUnknownFields（那是防漂移测试的武器，
//     不是运行期消费的行为）。
//   - 未知枚举降级：值落 UNKNOWN 哨兵并以 Raw 保留原值——调用方可记录、
//     审计与按语义降级（如未知 ShotState 一律按不可消费处理，fail-safe）。
package compat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// UnknownEnumValue 是未知枚举的哨兵值（降级终态，调用方按 fail-safe 处理）。
const UnknownEnumValue = "UNKNOWN"

// Enum 是降级后的枚举值：已知值原样保留；未知值 Value=UNKNOWN、Raw 存原串。
type Enum struct {
	Value string
	Raw   string // 未知时为原始值；已知时与 Value 相同（统一读取面）
}

// Unknown 报告是否为降级值。
func (e Enum) Unknown() bool { return e.Value == UnknownEnumValue && e.Raw != "" }

// DegradeEnum 按 allowed 谓词降级单个枚举值（G8 路径 2 的最小单元）。
func DegradeEnum(raw string, allowed func(string) bool) Enum {
	if allowed(raw) {
		return Enum{Value: raw, Raw: raw}
	}
	return Enum{Value: UnknownEnumValue, Raw: raw}
}

// DecodeTolerant 在"未知字段忽略"语义下解码 JSON（G8 路径 1 的统一入口）。
// 返回的错误只来自语法/类型不匹配——未知字段永远不是错误。
func DecodeTolerant(data []byte, out any) error {
	return json.NewDecoder(bytes.NewReader(data)).Decode(out)
}

// UnknownEnum 记录一处未知枚举（JSON Pointer 定位 + 原始值）。
type UnknownEnum struct {
	Pointer string // RFC 6901，如 /state、/semantic/scene
	Raw     string
}

// ScanUnknownEnums 按 paths（JSON Pointer → 合法值谓词）扫描 data，
// 返回全部未知枚举位置。路径不存在则跳过；JSON 不合法返回错误。
// 消费方典型用法：解码前扫描 → 记日志/审计 → 决定降级或拒绝。
func ScanUnknownEnums(data []byte, paths map[string]func(string) bool) ([]UnknownEnum, error) {
	var doc any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("compat: %w", err)
	}
	var out []UnknownEnum
	for pointer, allowed := range paths {
		if v, ok := atPointer(doc, pointer); ok {
			if s, isStr := v.(string); isStr && !allowed(s) {
				out = append(out, UnknownEnum{Pointer: pointer, Raw: s})
			}
		}
	}
	return out, nil
}

// atPointer 解析 RFC 6901 JSON Pointer（对象段与数组下标段，~1/~0 转义）。
func atPointer(doc any, pointer string) (any, bool) {
	if pointer == "" {
		return doc, true
	}
	cur := doc
	for _, seg := range strings.Split(strings.TrimPrefix(pointer, "/"), "/") {
		seg = strings.ReplaceAll(strings.ReplaceAll(seg, "~1", "/"), "~0", "~")
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		if cur, ok = obj[seg]; !ok {
			return nil, false
		}
	}
	return cur, true
}
