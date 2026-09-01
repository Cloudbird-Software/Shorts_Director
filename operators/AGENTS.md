# operators/ —— Python 无状态生成/评审算子（C2）

每算子一个子目录、独立镜像（nvidia runtime）、只走 C2 契约
（schema/contracts/operator/*.json）。Go 控制面经
internal/operator 的 Runner（Local/Docker/Fake）调用。

## 边界（继承 docs/ARCHITECTURE.md L2）

- 算子是无状态纯函数：stdin JSON → stdout JSON，不联网、不碰数据库/租户。
- 四态响应：OK / INPUT_ERROR（坏输入，可重传修复）/ RUNTIME_ERROR / TIMEOUT；
  INPUT_ERROR 必须带人话可执行的 error.message。
- 同输入（含 determinism.seed）同输出——算子作者交付义务包括
  testdata/golden/<op>/ fixtures（Go FakeRunner 按请求摘要查表）。
- 模型后端可插拔：注册表换模型不改契约；确定性信息（价格/电话/Logo）
  禁止交进生成域（INV-5）。

## gen_i2v

- inputs：image_path（绝对路径）/ prompt / duration_sec / fps；
  params：model / width / height / num_inference_steps。
- outputs：video_path / content_hash（产物 sha256）。
- 真实后端必须 determinism.seed；非确定性来源显式记入
  model_versions.determinism（AC-3 重放条款）。
- fake 后端零第三方依赖（ffmpeg lavfi），供无 GPU 管道联调。

## 验证

`make go-check`（golden 查表 + LocalRunner 端到端用 run.sh，缺 python3/ffmpeg
自动 skip）；镜像构建在 V100 实测时进行（E1）。
