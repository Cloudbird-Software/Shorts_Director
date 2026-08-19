// Package planner 实现 S5 PlannerService 阶段 B（PlanDay 日度组装）的
// 确定性核心。Phase 1 范围（Engineering_plan §Phase 1）：
// 先不做月度优化（阶段 A），每天按当日 schema + slot 分配贪心组装。
// 本包是纯函数：同输入同输出，禁读时钟/随机源/网络（internal/AGENTS.md 硬约束 1）。
package planner

import (
	"errors"
	"fmt"
	"sort"
)

// CraftParams 是工艺参数的唯一来源（Engineering_plan §S5：
// "这类工艺参数应集中在一个可调可测的地方"）。写死上限，超出即不自然。
type CraftParams struct {
	MaxSpeed            float64 // 播放加速上限（1.15；schema Clip.speed maximum 同源）
	SnapToleranceFrames int     // 切点吸附 beat_grid 的容差（±3 帧）
}

// DefaultCraftParams 返回冻结的默认工艺参数。
func DefaultCraftParams() CraftParams {
	return CraftParams{MaxSpeed: 1.15, SnapToleranceFrames: 3}
}

// BeatWindow 是时长求解的输入：单个 beat 的全部约束窗口。
// 三类约束按优先级合取：VO 地板 ≥ beat 地板 ≥ 0，全部被 max 封顶；
// shot 可用量再叠加物理上限（图形卡兜底不受限）。
type BeatWindow struct {
	Role      string // vocab/beat_role（7 值冻结表）
	MinFrames int    // duration_range.min
	MaxFrames int    // duration_range.max
	VOFrames  int    // 该 beat VO 实际帧数；0 = 无旁白（B-roll 自由填充）
	ShotAvail int    // 分配 shot 的可用帧数（含 handles）；≤0 = 图形卡，不限
}

// ClipTiming 是时长求解的输出：单个 beat 的时间片。
type ClipTiming struct {
	BeatIndex int
	Frames    int // timeline 帧数（整数帧，禁 float seconds）
	TlStart   int // 在时间线上的起始帧（由求解器排布，beat 有序）
	TlEnd     int // == TlStart + Frames；IV-VP-1：max(TlEnd) == 总时长
	Speed     float64
}

// DegradeRecord 是降级动作的可解释性记录，
// 直接映射 constraints_report.fallbacks_used（"为什么今天这条这么烂"的唯一途径）。
type DegradeRecord struct {
	BeatIndex int    `json:"beat_index"`
	Kind      string `json:"kind"` // VO_SHORTEN | SHOT_CLAMP
	Note      string `json:"note"`
}

// TimingResult 是一次时长求解的完整产物。
type TimingResult struct {
	Clips    []ClipTiming
	Total    int // Σ Frames；IV-VP-1 帧数守恒的权威值
	Degrades []DegradeRecord
}

// ErrNeedsRepatch 表示分配的 shot 容量低于 VO 地板——
// 时长无解，必须回到 slotquery.Resolve 换候选（"换更长 shot"降级路径）。
var ErrNeedsRepatch = errors.New("planner: shot 容量不足，需 repatch 该 slot")

// TotalRange 是 BeatSchema.total_duration_range 的运行期形态。
type TotalRange struct {
	Min int
	Max int
}

