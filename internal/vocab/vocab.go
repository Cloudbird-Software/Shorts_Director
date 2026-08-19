// Package vocab 是受控词表的运行期助手（Go 侧），
// 对应 src/contracts/vocab.ts 的 isVocabId/isDeprecated/replacedBy/
// equivalenceClassOf/zhOf。数据一律来自 codegen/go/vocab（make gen 产物）。
package vocab

import (
	"errors"
	"fmt"

	vocabgen "github.com/Cloudbird-Software/Shorts_Director/codegen/go/vocab"
)

// ErrUnknownVocab 表示词表名不在 VocabFiles 清单内。
var ErrUnknownVocab = errors.New("vocab: unknown vocab name")

// ErrUnknownValue 表示 id 不在该词表内。
var ErrUnknownValue = errors.New("vocab: unknown value id")

// Lookup 返回词表值的完整元数据；词表名或 id 非法时返回包装错误。
func Lookup(name, id string) (vocabgen.Meta, error) {
	meta, ok := vocabgen.VocabMeta[name]
	if !ok {
		return vocabgen.Meta{}, fmt.Errorf("%w: %q", ErrUnknownVocab, name)
	}
	m, ok := meta[id]
	if !ok {
		return vocabgen.Meta{}, fmt.Errorf("%w: %s/%s", ErrUnknownValue, name, id)
	}
	return m, nil
}

// IsVocabID 报告 id 是否为词表 name 的合法枚举值。
func IsVocabID(name, id string) bool {
	ids, ok := vocabgen.VocabIDs[name]
	if !ok {
		return false
	}
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

// IsDeprecated 报告值是否已废弃（词表名或 id 非法时视为未废弃并返回错误）。
func IsDeprecated(name, id string) (bool, error) {
	m, err := Lookup(name, id)
	if err != nil {
		return false, err
	}
	return m.Deprecated, nil
}

// ReplacedBy 返回废弃值的替代 id；值未废弃或无替代时 ok=false。
func ReplacedBy(name, id string) (replacement string, ok bool, err error) {
	m, err := Lookup(name, id)
	if err != nil {
		return "", false, err
	}
	if m.ReplacedBy == nil {
		return "", false, nil
	}
	return *m.ReplacedBy, true, nil
}

// EquivalenceClassOf 返回值所属的等价类清单（按类取材的依据）。
func EquivalenceClassOf(name, id string) ([]string, error) {
	m, err := Lookup(name, id)
	if err != nil {
		return nil, err
	}
	return m.EquivalenceClass, nil
}

// ZhOf 返回值的中文展示名。
func ZhOf(name, id string) (string, error) {
	m, err := Lookup(name, id)
	if err != nil {
		return "", err
	}
	return m.Zh, nil
}
