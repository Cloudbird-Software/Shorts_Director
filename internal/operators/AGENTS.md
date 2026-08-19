# operators —— C2 算子服务端骨架与纯 Go 算子

C2 协议的服务侧：`shorts-operator <op> --contract-version 1`。
纯 Go 可实现的算子在此注册（当前：probe 走 ffprobe）；Python 重模型
算子独立成镜像，与本入口共用同一契约。

## 边界

- 算子是无状态纯函数：Request → Response，不知道数据库/租户/业务。
- Serve 是唯一出口：handler 系统错误折叠为 RUNTIME_ERROR，
  CLI 永远输出合法 Response（违反契约的产物也被兜底）。
- probe 的 INPUT_ERROR 语义：URL/相对路径、文件不可读、非媒体、
  无视频流——都可由上游换素材/重传修复。

## probe 算子

- inputs.media_path 必须绝对路径（禁止 URL——算子不许联网）。
- outputs：width/height/fps/duration_sec/vcodec/aspect_ratio/has_audio
  （有音轨时 acodec；nb_frames 可得时）。
- model_versions 回填 ffprobe 版本指纹（provenance A2）。

## 禁止

- 算子内读环境做分支决策（版本指纹除外）。
- 把业务阈值硬编码进算子——params 传入（工艺参数集中管理）。

## 验证

`make go-check`；端到端用例依赖 ffmpeg/ffprobe（缺失自动 skip）。
