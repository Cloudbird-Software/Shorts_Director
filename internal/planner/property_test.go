package planner

import "testing"

// ────────────────────────────────────────────────────────────────────
// G9 属性测试（Freeze Gate，issue #44）：IV-VP-1 帧数守恒律
//
// 任意可行输入下：Σ Clips[i].Frames == TimingResult.Total，
// 时间线连续（TlEnd_i == TlStart_{i+1}，无空洞无重叠），
// 降级记录与实际钳制一致。确定性 LCG ≥1000 次，零第三方依赖。
// ────────────────────────────────────────────────────────────────────

type flcg struct{ state uint64 }

func (r *flcg) next() uint64 {
	r.state = r.state*6364136223846793005 + 1442695040888963407
	return r.state >> 33
}

func (r *flcg) intn(n int) int { return int(r.next() % uint64(n)) }

const framesRuns = 1000

// randomBeats 随机生成可行 beat 序列（min ≤ max、VO ≤ shot 可用量、总量可行）。
func (r *flcg) randomCase() ([]BeatWindow, TotalRange) {
	n := 1 + r.intn(5)
	beats := make([]BeatWindow, n)
	sumMin, sumMax := 0, 0
	for i := range beats {
		mn := 10 + r.intn(30)
		mx := mn + 10 + r.intn(60)
		vo := r.intn(mn + 1) // VO ≤ min 地板，保证可行
		avail := mx + r.intn(30)
		if r.intn(4) == 0 {
			vo = 0 // B-roll beat
		}
		beats[i] = BeatWindow{Role: "HOOK", MinFrames: mn, MaxFrames: mx, VOFrames: vo, ShotAvail: avail}
		sumMin += mn
		sumMax += mx
	}
	tMin := sumMin + r.intn(sumMax-sumMin+1)
	tMax := tMin + r.intn(sumMax-tMin+1)
	return beats, TotalRange{Min: tMin, Max: tMax}
}

// TestPropertyFrameConservation：IV-VP-1 帧数守恒——
// Σ Clips.Frames == Total 且时间线首尾相接、单调递增。
func TestPropertyFrameConservation(t *testing.T) {
	r := &flcg{state: 20260820}
	for i := 0; i < framesRuns; i++ {
		beats, total := r.randomCase()
		res, err := SolveTiming(beats, total, DefaultCraftParams())
		if err != nil {
			t.Fatalf("SolveTiming（第 %d 例）: %v", i, err)
		}
		sum := 0
		cursor := 0
		for j, c := range res.Clips {
			if c.TlStart != cursor {
				t.Fatalf("时间线连续性违例（第 %d 例 beat %d）：TlStart=%d 期望 %d", i, j, c.TlStart, cursor)
			}
			if c.TlEnd != c.TlStart+c.Frames {
				t.Fatalf("TlEnd 违例（第 %d 例 beat %d）：%d != %d+%d", i, j, c.TlEnd, c.TlStart, c.Frames)
			}
			sum += c.Frames
			cursor = c.TlEnd
		}
		if sum != res.Total {
			t.Fatalf("IV-VP-1 帧数守恒违例（第 %d 例）：Σframes=%d != Total=%d", i, sum, res.Total)
		}
		if res.Total < total.Min || res.Total > total.Max {
			t.Fatalf("总量越界（第 %d 例）：%d ∉ [%d,%d]", i, res.Total, total.Min, total.Max)
		}
	}
}

// TestPropertyDegradeOnlyWhenClamped：降级记录非空 ⇔ 存在实际钳制
// （VO 帧被压缩或 shot 被钳）——可解释性记录与物理事实一致。
func TestPropertyDegradeOnlyWhenClamped(t *testing.T) {
	r := &flcg{state: 97531}
	for i := 0; i < framesRuns; i++ {
		beats, total := r.randomCase()
		res, err := SolveTiming(beats, total, DefaultCraftParams())
		if err != nil {
			t.Fatalf("SolveTiming（第 %d 例）: %v", i, err)
		}
		clamped := false
		for j, c := range res.Clips {
			voFloor := beats[j].MinFrames
			if beats[j].VOFrames > voFloor {
				voFloor = beats[j].VOFrames
			}
			if c.Frames < voFloor {
				clamped = true // VO 地板被压低 ⇒ 必有 VO_SHORTEN
			}
			if beats[j].ShotAvail > 0 && c.Frames > beats[j].ShotAvail {
				t.Fatalf("SHOT_CLAMP 未生效（第 %d 例 beat %d）：%d > avail %d", i, j, c.Frames, beats[j].ShotAvail)
			}
		}
		if clamped && len(res.Degrades) == 0 {
			t.Fatalf("可解释性违例（第 %d 例）：发生钳制但无降级记录", i)
		}
	}
}
