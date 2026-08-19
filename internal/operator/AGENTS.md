# operator —— C2 算子契约执行器

schema/contracts/operator/*.json 的 Go 侧实现：控制面 ↔ Python 算子。
算子是纯函数 CLI（stdin/stdout JSON + 文件路径），不知道数据库/租户/业务。

## 调用协议

`operator <op> --contract-version 1 < request.json > response.json`

- 业务四态（OK/INPUT_ERROR/RUNTIME_ERROR/TIMEOUT）经 Response.Status
  表达，只有系统级故障（进程崩溃、垃圾输出、超时）才返回 Go error。
- 算子非零退出但 stdout 有合法结构化响应 ⇒ 按 Response 返回（已履约）。

## 三实现

- LocalRunner：exec 本地算子（开发）。
- DockerRunner：每算子独立镜像，`--network none`（算子不许联网），
  workdir 挂载进容器。命令构造独立成 dockerArgs 便于表驱动测试。
- FakeRunner：按"op+inputs+params+determinism"的 JCS 摘要查
  testdata/golden/<op>/<digest>.json；workdir 不参与键（不影响输出）。

## 不变量

- inputs 媒体一律绝对路径，禁止 URL。
- INPUT_ERROR 必须带可执行的 error.message（外包返修指令的上游）；
  OK 不得携带 error。
- model_versions 必须回填（provenance A2 的上游）。
- golden fixtures 是算子作者的交付义务——缺 fixture 报错指路，不静默。

## 禁止

- Runner 内做业务判断/重试策略——那是编排层（qc/planner）的事。
- 算子清单（12 算子）在这里注册具体实现；本包只管协议。

## 验证

`make go-check`（gofmt + go vet + go test ./internal/operator/...）

## G7 静态检查（golden 字段清单）

- 提交的 golden fixture 在 `testdata/golden/<op>/<GoldenKey>.json`（键 =
  影响输出请求字段的 JCS 摘要）；同 op 多 fixture 的 outputs 字段集合必须一致。
- `golden_fields_test.go`：清单生成 + AST 扫描 internal/ 非测试源码的
  `Outputs["字面量"]` 访问 ⊆ 清单；消费包以 `ConsumedGoldenOps` 声明锚点
  （现仅 qc 包），未声明包出现访问即失败；负例（fixture 删字段 / 越界访问）
  必须失败。fixture 经 FakeRunner 与真实消费路径（qc 桥接器）命中——防腐烂。
