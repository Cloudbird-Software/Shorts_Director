# src/contracts —— 契约常量（TS 侧）

## 职责

schema/ 单一真源在 TypeScript 侧的锚点：schema 版本串、契约版本号、词表清单、
冻结枚举（beat_role）、画布与渲染工艺参数。是"渲染层/schema 工具"的第一个模块。

## 不变量

- SCHEMA_VERSIONS / VOCAB_V1 / beatRole 与 schema/ 下文件一一对应，
  tests/contracts.test.ts 用 fs 直读 schema/ 目录校验漂移
- 常量只读（as const），禁止运行时变更

## 禁止

- 引入任何外部依赖（本模块是零依赖叶子，被所有后续模块依赖）
- 在这里写业务逻辑

## 验证

make check（其中 arch 会校验：其他模块只能 import 本模块的 index.ts）

## 演进

codegen（make gen）就位后由生成代码替换，调用方经模块入口无感切换。
