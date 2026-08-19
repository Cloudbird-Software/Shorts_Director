# internal/digest —— RFC 8785 (JCS) 内容寻址摘要

## 职责

- `CanonicalizeJSON`：RFC 8785 规范化（ES 数值序列化 / UTF-16 key 排序 / 最小转义 / 无空白）
- `ContentDigest`：规范化后 sha256，输出 `sha256:<64 hex>`（匹配 schema content_hash pattern）

## 不变量

- 确定性纯函数：同输入（语义等价 JSON）必同输出；禁止任何环境依赖。
- 跨语言逐字节一致：TS/Python 侧若实现 JCS，必须与本包对拍同一组向量。
- NaN/Infinity 非法；数值语义 = IEEE 754 double（JCS 规定，与 ES 一致）。

## 禁止

- 在此包引入任何非标准库依赖。
- 在规范化前做"美化"或字段过滤——内容寻址针对原始语义内容。

## 验证

`go test ./internal/digest/`（含 RFC 8785 Appendix B 数值向量、
UTF-16 代理对排序反向案例、摘要稳定性）。

## G3 共享向量集

`testdata/digest/jcs_vectors.json`（RFC 8785 锚点 + 仓库真实形态）由 Go
（internal/digest/vectors_test.go）与 TS（tests/digest.test.ts → src/digest）
双侧消费，canonical 与 sha256 逐字节对照——任何一侧实现漂移，双侧测试
同时失败。TS 侧实现见 src/digest/jcs.ts（零依赖，String(n) 即 ES 规范数字、
默认 sort 即 UTF-16 码元序）。