// SolveTiming 把 beat 时长约束 + VO 实际时长 + shot 可用量解成具体帧数。
//
// 算法（Engineering_plan §S5 阶段 B 第 5 步，LP 的小规模替代）：
//  1. 地板：floor_i = max(min_i, vo_i)，再被 max_i 封顶；
//     vo_i > max_i ⇒ 钳到 max_i 并记 VO_SHORTEN（文案层重新生成的信号）
//  2. 总量区间（total）：Σfloors < total.Min ⇒ B-roll 注水补齐到 total.Min；
//     Σfloors > total.Max ⇒ 先收 B-roll 富余（不低于 VO 地板），仍超 ⇒ 按
//     比例削 VO（VO_SHORTEN）；帧数守恒：Σ frames == 落点总量恒成立
//  3. shot 物理上限：frames_i > avail_i ⇒ 钳到 avail_i 记 SHOT_CLAMP；
//     avail_i < VO 地板 ⇒ 返回 ErrNeedsRepatch（换 shot，不硬凑）
func SolveTiming(beats []BeatWindow, total TotalRange, p CraftParams) (*TimingResult, error) {
	if len(beats) == 0 {
		return nil, errors.New("planner: beats 为空")
	}
	if total.Min < 1 || total.Max < total.Min {
		return nil, fmt.Errorf("planner: total 区间非法 [%d,%d]", total.Min, total.Max)
	}
	if p.MaxSpeed < 1 {
		return nil, fmt.Errorf("planner: MaxSpeed 必须 ≥1（得到 %v）", p.MaxSpeed)
	}
	floors := make([]int, len(beats))
	voFloors := make([]int, len(beats)) // VO 地板（收缩模式下的硬下界）
	degrades := []DegradeRecord{}
	sumFloors, sumMin, sumMax := 0, 0, 0
	for i, b := range beats {
		switch {
		case b.MinFrames < 1:
			return nil, fmt.Errorf("planner: beats[%d].MinFrames 必须 ≥1", i)
		case b.MaxFrames < b.MinFrames:
			return nil, fmt.Errorf("planner: beats[%d].MaxFrames(%d) < MinFrames(%d)", i, b.MaxFrames, b.MinFrames)
		case b.VOFrames < 0:
			return nil, fmt.Errorf("planner: beats[%d].VOFrames 不得为负", i)
		}
		floor := maxInt(b.MinFrames, b.VOFrames)
		if b.VOFrames > b.MaxFrames { // VO 超出 beat 上限：钳位 + 降级记录
			degrades = append(degrades, DegradeRecord{
				BeatIndex: i, Kind: "VO_SHORTEN",
				Note: fmt.Sprintf("vo=%d 超出 beat 上限=%d，钳位；文案层应重新生成更短文案", b.VOFrames, b.MaxFrames),
			})
			floor = b.MaxFrames
		}
		floors[i] = floor
		voFloors[i] = minInt(b.VOFrames, b.MaxFrames)
		sumFloors += floor
		sumMin += b.MinFrames
		sumMax += b.MaxFrames
	}
	if total.Min > sumMax {
		return nil, fmt.Errorf("planner: total.Min(%d) > ΣMax(%d)——schema 不可满足", total.Min, sumMax)
	}
	if sumMin > total.Max { // IV-BS-2：静态可满足性检查的运行期防线
		return nil, fmt.Errorf("planner: Σmin(%d) > total.Max(%d)——违反 IV-BS-2", sumMin, total.Max)
	}

	// 总量落点：[Σfloors 与 total.Min 取大] 再被 total.Max 封顶。
	target := maxInt(sumFloors, total.Min)
	if target > total.Max {
		target = total.Max
	}
	frames := fillTo(floors, caps(beats), target)
	// Σfloors 超 total.Max ⇒ 削 VO（比例 + 整数守恒，文案层重生成信号）。
	if sumFrames(frames) > target {
		frames = trimVO(frames, target, &degrades)
	}

	clips := make([]ClipTiming, len(beats))
	cursor := 0
	for i, b := range beats {
		d := frames[i]
		if b.ShotAvail > 0 && d > b.ShotAvail { // 图形卡（≤0）不受限
			// shot 装不下（被 max 钳位后的）VO ⇒ 换 shot，不硬凑。
			voNeed := minInt(b.VOFrames, b.MaxFrames)
			if b.VOFrames > 0 && b.ShotAvail < voNeed {
				return nil, fmt.Errorf("%w: beats[%d] avail=%d < vo=%d", ErrNeedsRepatch, i, b.ShotAvail, b.VOFrames)
			}
			degrades = append(degrades, DegradeRecord{
				BeatIndex: i, Kind: "SHOT_CLAMP",
				Note: fmt.Sprintf("分配=%d 钳到 shot 可用=%d", d, b.ShotAvail),
			})
			d = b.ShotAvail
		}
		clips[i] = ClipTiming{BeatIndex: i, Frames: d, TlStart: cursor, TlEnd: cursor + d, Speed: 1.0}
		cursor += d
	}
	return &TimingResult{Clips: clips, Total: cursor, Degrades: degrades}, nil
}

