// eval_test.go —— 卡 #115（IR-0007 AC-4/AC-5）：套件校验、golden 查表执行、
// 出片率复算（property）、预算截断、断言重试、并发 -race、digest 钉死。
package eval_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Cloudbird-Software/Shorts_Director/internal/contracts"
	"github.com/Cloudbird-Software/Shorts_Director/internal/eval"
	"github.com/Cloudbird-Software/Shorts_Director/internal/operator"
	"github.com/Cloudbird-Software/Shorts_Director/internal/qc"
)

const goldenRoot = "../../testdata/golden"

// stubGen 是可编程的生成算子替身：按 seed 决定 OK / INPUT_ERROR / 系统错误。
type stubGen struct {
	mu       sync.Mutex
	calls    int
	behavior func(seed int64) (operator.Response, error)
}

func (s *stubGen) Run(ctx context.Context, req operator.Request) (operator.Response, error) {
	s.mu.Lock()
	s.calls++
	s.mu.Unlock()
	seed := int64(0)
	if req.Determinism.Seed != nil {
		seed = *req.Determinism.Seed
	}
	return s.behavior(seed)
}

func okResp(seed int64) operator.Response {
	return operator.Response{
		ContractVersion: contracts.ContractOperator, Op: "gen_i2v", Status: operator.StatusOK,
		Outputs: map[string]any{
			"video_path":   fmt.Sprintf("/tmp/w/seed-%d.mp4", seed),
			"content_hash": fmt.Sprintf("sha256:%064x", seed),
		},
		Metrics:         operator.Metrics{WallMs: 100, GpuSecond: 1.5, PeakMemMB: 2048},
		OperatorVersion: "gen_i2v@test", ModelVersions: map[string]string{"model": "stub"},
	}
}

// stubProbe 是可编程断言探针：measure 由测试注入（含失败计数，验证重试；
// 可感知 subject 的 fields——entry_id/seed/gen_form 等）。
type stubProbe struct {
	id      string
	tier    qc.CostTier
	measure func(call int, subj *qc.Subject) (qc.Measurement, error)
	calls   int
}

func (p *stubProbe) ID() string        { return p.id }
func (p *stubProbe) Cost() qc.CostTier { return p.tier }
func (p *stubProbe) Measure(ctx context.Context, s *qc.Subject, args map[string]any) (qc.Measurement, error) {
	p.calls++
	return p.measure(p.calls, s)
}

func engineWith(probes ...qc.ProbeOperator) *qc.Engine {
	e, err := qc.NewEngine(probes...)
	if err != nil {
		panic(err)
	}
	return e
}

// passAssert 构造一条必过断言（probe op 在 29 值白名单内）。
func passAssert() qc.Assertion {
	return qc.Assertion{
		AssertionID: "L0.test_assert", Level: qc.L0, Severity: qc.SeverityBlocker,
		Probe:  qc.Probe{Op: "resolution", Args: map[string]any{}},
		Expect: qc.Expect{Op: "gte", Value: 1},
		Remedy: qc.Remedy{Action: "REGENERATE", InstructionTemplate: "重抽 {{measured}}"},
	}
}

func baseSuite() *eval.Suite {
	return &eval.Suite{
		SchemaVersion: 1, SuiteID: "t", GenForm: "I2V_AMBIENCE", Op: "gen_i2v",
		Model: "fake", Seeds: []int64{1, 2}, FPS: 16,
		Entries: []eval.Entry{
			{ID: "a", ImagePath: "/tmp/a.jpg", Prompt: "p", DurationSec: 5},
			{ID: "b", ImagePath: "/tmp/b.jpg", Prompt: "p", DurationSec: 5},
		},
		AssertionPack: nil,
		Budget:        eval.Budget{WallSeconds: 600, GpuSeconds: 1000},
	}
}

