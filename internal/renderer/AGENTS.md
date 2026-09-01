# internal/renderer —— C3 渲染契约执行端

负责什么：把 plan 经 compiler.Compile 得到的 RenderRequest 渲成可播放
视频（IR-0007 E4 信息层零失败 / E5 端到端形态1 的渲染侧）。

## 渲染路径两级

- **真实解码**（生产，`modes.placeholder_media=false`）：SHOT/GENERATED 轨
  经 ffmpeg 抽帧归一化到画布（fps 对齐 + letterbox），纯 Go 合成时间线帧。
- **占位纯色帧**（测试模式，`modes.placeholder_media=true`）：
  content_hash 派生色——保留给无真实媒体的单元测试。

## 不变量

- R-1 确定性：帧生成禁止时钟/随机/网络；ffmpeg 全参数 bitexact +
  `-map_metadata -1`；AIGC 元数据的 produceTime 取 plan.ScheduledDate
  （非时钟）。同输入同版本恒同字节（双跑 digest 全等测试）。
- R-2 帧数守恒：先落满 `plan.TotalFrames()` 张帧再合成。
- R-3 安全区拒绝：overlay 布局盒（锚点换算后）越出 canvas−safe_area 即报错。
- R-4 无隐式回退：缺 ffmpeg 显式报错；媒体抽帧不足（源短于时间线引用）、
  字体文件不可读/哈希漂移、overlay 缺文本——一律报错，绝不静默降级。
- R-5 版本含字体哈希：`Version()` = ffmpeg 版本 + 排序后字体哈希清单。
- INV-5：确定性信息（文字/价格/电话/AIGC 披露）只经 overlay → drawtext
  （textfile 传参）进入成品；文本由上游用信息表模板渲染，渲染器不做拼接。
- AIGC 双轨：显式 aigc.disclosure overlay（drawtext 角标）+ 容器隐式
  元数据（compliance.BuildImplicitLabel，需 `use_metadata_tags`）。

## 禁止什么

- 禁止在帧生成/信息层引入 `time`、`math/rand`、网络调用。
- 禁止把信息层文本交给生成域（gen 算子）产生。
- 禁止新增 Go 依赖（解码/编码经外部 ffmpeg 二进制）。

## 如何验证

`go test ./internal/renderer/`（缺 ffmpeg/ffprobe/DejaVu 字体自动 skip）；
`make render-demo`（手写 plan → 视频 → C3 response 摘要）。
