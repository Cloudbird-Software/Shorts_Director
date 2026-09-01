# doctor —— 环境探测与 capability profile（IR-0007 AC-1，E0 实验）

## 职责

探测实验机环境（NVIDIA GPU 算力版本/显存/驱动/CUDA、ffmpeg、docker），
对候选生成模型清单逐项给出 feasible|infeasible 判定与原因，产出内容寻址
的 capability profile（IFACE-2）。CLI 入口：cmd/shorts-doctor（make doctor）。

## 不变量

- 判定只基于静态硬件门槛（必要条件筛查）；最终可行性由 E1 单发冒烟裁决
  （DECISION-3）——本包不预置模型排名，feasible ≠ 可用。
- 缺失项（无 GPU / 无 ffmpeg / 无 docker）必须显式 infeasible|note，
  禁止静默降级（与 renderer ErrFFmpegMissing 同语义）。
- profile 内容寻址：digest 复用 internal/digest（RFC 8785 JCS + sha256），
  Digest 字段自引用排除；落盘文件名 = digest hex（可回查）。
- 解析函数纯函数化（nvidia-smi/ffmpeg/docker 输出 → 结构化字段），
  fixture 单测覆盖；命令执行经 Runner 接口注入（可测）。

## 禁止

- 在本包做模型下载、推理、冒烟执行（那是算子与 E1 的事）
- 候选清单外散落硬编码模型名（清单变更 = 评审焦点，进 candidates.go）
- 引入非标准库依赖

## 验证

`make go-check`（解析 fixture 单测 + fake Runner golden + 无 GPU 降级路径

- 摘要复算）。
