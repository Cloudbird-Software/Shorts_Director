package contracts_test

// baml_lint_test 是 C1 契约（BAML）的 Go 侧结构守卫：
// CI 无法调 LLM（无密钥），但契约的结构条款必须可机检——
//   B-1：class 字段的非基本类型必须解析到已声明 enum/class（词表真源 vocab.baml）；
//   B-4：每个 function ≥5 个 test block，其中 ≥2 个对抗样本（名字含 adversarial）。
// 解析器是行导向的轻量实现：BAML 花括号块在冻结契约里不换行嵌套花括号，
// 足够覆盖当前与可预见的契约形态；不是通用 BAML parser。

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	reEnum     = regexp.MustCompile(`^enum\s+([A-Z][A-Za-z0-9]*)\s*\{`)
	reClass    = regexp.MustCompile(`^class\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{`)
	reFunction = regexp.MustCompile(`^function\s+([A-Za-z0-9_]+)\s*\(`)
	reTest     = regexp.MustCompile(`^test\s+([A-Za-z0-9_]+)\s*\{`)
	reField    = regexp.MustCompile(`^\s{2}([a-z_][a-z0-9_]*)\s+([A-Za-z0-9_?\[\]"\s|]+?)(?:\s+@description|\s*$)`)
	reFuncs    = regexp.MustCompile(`^\s{2}functions\s+\[([A-Za-z0-9_,\s]+)\]`)
)

type bamlFile struct {
	enums     map[string]bool
	classes   map[string]bool
	functions []string
	tests     map[string][]string // function → test 名列表
	lastTest  string              // 解析游标：最近一个 test block 名
}

func parseBaml(t *testing.T, path string) bamlFile {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读 %s: %v", path, err)
	}
	f := bamlFile{enums: map[string]bool{}, classes: map[string]bool{}, tests: map[string][]string{}}
	for _, line := range strings.Split(string(raw), "\n") {
		switch {
		case reEnum.MatchString(line):
			f.enums[reEnum.FindStringSubmatch(line)[1]] = true
			continue
		case reClass.MatchString(line):
			f.classes[reClass.FindStringSubmatch(line)[1]] = true
			continue
		case strings.HasPrefix(line, "}"):
			continue
		}
		if m := reFunction.FindStringSubmatch(line); m != nil {
			f.functions = append(f.functions, m[1])
			continue
		}
		if m := reTest.FindStringSubmatch(line); m != nil {
			f.lastTest = m[1]
			continue
		}
		if m := reFuncs.FindStringSubmatch(line); m != nil && f.lastTest != "" {
			for _, fn := range strings.Split(m[1], ",") {
				fn = strings.TrimSpace(fn)
				if fn != "" {
					f.tests[fn] = append(f.tests[fn], f.lastTest)
				}
			}
		}
	}
	return f
}

// 基本类型白名单：这些 token 允许直接做字段类型（B-1 禁的是"自由 string 表达受控域"）。
var primitives = map[string]bool{
	"string": true, "int": true, "float": true, "bool": true, "image": true,
	"true": true, "false": true, // 字面量联合分支
}

// TestBAMLStructuralContract：B-1 类型可解析 + B-4 测试规模。
func TestBAMLStructuralContract(t *testing.T) {
	dir := "../../baml_src"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读 baml_src: %v", err)
	}

	// 第一遍：收集全部声明（enum/class 可跨文件引用，vocab.baml 是词表真源）。
	declared := map[string]bool{}
	var files []bamlFile
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".baml") {
			continue
		}
		f := parseBaml(t, filepath.Join(dir, e.Name()))
		files = append(files, f)
		for n := range f.enums {
			declared[n] = true
		}
		for n := range f.classes {
			declared[n] = true
		}
	}

	for i, f := range files {
		// B-1：class 字段的类型 token 必须是基本类型、字面量或已声明类型。
		raw, _ := os.ReadFile(filepath.Join(dir, entries[i].Name()))
		var curClass string
		for _, line := range strings.Split(string(raw), "\n") {
			if m := reClass.FindStringSubmatch(line); m != nil {
				curClass = m[1]
				continue
			}
			if strings.HasPrefix(line, "}") {
				curClass = ""
				continue
			}
			if curClass == "" {
				continue
			}
			m := reField.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			typeExpr := strings.TrimSpace(m[2])
			// 按 | 拆联合；每个分支剥 ? / [] 修饰后必须是
			// 基本类型、字符串字面量（带引号）或已声明 enum/class。
			for _, alt := range strings.Split(typeExpr, "|") {
				alt = strings.TrimSpace(alt)
				alt = strings.NewReplacer("?", "", "[", " ", "]", " ").Replace(alt)
				for _, tok := range strings.Fields(alt) {
					if strings.HasPrefix(tok, `"`) || primitives[tok] || declared[tok] {
						continue
					}
					t.Errorf("%s: 字段 %s 的类型 %q 未声明（B-1：受控域必须引用 codegen enum）",
						entries[i].Name(), m[1], tok)
				}
			}
		}

		// B-4：每个 function ≥5 tests，≥2 adversarial。
		for _, fn := range f.functions {
			tests := f.tests[fn]
			if len(tests) < 5 {
				t.Errorf("%s: function %s 仅 %d 个 test block（B-4 要求 ≥5）: %v",
					entries[i].Name(), fn, len(tests), tests)
			}
			adv := 0
			for _, name := range tests {
				if strings.Contains(name, "adversarial") {
					adv++
				}
			}
			if adv < 2 {
				t.Errorf("%s: function %s 仅 %d 个对抗样本（B-4 要求 ≥2）",
					entries[i].Name(), fn, adv)
			}
		}
	}
}

// TestVocabBamlIsGenerated：vocab.baml 必须携带生成标记（禁手改的机检形态）。
func TestVocabBamlIsGenerated(t *testing.T) {
	raw, err := os.ReadFile("../../baml_src/vocab.baml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "AUTO-GENERATED") {
		t.Error("vocab.baml 缺 AUTO-GENERATED 标记——词表真源必须由 make gen 生成")
	}
}

// TestBAMLTestDataRefs：test block 引用的关键帧文件必须真实存在。
func TestBAMLTestDataRefs(t *testing.T) {
	reFile := regexp.MustCompile(`\{\s*file\s+"([^"]+)"\s*\}`)
	entries, err := os.ReadDir("../../baml_src")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".baml") {
			continue
		}
		raw, _ := os.ReadFile(filepath.Join("../../baml_src", e.Name()))
		for _, m := range reFile.FindAllStringSubmatch(string(raw), -1) {
			p := filepath.Join("../../baml_src", m[1])
			if _, err := os.Stat(p); err != nil {
				t.Errorf("%s: 引用的测试素材缺失: %s", e.Name(), m[1])
			}
		}
	}
}
