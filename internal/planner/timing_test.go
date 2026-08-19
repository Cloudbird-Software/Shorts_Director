package planner

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestSolveTimingVOAsFloor(t *testing.T) {
	// VO 比 min 长 ⇒ VO 就是地板；无 VO 的 beat ⇒ 补到 min。
	beats := []BeatWindow{
		{Role: "HOOK", MinFrames: 20, MaxFrames: 60, VOFrames: 45},
		{Role: "CTA", MinFrames: 15, MaxFrames: 40, VOFrames: 0},
	}
	total := TotalRange{Min: 60, Max: 100} // Σfloors=60 恰为下限
	res, err := SolveTiming(beats, total, DefaultCraftParams())
	if err != nil {
		t.Fatalf("SolveTiming: %v", err)
	}
	if res.Total != 60 {
		t.Errorf("Total = %d, want 60", res.Total)
	}
	if res.Clips[0].Frames < 45 || res.Clips[1].Frames < 15 {
		t.Errorf("VO/min 地板被破坏: %v", res.Clips)
	}
	// 时间线连续且守恒（IV-VP-1 前置）。
	if res.Clips[0].TlStart != 0 || res.Clips[1].TlStart != res.Clips[0].TlEnd ||
		res.Clips[len(res.Clips)-1].TlEnd != res.Total {
		t.Errorf("时间线不连续: %+v", res.Clips)
	}
}

func TestSolveTimingBrollWaterFill(t *testing.T) {
	// Σfloors < total.Min ⇒ B-roll 注水补齐，帧数守恒且不越 max。
	beats := []BeatWindow{
		{Role: "HOOK", MinFrames: 30, MaxFrames: 100, VOFrames: 30},
		{Role: "PROOF", MinFrames: 30, MaxFrames: 100, VOFrames: 30},
		{Role: "CTA", MinFrames: 20, MaxFrames: 60, VOFrames: 20},
	}
	total := TotalRange{Min: 240, Max: 260} // Σfloors=80，补到 240
	res, err := SolveTiming(beats, total, DefaultCraftParams())
	if err != nil {
		t.Fatalf("SolveTiming: %v", err)
	}
	if res.Total != 240 {
		t.Errorf("Total = %d, want 240（total.Min）", res.Total)
	}
	sum := 0
	for i, c := range res.Clips {
		if c.Frames < beats[i].MinFrames || c.Frames > beats[i].MaxFrames {
			t.Errorf("clip[%d]=%d 越界 [%d,%d]", i, c.Frames, beats[i].MinFrames, beats[i].MaxFrames)
		}
		sum += c.Frames
	}
	if sum != res.Total {
		t.Errorf("Σ=%d ≠ Total=%d（帧数守恒破坏）", sum, res.Total)
	}
}

func TestSolveTimingShrinkToTotalMax(t *testing.T) {
	// Σfloors > total.Max ⇒ 先收 B-roll 富余（VO 地板不动），总量钉在 max。
	beats := []BeatWindow{
		{Role: "HOOK", MinFrames: 20, MaxFrames: 90, VOFrames: 80},
		{Role: "PROOF", MinFrames: 20, MaxFrames: 90, VOFrames: 30},
		{Role: "CTA", MinFrames: 20, MaxFrames: 90, VOFrames: 20},
	}
	// floors=[80,30,20] Σ=130 > max=100：floors 无注水富余可收 ⇒ trimVO 削 VO。
	total := TotalRange{Min: 60, Max: 100}
	res, err := SolveTiming(beats, total, DefaultCraftParams())
	if err != nil {
		t.Fatalf("SolveTiming: %v", err)
	}
	if res.Total != 100 {
		t.Errorf("Total = %d, want 100（total.Max）", res.Total)
	}
	found := false
	for _, d := range res.Degrades {
		if d.Kind == "VO_SHORTEN" {
			found = true
		}
	}
	if !found {
		t.Errorf("VO 被削但未记录: %+v", res.Degrades)
	}
}

func TestSolveTimingIVBS2Rejected(t *testing.T) {
	// Σmin > total.Max ⇒ schema 静态可满足性违反（IV-BS-2），直接拒绝。
	beats := []BeatWindow{
		{Role: "HOOK", MinFrames: 40, MaxFrames: 90, VOFrames: 20},
		{Role: "PROOF", MinFrames: 40, MaxFrames: 90, VOFrames: 20},
	}
	_, err := SolveTiming(beats, TotalRange{Min: 30, Max: 50}, DefaultCraftParams())
	if err == nil || !strings.Contains(err.Error(), "IV-BS-2") {
		t.Errorf("应拒绝 IV-BS-2 违反: %v", err)
	}
}

func TestSolveTimingVOShortenBeatCap(t *testing.T) {
	beats := []BeatWindow{
		{Role: "HOOK", MinFrames: 20, MaxFrames: 40, VOFrames: 55}, // VO 超单 beat 上限
		{Role: "CTA", MinFrames: 20, MaxFrames: 40, VOFrames: 20},
	}
	res, err := SolveTiming(beats, TotalRange{Min: 40, Max: 80}, DefaultCraftParams())
	if err != nil {
		t.Fatalf("SolveTiming: %v", err)
	}
	if res.Clips[0].Frames > 40 {
		t.Errorf("VO_SHORTEN 未钳位: %d", res.Clips[0].Frames)
	}
	found := false
	for _, d := range res.Degrades {
		if d.Kind == "VO_SHORTEN" && d.BeatIndex == 0 {
			found = true
		}
	}
	if !found {
		t.Errorf("未记录 VO_SHORTEN: %+v", res.Degrades)
	}
}

