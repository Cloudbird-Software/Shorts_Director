# Shorts_Director

面向本地生活商家的短视频工业化生产系统：**意图与产物分离、一切非确定性显式化、
约束优先于生成、契约先冻结实现后填充**。完整工程设计见
[docs/Engineering_plan.md](docs/Engineering_plan.md)，架构纪律见
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)。

## 当前阶段：Phase 0 进行中（契约冻结未达成）

按工程设计 §7，本阶段产出只有 **单一真源 schema**（受控词表 + 实体契约 +
服务间契约）与配套测试样本，不写业务代码：

```
schema/                  # 唯一真源，人手写（详见 schema/AGENTS.md）
├── vocab/v1/*.yaml      # 受控词表（14 张，enum 只允许追加）
├── entities/            # 实体 JSON Schema（结构）
├── contracts/           # C2 算子协议 / C3 渲染契约
└── testdata/            # valid / invalid / evolution 样本
```

后续语言栈：控制面 Go（已落地：cmd/ + internal/）、算子 Python（规划中，尚未落地——operators/ 目录未建）、渲染层 TypeScript/Remotion（规划中）。

### Freeze Gate 状态（工程设计 §4.8）

「契约冻结」的准入条件是 12 项 Gate 全部达成。当前 **1 项达成、5 项部分完成、6 项未完成**（2026-08-20 核对，来源 #50），阶段声明以本表为准；每关闭一项跟踪 issue 须同步更新本表。

| Gate | 内容（§4.8 原文摘要）              | 状态                                | 跟踪 issue |
| ---- | ---------------------------------- | ----------------------------------- | ---------- |
| G1   | Schema ≥5 valid + ≥15 invalid 样本 | ✅ 达成（含孤儿样本守卫）           | —          |
| G2   | 跨语言（Go/TS/BAML）校验一致性     | 🟡 部分：双语言各自校验，无对照测试 | #48        |
| G3   | JCS digest 跨语言一致              | 🟡 部分：仅 Go 实现（RFC 8785）     | #48        |
| G4   | 跨字段不变式 IV-* 全实现           | 🟡 部分：BrandKernel 无 Go 实体     | #49        |
| G5   | 向后兼容 evolution 样本            | ❌ 缺失（无 evolution/ 目录）       | #45        |
| G6   | Provider golden ≥5 组真实素材      | 🟡 部分：golden 全为测试内合成 JSON | —          |
| G7   | Consumer 字段访问静态检查          | ❌ 缺失                             | #46        |
| G8   | 未知字段忽略 + 未知 enum 降级      | ❌ 缺失（无降级路径）               | #45        |
| G9   | 属性测试 ≥1000 次无反例            | ❌ 缺失（fast-check 已装未用）      | #44        |
| G10  | ≥3 条变形不变式测试                | ❌ 缺失                             | #44        |
| G11  | 确定性 + 非退化多样性测试          | 🟡 部分：确定性有，多样性无         | —          |
| G12  | 3 种子租户 30 天 E2E               | ❌ 未启动（后置 Phase 1）           | #47        |

Phase 0 DoD 的其余缺口：手写 plan.json 渲出视频（渲染器）→ #43；Go 检查接入 CI → #42。

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
