// calibrate_test.go —— 卡 #121（IR-0007 AC-8 / BEH-7）：裁判校准
// 矩阵口径单测 + 已提交标注集 fake 负对照端到端（确定性 digest）。
package calibrate_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Cloudbird-Software/Shorts_Director/internal/calibrate"
	"github.com/Cloudbird-Software/Shorts_Director/internal/operator"
)

const committedLabels = "../../evals/human_labels/labels.json"

// TestMatrixAgreement：混淆矩阵与一致率口径（正例=人工可用）。
func TestMatrixAgreement(t *testing.T) {
	m := calibrate.Matrix{TruePositive: 7, FalsePositive: 3, TrueNegative: 8, FalseNegative: 2}
	if m.Total() != 20 {
		t.Fatalf("Total 应 20: %d", m.Total())
	}
	if got := m.Agreement(); got != 0.75 {
		t.Fatalf("一致率应 0.75: %v", got)
	}
	if (calibrate.Matrix{}).Agreement() != 0 {
		t.Fatal("空矩阵一致率应 0（无除零）")
	}
}

// TestLoadLabelsValidation：标注集校验拒收坏文件。
func TestLoadLabelsValidation(t *testing.T) {
	dir := t.TempDir()
	media := filepath.Join(dir, "m.png")
	if err := os.WriteFile(media, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "labels.json")
	good := `{"schema_version":1,"labels":[{"item_id":"a","media_path":"m.png",
		"question":"Q?","human_label":true,"labeler":"t@1"}]}`
	if err := os.WriteFile(path, []byte(good), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := calibrate.LoadLabels(path); err != nil {
		t.Fatalf("合法标注集被拒: %v", err)
	}
	for name, doc := range map[string]string{
		"版本不符": `{"schema_version":2,"labels":[]}`,
		"空集":   `{"schema_version":1,"labels":[]}`,
		"缺标注人": `{"schema_version":1,"labels":[{"item_id":"a","media_path":"m.png","question":"Q?","human_label":true}]}`,
		"媒体缺失": `{"schema_version":1,"labels":[{"item_id":"a","media_path":"nope.png","question":"Q?","human_label":true,"labeler":"t@1"}]}`,
		"ID重复": `{"schema_version":1,"labels":[{"item_id":"a","media_path":"m.png","question":"Q?","human_label":true,"labeler":"t@1"},{"item_id":"a","media_path":"m.png","question":"Q?","human_label":false,"labeler":"t@1"}]}`,
	} {
		if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := calibrate.LoadLabels(path); err == nil {
			t.Errorf("%s 应被拒收", name)
		}
	}
}

// TestCalibrateFakeNegativeControl：已提交标注集（100 条）× fake 后端
// （无语义负对照）——矩阵覆盖全部条目、零错误、双跑 digest 全等
// （校准仪器确定性；一致率数值本身不设期望——fake 无语义）。
func TestCalibrateFakeNegativeControl(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 缺失")
	}
	labels, err := calibrate.LoadLabels(committedLabels)
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join("..", "..", "operators", "vlm_boolean", "run.sh")
	var digests []string
	for i := 0; i < 2; i++ {
		rep, err := calibrate.Run(context.Background(), calibrate.Options{
			Labels: labels, LabelsPath: committedLabels,
			Runner: &operator.LocalRunner{Bin: bin}, Model: "fake",
			WorkdirRoot: t.TempDir(),
		})
		if err != nil {
			t.Fatal(err)
		}
		if rep.Errors != 0 {
			t.Fatalf("fake 后端不应有探针错误: %d", rep.Errors)
		}
		if got := rep.Matrix.Total(); got != len(labels) {
			t.Fatalf("矩阵应覆盖全部 %d 条: %d", len(labels), got)
		}
		if rep.Agreement <= 0 || rep.Agreement > 1 {
			t.Fatalf("一致率应在 (0,1]: %v", rep.Agreement)
		}
		if rep.Digest == "" {
			t.Fatal("报告缺 digest")
		}
		digests = append(digests, rep.Digest)
	}
	if digests[0] != digests[1] {
		t.Fatalf("校准报告不 deterministic: %s ≠ %s", digests[0], digests[1])
	}
}

// TestCalibrationDatasetIntegrity：标注集结构守护——100 条、
// 四类各 25、正例 25/负例 75（L-04 对抗占 75%）、标注人留痕。
func TestCalibrationDatasetIntegrity(t *testing.T) {
	labels, err := calibrate.LoadLabels(committedLabels)
	if err != nil {
		t.Fatal(err)
	}
	if len(labels) != 100 {
		t.Fatalf("校准集约百条（AC-8），得到 %d", len(labels))
	}
	byCat := map[string]int{}
	pos := 0
	for _, l := range labels {
		cat := strings.SplitN(l.ItemID, "_", 2)[0]
		byCat[cat]++
		if l.HumanLabel {
			pos++
		}
	}
	for _, cat := range []string{"sharp", "blur", "black", "distort"} {
		if byCat[cat] != 25 {
			t.Errorf("类别 %s 应 25 条: %d", cat, byCat[cat])
		}
	}
	if pos != 25 || byCat["sharp"] != pos {
		t.Fatalf("正例应恰为 sharp 25 条: pos=%d", pos)
	}
}