func TestSolveTimingShotClampAndRepatch(t *testing.T) {
	// SHOT_CLAMP：注水分配超过 shot 可用 ⇒ 钳位 + 记录（VO 装得下）。
	beats := []BeatWindow{
		{Role: "HOOK", MinFrames: 20, MaxFrames: 90, VOFrames: 20, ShotAvail: 60},
		{Role: "CTA", MinFrames: 20, MaxFrames: 40, VOFrames: 0, ShotAvail: 0}, // 图形卡不限
	}
	// floors=[20,20]；total=130 ⇒ 注水把 HOOK 推过 avail=60 ⇒ SHOT_CLAMP。
	total := TotalRange{Min: 130, Max: 130}
	res, err := SolveTiming(beats, total, DefaultCraftParams())
	if err != nil {
		t.Fatalf("SolveTiming: %v", err)
	}
	if res.Clips[0].Frames != 60 {
		t.Errorf("SHOT_CLAMP 后应=60, got %d", res.Clips[0].Frames)
	}
	clamped := false
	for _, d := range res.Degrades {
		if d.Kind == "SHOT_CLAMP" && d.BeatIndex == 0 {
			clamped = true
		}
	}
	if !clamped {
		t.Errorf("SHOT_CLAMP 未记录: %+v", res.Degrades)
	}

	// repatch：shot 连 VO 都装不下 ⇒ ErrNeedsRepatch。
	beats[0].ShotAvail = 15 // < vo=20
	if _, err := SolveTiming(beats, total, DefaultCraftParams()); !errors.Is(err, ErrNeedsRepatch) {
		t.Errorf("应返回 ErrNeedsRepatch, got %v", err)
	}
}

func TestSolveTimingValidation(t *testing.T) {
	ok := []BeatWindow{{Role: "HOOK", MinFrames: 10, MaxFrames: 20, VOFrames: 5}}
	cases := []struct {
		name  string
		beats []BeatWindow
		total TotalRange
	}{
		{"空 beats", nil, TotalRange{Min: 1, Max: 2}},
		{"min<1", []BeatWindow{{MinFrames: 0, MaxFrames: 10}}, TotalRange{Min: 1, Max: 2}},
		{"max<min", []BeatWindow{{MinFrames: 20, MaxFrames: 10}}, TotalRange{Min: 1, Max: 2}},
		{"vo 为负", []BeatWindow{{MinFrames: 1, MaxFrames: 10, VOFrames: -1}}, TotalRange{Min: 1, Max: 2}},
		{"total 区间非法", ok, TotalRange{Min: 5, Max: 3}},
		{"total.Min>ΣMax", ok, TotalRange{Min: 100, Max: 200}},
		{"Σmin>total.Max（IV-BS-2）", []BeatWindow{{MinFrames: 50, MaxFrames: 60}}, TotalRange{Min: 1, Max: 40}},
	}
	for _, c := range cases {
		if _, err := SolveTiming(c.beats, c.total, DefaultCraftParams()); err == nil {
			t.Errorf("%s: 应报错", c.name)
		}
	}
	if _, err := SolveTiming(ok, TotalRange{Min: 1, Max: 2}, CraftParams{MaxSpeed: 0.5}); err == nil {
		t.Error("MaxSpeed<1 应报错")
	}
}

func TestSolveTimingDeterministic(t *testing.T) {
	beats := []BeatWindow{
		{Role: "HOOK", MinFrames: 25, MaxFrames: 80, VOFrames: 33},
		{Role: "CONTEXT", MinFrames: 25, MaxFrames: 80, VOFrames: 27},
		{Role: "CTA", MinFrames: 20, MaxFrames: 60, VOFrames: 29},
	}
	total := TotalRange{Min: 120, Max: 180}
	first, err := SolveTiming(beats, total, DefaultCraftParams())
	if err != nil {
		t.Fatalf("SolveTiming: %v", err)
	}
	for i := 0; i < 20; i++ {
		again, err := SolveTiming(beats, total, DefaultCraftParams())
		if err != nil {
			t.Fatalf("round %d: %v", i, err)
		}
		if !reflect.DeepEqual(first, again) {
			t.Fatalf("round %d 结果漂移: %+v vs %+v", i, first, again)
		}
	}
}

func TestSnapToBeats(t *testing.T) {
	grid := []int{0, 24, 48, 72, 96}
	cases := []struct {
		name string
		cuts []int
		tol  int
		want []int
	}{
		{"容差内吸附", []int{0, 25, 50, 120}, 3, []int{0, 24, 48, 120}}, // 首尾不动
		{"容差外不动", []int{0, 30, 60, 120}, 3, []int{0, 30, 60, 120}},
		{"吸附撞后继则放弃", []int{0, 23, 24, 120}, 3, []int{0, 23, 24, 120}},
		{"tol=0 原样返回", []int{0, 25, 120}, 0, []int{0, 25, 120}},
	}
	for _, c := range cases {
		got := SnapToBeats(c.cuts, grid, c.tol)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
	if SnapToBeats([]int{0, 50, 100}, nil, 3) == nil {
		t.Error("空 grid 应原样返回")
	}
}

func TestSnapToBeatsUnsortedGrid(t *testing.T) {
	got := SnapToBeats([]int{0, 49, 100}, []int{96, 48, 0, 24, 72}, 3)
	if !reflect.DeepEqual(got, []int{0, 48, 100}) {
		t.Errorf("乱序 grid: got %v", got)
	}
}