// fillTo 把 cur 注水到 Σ == target：向低于 upper 的 beat 按余量比例注入
// （B-roll 补齐）；逐轮钳位，整数误差逐帧落到最小下标的有余量 beat。
// 迭代按 beat 下标有序——禁 map（迭代序不确定）。只注入不收缩：
// floor 已是每 beat 硬下限（min 与 VO 取大），总量超限走 trimVO 降级。
func fillTo(cur, upper []int, target int) []int {
	out := make([]int, len(cur))
	copy(out, cur)
	for sumFrames(out) < target {
		sum := sumFrames(out)
		room := orderedIdx(out, func(i, v int) bool { return v < upper[i] })
		if len(room) == 0 {
			return out // 全部到 cap 仍不够：按现状收（target 不可达的防御路径）
		}
		totalRoom := 0
		for _, i := range room {
			totalRoom += upper[i] - out[i]
		}
		gave := 0
		for _, i := range room {
			share := (target - sum) * (upper[i] - out[i]) / totalRoom
			out[i] += share
			gave += share
		}
		if gave == 0 {
			out[room[0]]++
		}
	}
	return out
}

// trimVO 在 Σfloors 超 total.Max 时按帧数比例削（可低于 VO 地板与 beat min
// ——对应降级路径"缩短 VO / 加速播报"，播报加速上限 1.15x 由 CraftParams
// 钉死），逐条记 VO_SHORTEN（文案层重新生成的信号）。
// 每个 beat 至少保留 1 帧（schema：tl_end > tl_start）。
func trimVO(frames []int, target int, degrades *[]DegradeRecord) []int {
	out := make([]int, len(frames))
	copy(out, frames)
	for sumFrames(out) > target {
		over := orderedIdx(out, func(_, v int) bool { return v > 1 })
		if len(over) == 0 {
			return out // 全部到 1 帧仍超——防御性退出
		}
		sum := sumFrames(out)
		need := sum - target
		base, rem := need/len(over), need%len(over)
		progressed := false
		for j, i := range over {
			cut := base
			if j < rem {
				cut++ // 余数分给最小下标，保证每轮至少削 1 帧
			}
			if cut > out[i]-1 {
				cut = out[i] - 1
			}
			if cut <= 0 {
				continue
			}
			out[i] -= cut
			progressed = true
			*degrades = append(*degrades, DegradeRecord{
				BeatIndex: i, Kind: "VO_SHORTEN",
				Note: fmt.Sprintf("总量超限，削 %d 帧（VO 加速播报/文案重生成）", cut),
			})
		}
		if !progressed {
			return out // 防御：不再有可削余量
		}
	}
	return out
}

// orderedIdx 返回满足 cond(i, v) 的升序下标切片。
func orderedIdx(vals []int, cond func(i, v int) bool) []int {
	idx := make([]int, 0, len(vals))
	for i, v := range vals {
		if cond(i, v) {
			idx = append(idx, i)
		}
	}
	return idx
}

func sumFrames(v []int) int {
	s := 0
	for _, x := range v {
		s += x
	}
	return s
}

func caps(beats []BeatWindow) []int {
	c := make([]int, len(beats))
	for i, b := range beats {
		c[i] = b.MaxFrames
	}
	return c
}

// SnapToBeats 把内部切点吸附到最近的 beat_grid 帧号（容差 tol 内），
// 首尾切点不动（总时长不受影响），吸附后仍保持严格递增。
// Engineering_plan §S5 阶段 B 第 6 步。
func SnapToBeats(cuts []int, beatGrid []int, tol int) []int {
	if len(cuts) == 0 || len(beatGrid) == 0 || tol <= 0 {
		return cuts
	}
	grid := append([]int(nil), beatGrid...)
	sort.Ints(grid)
	out := make([]int, len(cuts))
	copy(out, cuts)
	nearest := func(v int) (int, bool) {
		idx := sort.SearchInts(grid, v)
		best, bestDist := -1, tol+1
		for _, g := range []int{idx - 1, idx} {
			if g < 0 || g >= len(grid) {
				continue
			}
			if d := absInt(grid[g] - v); d < bestDist {
				best, bestDist = grid[g], d
			}
		}
		return best, bestDist <= tol
	}
	for i := 1; i < len(out)-1; i++ { // 首尾不动
		if g, ok := nearest(out[i]); ok {
			prev := out[i-1]
			next := out[i+1]
			if g > prev && (i == len(out)-2 || g < next) {
				out[i] = g
			}
		}
	}
	return out
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
