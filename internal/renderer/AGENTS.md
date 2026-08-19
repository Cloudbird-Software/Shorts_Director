# internal/renderer —— C3 渲染契约执行端（Phase 0 最小路径）

负责什么：把手写 plan.json 经 compiler.Compile 得到的 RenderRequest 渲成
可播放视频（Phase 0 DoD，#43）。帧画面为确定性纯色占位（content_hash
派生色），视频合成经外部 ffmpeg 二进制（os/exec，零新增 Go 依赖）。

## 不变量

- R-1 确定性：纯 Go 帧生成禁止时钟/随机/网络；ffmpeg 全参数 bitexact
  （`-fflags +bitexact`、`-map_metadata -1`），同输入同版本恒同字节
  （`TestRenderDeterministic` 双跑断言 digest 全等）。
- R-2 帧数守恒：先落满 `plan.TotalFrames()` 张帧再合成，数量不符即失败。
- R-4 无隐式回退：缺 ffmpeg 二进制显式报错（ErrFFmpegMissing），绝不
  静默降级；媒体/字体完备性由 compiler.Compile 上游把关。
- R-5 版本含字体哈希：`Version()` = ffmpeg 版本 + 排序后字体哈希清单。

## 禁止什么

- 禁止在帧生成中引入 `time`、`math/rand`、网络调用（三禁令）。
- 禁止读 resolved_media 源文件做"尽力而为"的解码降级——Phase 0 不解码
  源素材，Phase 1 解码失败必须报错回控制面。
- 禁止新增 Go 依赖（ffmpeg 经外部二进制；编码库引入须先按 AGENTS.md
  硬规则 3 报批）。

## 如何验证

- `go test ./internal/renderer/`（无 ffmpeg 环境自动跳过合成段）
- `make render-demo`（完整链路：手写 plan → 视频 → C3 response 摘要）
