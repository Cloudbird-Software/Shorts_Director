# Shorts_Director

面向本地生活商家的短视频工业化生产系统：**意图与产物分离、一切非确定性显式化、
约束优先于生成、契约先冻结实现后填充**。完整工程设计见
[docs/Engineering_plan.md](docs/Engineering_plan.md)，架构纪律见
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)。

## 当前阶段：Phase 0 契约冻结

按工程设计 §7，本阶段产出只有 **单一真源 schema**（受控词表 + 实体契约 +
服务间契约）与配套测试样本，不写业务代码：

```
schema/                  # 唯一真源，人手写（详见 schema/AGENTS.md）
├── vocab/v1/*.yaml      # 受控词表（14 张，enum 只允许追加）
├── entities/            # 实体 JSON Schema（结构）
├── contracts/           # C2 算子协议 / C3 渲染契约
└── testdata/            # valid / invalid / evolution 样本
```

后续语言栈：控制面 Go、算子 Python（无状态 CLI）、渲染层 TypeScript/Remotion。

## Makefile 接口（所有语言统一，CI 只认这个）

| 目标         | 作用                                     |
| ------------ | ---------------------------------------- |
| `make setup` | 安装依赖（`npm ci`）                     |
| `make fmt`   | 格式化                                   |
| `make lint`  | prettier --check + eslint + tsc --noEmit |
| `make test`  | vitest + coverage                        |
| `make build` | 构建                                     |
| `make check` | lint + test，**提交前必须全绿**          |

## CI 结构

- `hygiene`：密钥扫描（gitleaks）、大文件/凭据文件拦截、zizmor Actions 审计
- `check`：`make setup && make check`
- `deps`：依赖漏洞 + 许可证审查（PR 时）
- `gate`：聚合门（组织 ruleset 的唯一必需 check）

工作流实现在 [CI-Workflows](https://github.com/Cloudbird-Software/CI-Workflows)，本仓只引用 `@v1`。
