# planner —— S5 PlannerService（阶段 B：PlanDay 日度组装）

负责把"当日 BeatSchema + slot 取材结果 + VO 时间戳"解成 VideoPlan IR 的
确定性时间骨架。Phase 1 范围：不做月度优化（阶段 A），每天贪心组装。

## 不变量

- 纯函数：同输入同输出。禁读时钟、随机源、网络（internal/AGENTS.md 硬约束 1）。
- 迭代一律按 beat 下标有序——map 迭代序不确定，出现即缺陷。
- 帧数守恒（IV-VP-1 前置）：TimingResult.Total == Σ Clips[i].Frames，
  由 waterFill 的整数注水保证，不是近似。
- 时间一律整数帧；播放加速上限 1.15 只经 CraftParams.MaxSpeed 暴露。

## 降级语义（constraints_report.fallbacks_used 的来源）

- VO_SHORTEN：VO 超 beat 上限，钳位并要求文案层重新生成。
- SHOT_CLAMP：分配帧数超 shot 可用量，钳到可用量。
- ErrNeedsRepatch：shot 连 VO 都装不下——回到 slotquery.Resolve 换候选，
  不在本包硬凑。

## 禁止

- 引入 LLM/TTS/网络调用（那些在 BAML 层与算子边界）。
- float seconds 进入任何结构体。
- 手调 CraftParams 之外的工艺参数（魔法数）。

## 验证

`make go-check`（gofmt + go vet + go test ./internal/planner/...）
