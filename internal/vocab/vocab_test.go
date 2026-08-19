package vocab

import (
	"errors"
	"testing"

	vocabgen "github.com/Cloudbird-Software/Shorts_Director/codegen/go/vocab"
)

func TestIsVocabID(t *testing.T) {
	cases := []struct {
		name, id string
		want     bool
	}{
		{"shot_type", "CLOSEUP", true},
		{"shot_type", "NOPE", false},
		{"not_a_vocab", "CLOSEUP", false},
		{"scene.food", "STOREFRONT", true},
	}
	for _, c := range cases {
		if got := IsVocabID(c.name, c.id); got != c.want {
			t.Errorf("IsVocabID(%q, %q) = %v, want %v", c.name, c.id, got, c.want)
		}
	}
}

func TestLookupErrors(t *testing.T) {
	if _, err := Lookup("nope", "X"); !errors.Is(err, ErrUnknownVocab) {
		t.Errorf("Lookup 未知词表错误 = %v, want ErrUnknownVocab", err)
	}
	if _, err := Lookup("shot_type", "X"); !errors.Is(err, ErrUnknownValue) {
		t.Errorf("Lookup 未知值错误 = %v, want ErrUnknownValue", err)
	}
}

func TestMetaHelpers(t *testing.T) {
	// 全量遍历生成的注册表：每个值可查元数据，zh 非空。
	for _, name := range vocabgen.VocabFiles {
		for _, id := range vocabgen.VocabIDs[name] {
			zh, err := ZhOf(name, id)
			if err != nil || zh == "" {
				t.Fatalf("ZhOf(%s,%s) = %q, %v", name, id, zh, err)
			}
			cls, err := EquivalenceClassOf(name, id)
			if err != nil || len(cls) == 0 {
				t.Fatalf("EquivalenceClassOf(%s,%s) = %v, %v", name, id, cls, err)
			}
			if _, _, err := ReplacedBy(name, id); err != nil {
				t.Fatalf("ReplacedBy(%s,%s) err = %v", name, id, err)
			}
			if _, err := IsDeprecated(name, id); err != nil {
				t.Fatalf("IsDeprecated(%s,%s) err = %v", name, id, err)
			}
		}
	}
}

func TestRegistryShape(t *testing.T) {
	if len(vocabgen.VocabFiles) < 14 {
		t.Fatalf("VocabFiles = %d, want >= 14", len(vocabgen.VocabFiles))
	}
	for _, name := range vocabgen.VocabFiles {
		if _, ok := vocabgen.VocabIDs[name]; !ok {
			t.Errorf("VocabIDs 缺 %s", name)
		}
		if _, ok := vocabgen.VocabMeta[name]; !ok {
			t.Errorf("VocabMeta 缺 %s", name)
		}
	}
}
