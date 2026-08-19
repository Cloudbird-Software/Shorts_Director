package planner

import (
	"fmt"
	"math/rand"
	"testing"
)

// Freeze Gate G9（属性 ×1000）：任意可行输入下 SolveTiming 的帧数守恒律
// （IV-VP-1：Σclips == Total）与时间线连续性（无缝铺满 [0, Total]）。
// 生成器固定种子（纯函数纪律），只产可行输入：total ∈ [Σmin, Σmax]、
// ShotAvail 充足（避免 ErrNeedsRepatch 分支——该分支由示例测试覆盖）。

func genBeats(r *rand.Rand) ([]BeatWindow, TotalRange) {
	n := 1 + r.Intn(8)
	beats := make([]BeatWindow, n)
	sumMin, sumMax := 0, 0
	for i := range beats {
		mn := 1 + r.Intn(90)
		mx := mn + r.Intn(270)
		vo := r.Intn(121)
		if vo > mx {
			vo = mx // 生成端直接钳位，聚焦守恒律本身
		}
		avail := mx + r.Intn(600) // 充足：> max ⇒ 永不触发 SHOT_CLAMP/repatch
		beats[i] = BeatWindow{
			Role: "HOOK", MinFrames: mn, MaxFrames: mx, VOFrames: vo, ShotAvail: avail,
		}
		sumMin += mn
		sumMax += mx
	}
	tMin := sumMin + r.Intn(sumMax-sumMin+1)
	tMax := tMin + r.Intn(sumMax-tMin+1)
	return beats, TotalRange{Min: tMin, Max: tMax}
}

// TestG9FrameConservation（属性 ×1000）：
//   - Σ Clips.Frames == Total（帧数守恒，IV-VP-1）
//   - 时间线连续：TlStart[0]=0，TlEnd[i]==TlStart[i+1]，TlEnd[last]==Total
//   - Frames > 0；Speed ∈ [1, MaxSpeed]
func TestG9FrameConservation(t *testing.T) {
	r := rand.New(rand.NewSource(2026)) //nolint:gosec — 确定性种子
	p := DefaultCraftParams()
	for run := 0; run < 1000; run++ {
		beats, total := genBeats(r)
		res, err := SolveTiming(beats, total, p)
		if err != nil {
			t.Fatalf("run %d 可行输入却报错: %v", run, err)
		}
		sum, cursor := 0, 0
		for i, c := range res.Clips {
			if c.Frames <= 0 {
				t.Fatalf("run %d clip[%d] 非正帧数 %d", run, i, c.Frames)
			}
			if c.Speed < 1 || c.Speed > p.MaxSpeed {
				t.Fatalf("run %d clip[%d] speed %v 越界", run, i, c.Speed)
			}
			if c.TlStart != cursor || c.TlEnd != c.TlStart+c.Frames {
				t.Fatalf("run %d clip[%d] 时间线断裂: %+v (期望起点 %d)",
					run, i, c, cursor)
			}
			cursor = c.TlEnd
			sum += c.Frames
		}
		if sum != res.Total || cursor != res.Total {
			t.Fatalf("run %d 守恒破坏: Σframes=%d cursor=%d Total=%d",
				run, sum, cursor, res.Total)
		}
		if res.Total < total.Min || res.Total > total.Max {
			t.Fatalf("run %d Total=%d 越出 schema 区间 [%d,%d]",
				run, res.Total, total.Min, total.Max)
		}
	}
}

// TestG9DeterministicSameInput（属性）：同输入恒同输出（求值器纯函数性）。
func TestG9DeterministicSameInput(t *testing.T) {
	r := rand.New(rand.NewSource(77)) //nolint:gosec
	for run := 0; run < 100; run++ {
		beats, total := genBeats(r)
		a, err := SolveTiming(beats, total, DefaultCraftParams())
		if err != nil {
			t.Fatal(err)
		}
		b, err := SolveTiming(beats, total, DefaultCraftParams())
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprint(*a) != fmt.Sprint(*b) {
			t.Fatalf("run %d 非确定性输出", run)
		}
	}
}