func TestSuiteValidate(t *testing.T) {
	cases := map[string]func(*eval.Suite){
		"schema":  func(s *eval.Suite) { s.SchemaVersion = 2 },
		"genform": func(s *eval.Suite) { s.GenForm = "NOPE" },
		"op":      func(s *eval.Suite) { s.Op = "gen_magic" },
		"model":   func(s *eval.Suite) { s.Model = "" },
		"seeds":   func(s *eval.Suite) { s.Seeds = nil },
		"fps":     func(s *eval.Suite) { s.FPS = 0 },
		"entries": func(s *eval.Suite) { s.Entries = nil },
		"dup_ids": func(s *eval.Suite) { s.Entries[1].ID = "a" },
		"budget":  func(s *eval.Suite) { s.Budget.WallSeconds = 0 },
		"assert":  func(s *eval.Suite) { s.AssertionPack = []qc.Assertion{{AssertionID: "X"}} },
	}
	for name, mutate := range cases {
		s := baseSuite()
		mutate(s)
		if err := s.Validate(); err == nil {
			t.Errorf("%s: 期望校验失败", name)
		}
	}
	if err := baseSuite().Validate(); err != nil {
		t.Fatalf("合法套件被拒: %v", err)
	}
}

// TestRunYield：2 条目 × 2 seed——a 条目断言全败、b 条目恰 1 条可用
// → 出片率 1/2（IFACE-5：K 抽至少 1 条可用的条目比例）。
func TestRunYield(t *testing.T) {
	gen := &stubGen{behavior: func(seed int64) (operator.Response, error) {
		return okResp(seed), nil
	}}
	probe := &stubProbe{id: "resolution", tier: qc.CostFree, measure: func(call int, subj *qc.Subject) (qc.Measurement, error) {
		entry, _ := subj.Fields["entry_id"].(string)
		seed, _ := subj.Fields["seed"].(int64)
		switch {
		case entry == "a":
			return qc.Measurement{Value: 720, EvidenceURI: "file:///evidence-a"}, nil // a 全败
		case seed == 2:
			return qc.Measurement{Value: 720, EvidenceURI: "file:///evidence-b2"}, nil // b 第 2 抽败
		}
		return qc.Measurement{Value: 1080, EvidenceURI: "file:///evidence-b1"}, nil
	}}
	s := baseSuite()
	s.AssertionPack = []qc.Assertion{{
		AssertionID: "L0.res", Level: qc.L0, Severity: qc.SeverityBlocker,
		Probe:  qc.Probe{Op: "resolution", Args: map[string]any{}},
		Expect: qc.Expect{Op: "eq", Value: 1080},
		Remedy: qc.Remedy{Action: "REGENERATE", InstructionTemplate: "分辨率 {{measured}}"},
	}}
	art, err := eval.Run(context.Background(), eval.RunOptions{
		Suite: s, Gen: gen, Engine: engineWith(probe),
		ProfileRef: "sha256:ab", RunnerMode: "fake", WorkdirRoot: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	y := art.Yield
	if y.ItemsTotal != 4 || y.ItemsUsable != 1 || y.ItemsAssertFail != 3 ||
		y.EntriesTotal != 2 || y.EntriesWithUsable != 1 || y.YieldRatio != 0.5 {
		t.Fatalf("出片率聚合错误: %+v", y)
	}
	if art.Items[0].Status != eval.ItemAssertFail || art.Items[0].Usable {
		t.Fatalf("断言失败条目应不可用: %+v", art.Items[0])
	}
	if len(art.Items[1].Assertions) != 1 || art.Items[1].Assertions[0].EvidenceURI == "" {
		t.Fatalf("判定明细缺证据 URI: %+v", art.Items[1].Assertions)
	}
}

// TestRunGoldenFakeRunner：FakeRunner 命中 testdata/golden/gen_i2v fixtures
// （两个 entry 的请求与 fixture 完全一致，含 seed）——零 Python 依赖的仪器自测。
func TestRunGoldenFakeRunner(t *testing.T) {
	s := &eval.Suite{
		SchemaVersion: 1, SuiteID: "golden", GenForm: "I2V_AMBIENCE", Op: "gen_i2v",
		Model: "fake", FPS: 16, Params: map[string]any{"width": float64(576), "height": float64(1024)},
		Entries: []eval.Entry{{
			ID: "noodles", ImagePath: "/mnt/assets/noodles_hero.jpg",
			Prompt: "一碗热气腾腾的牛肉面特写，缓慢推近，蒸汽升腾", DurationSec: 3,
		}},
		Seeds:  []int64{7},
		Budget: eval.Budget{WallSeconds: 600, GpuSeconds: 100},
	}
	art, err := eval.Run(context.Background(), eval.RunOptions{
		Suite: s, Gen: &operator.FakeRunner{Dir: goldenRoot}, Engine: engineWith(),
		ProfileRef: "sha256:" + "cd", RunnerMode: "fake", WorkdirRoot: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(art.Items) != 1 || art.Items[0].Status != eval.ItemOK || !art.Items[0].Usable {
		t.Fatalf("golden 抽卡应 OK 可用: %+v", art.Items)
	}
	if art.Items[0].ContentHash == "" || len(art.Items[0].ModelVersions) == 0 {
		t.Fatalf("产物哈希/model_versions 缺失: %+v", art.Items[0])
	}
	if art.Yield.YieldRatio != 1.0 || art.Yield.EntriesTotal != 1 {
		t.Fatalf("出片率聚合错误: %+v", art.Yield)
	}
	if art.Digest == "" {
		t.Fatal("artifact 缺 digest")
	}
}

// fakeClock 定步进时钟：每次调用 +step（预算截断可确定性触发）。
type fakeClock struct {
	mu   sync.Mutex
	now  time.Time
	step time.Duration
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(c.step)
	return c.now
}

// TestBudgetTruncation：wall 预算 10s、步进 10s → 第二条起全部截断（BUDGET-3）。
func TestBudgetTruncation(t *testing.T) {
	s := baseSuite()
	s.Seeds = []int64{1}
	s.Budget = eval.Budget{WallSeconds: 10}
	clk := &fakeClock{now: time.Unix(0, 0), step: 10 * time.Second}
	gen := &stubGen{behavior: func(seed int64) (operator.Response, error) {
		return okResp(seed), nil
	}}
	art, err := eval.Run(context.Background(), eval.RunOptions{
		Suite: s, Gen: gen, Engine: engineWith(),
		RunnerMode: "fake", WorkdirRoot: t.TempDir(), Now: clk.Now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !art.BudgetTruncated || art.Yield.ItemsSkippedBudget == 0 {
		t.Fatalf("预算截断未生效: %+v", art.Yield)
	}
	if art.Yield.ItemsTotal != 2 || art.Yield.ItemsTotal != len(art.Items) {
		t.Fatalf("截断后条目仍须落盘标注: %+v", art.Yield)
	}
}

// TestAssertRetry：断言基础设施故障重试 1 次内恢复 → 条目 OK；持续故障 →
// ASSERT_ERROR（BUDGET-2）。
func TestAssertRetry(t *testing.T) {
	newSuite := func() *eval.Suite {
		s := baseSuite()
		s.Seeds = []int64{1}
		s.Entries = s.Entries[:1]
		s.AssertionPack = []qc.Assertion{passAssert()}
		return s
	}
	// 第一次调用失败，第二次成功
	flaky := &stubProbe{id: "resolution", tier: qc.CostFree, measure: func(call int, _ *qc.Subject) (qc.Measurement, error) {
		if call == 1 {
			return qc.Measurement{}, fmt.Errorf("瞬时故障")
		}
		return qc.Measurement{Value: 1080}, nil
	}}
	art, err := eval.Run(context.Background(), eval.RunOptions{
		Suite: newSuite(), Gen: &stubGen{behavior: func(s int64) (operator.Response, error) {
			return okResp(s), nil
		}}, Engine: engineWith(flaky), WorkdirRoot: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if art.Items[0].Status != eval.ItemOK || flaky.calls != 2 {
		t.Fatalf("重试一次应恢复: %+v calls=%d", art.Items[0], flaky.calls)
	}
	dead := &stubProbe{id: "resolution", tier: qc.CostFree, measure: func(int, *qc.Subject) (qc.Measurement, error) {
		return qc.Measurement{}, fmt.Errorf("持续故障")
	}}
	art2, err := eval.Run(context.Background(), eval.RunOptions{
		Suite: newSuite(), Gen: &stubGen{behavior: func(s int64) (operator.Response, error) {
			return okResp(s), nil
		}}, Engine: engineWith(dead), WorkdirRoot: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if art2.Items[0].Status != eval.ItemAssertError || dead.calls != 2 || art2.Items[0].Usable {
		t.Fatalf("持续故障应 ASSERT_ERROR 且不重试超限: %+v calls=%d", art2.Items[0], dead.calls)
	}
}

// TestGenFail：算子 INPUT_ERROR → GEN_FAIL，保留可执行错误信息（证据）。
func TestGenFail(t *testing.T) {
	s := baseSuite()
	s.Seeds = []int64{1}
	s.Entries = s.Entries[:1]
	art, err := eval.Run(context.Background(), eval.RunOptions{
		Suite: s,
		Gen: &stubGen{behavior: func(int64) (operator.Response, error) {
			return operator.Response{}, fmt.Errorf("连接算子失败")
		}},
		Engine: engineWith(), WorkdirRoot: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if art.Items[0].Status != eval.ItemGenFail || art.Items[0].Error == "" || art.Items[0].Usable {
		t.Fatalf("生成失败应 GEN_FAIL 带证据: %+v", art.Items[0])
	}
}

// TestComputeYieldProperty：任意判定明细序列 → 出片率可复算（100 轮随机）。
// 参照实现独立手写，与 eval.ComputeYield 互证。
func TestComputeYieldProperty(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	statuses := []string{eval.ItemOK, eval.ItemGenFail, eval.ItemAssertFail, eval.ItemAssertError, eval.ItemSkippedBudget}
	for round := 0; round < 100; round++ {
		n := rng.Intn(30)
		items := make([]eval.ItemResult, n)
		for i := range items {
			st := statuses[rng.Intn(len(statuses))]
			items[i] = eval.ItemResult{
				EntryID: fmt.Sprintf("e%d", rng.Intn(5)), Seed: int64(rng.Intn(3)),
				Status: st, Usable: st == eval.ItemOK, GpuSeconds: float64(rng.Intn(10)),
			}
		}
		got := eval.ComputeYield(items)
		// 参照实现
		var itemsTotal, usable, genFail, assertFail, assertErr, skip int
		var gpu float64
		attempted, entryUsable := map[string]bool{}, map[string]bool{}
		for _, it := range items {
			itemsTotal++
			switch it.Status {
			case eval.ItemSkippedBudget:
				skip++
				continue
			case eval.ItemGenFail:
				genFail++
			case eval.ItemAssertFail:
				assertFail++
			case eval.ItemAssertError:
				assertErr++
			}
			attempted[it.EntryID] = true
			if it.Usable {
				usable++
				entryUsable[it.EntryID] = true
			}
			gpu += it.GpuSeconds
		}
		entries := 0
		withUsable := 0
		for id := range attempted {
			entries++
			if entryUsable[id] {
				withUsable++
			}
		}
		want := eval.Yield{
			EntriesTotal: entries, EntriesWithUsable: withUsable,
			ItemsTotal: itemsTotal, ItemsUsable: usable, ItemsGenFail: genFail,
			ItemsAssertFail: assertFail, ItemsAssertError: assertErr,
			ItemsSkippedBudget: skip, GpuSecondsTotal: gpu,
		}
		if entries > 0 {
			want.YieldRatio = float64(withUsable) / float64(entries)
		}
		if got != want {
			t.Fatalf("round %d: 复算不一致\n got %+v\nwant %+v", round, got, want)
		}
	}
}

// TestArtifactJSONRoundTrip：artifact 序列化往返后聚合仍可复算且 digest 稳定
// （IFACE-2：自 artifact 可复算）。
func TestArtifactJSONRoundTrip(t *testing.T) {
	gen := &stubGen{behavior: func(seed int64) (operator.Response, error) {
		return okResp(seed), nil
	}}
	opts := eval.RunOptions{
		Suite: baseSuite(), Gen: gen, Engine: engineWith(),
		ProfileRef: "sha256:profile", RunnerMode: "fake", WorkdirRoot: "/tmp/w",
		Now: func() time.Time { return time.Unix(1700000000, 0).UTC() },
	}
	art1, err := eval.Run(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(art1)
	if err != nil {
		t.Fatal(err)
	}
	var art2 eval.RunArtifact
	if err := json.Unmarshal(raw, &art2); err != nil {
		t.Fatal(err)
	}
	if eval.ComputeYield(art2.Items) != art1.Yield {
		t.Fatalf("往返后聚合漂移: %+v ≠ %+v", eval.ComputeYield(art2.Items), art1.Yield)
	}
	d2, err := art2.ComputeDigest()
	if err != nil || d2 != art1.Digest {
		t.Fatalf("往返后 digest 漂移: %q ≠ %q (%v)", d2, art1.Digest, err)
	}
	// 同输入同 Now 重跑 → digest 一致（确定性）
	art3, err := eval.Run(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if art3.Digest != art1.Digest {
		t.Fatalf("同输入重跑 digest 漂移: %q ≠ %q", art3.Digest, art1.Digest)
	}
	if art1.Suite.SuiteID != "t" || len(art1.Suite.Entries) != 2 {
		t.Fatal("artifact 未内嵌套件定义全文")
	}
}

// TestGoldenReportSample：golden 报告样本钉死——digest 可复算、聚合可自
// artifact 复算（IFACE-2）。样本漂移（断言/聚合/序列化任一改动）即失败。
func TestGoldenReportSample(t *testing.T) {
	raw, err := os.ReadFile("testdata/golden_report.json")
	if err != nil {
		t.Fatal(err)
	}
	var art eval.RunArtifact
	if err := json.Unmarshal(raw, &art); err != nil {
		t.Fatal(err)
	}
	if want := "sha256:9b86e4f7569441f14ee97e962e560e41f4f3952d6e4975c3377427c7c48e12fe"; art.Digest != want {
		t.Fatalf("golden 报告 digest 漂移: %s ≠ %s", art.Digest, want)
	}
	d, err := art.ComputeDigest()
	if err != nil || d != art.Digest {
		t.Fatalf("golden 报告 digest 不可复算: %q (%v)", d, err)
	}
	if got := eval.ComputeYield(art.Items); got != art.Yield {
		t.Fatalf("golden 报告聚合不可复算: %+v ≠ %+v", got, art.Yield)
	}
	if art.Yield.YieldRatio != 1 || art.Items[0].Status != eval.ItemOK || !art.Items[0].Usable {
		t.Fatalf("golden 报告样本语义异常: %+v", art.Yield)
	}
	if art.CapabilityProfileRef == "" || art.Suite.SuiteID == "" {
		t.Fatal("golden 报告缺 profile 引用或套件全文")
	}
}

// TestRunConcurrentRace：多套件并发执行，-race 下干净。
func TestRunConcurrentRace(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s := baseSuite()
			s.Seeds = []int64{int64(i)}
			art, err := eval.Run(context.Background(), eval.RunOptions{
				Suite: s,
				Gen: &stubGen{behavior: func(seed int64) (operator.Response, error) {
					return okResp(seed), nil
				}},
				Engine: engineWith(&stubProbe{
					id: "resolution", tier: qc.CostFree,
					measure: func(int, *qc.Subject) (qc.Measurement, error) {
						return qc.Measurement{Value: 1080}, nil
					},
				}),
				RunnerMode: "fake", WorkdirRoot: t.TempDir(),
			})
			if err != nil || art.Yield.ItemsTotal != 2 {
				t.Errorf("并发 run %d: err=%v yield=%+v", i, err, art.Yield)
			}
		}(i)
	}
	wg.Wait()
}
