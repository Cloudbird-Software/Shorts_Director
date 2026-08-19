/** dependency-cruiser 配置 —— 架构边界守卫
 *
 * 作用：防止模块边界腐化（"写得散"）。规则随项目演进填充，
 * 骨架保证机制在每个新仓开箱即用。
 *
 * 新仓第一个任务：让 AI 根据实际模块结构把 TODO 规则补全。
 * 文档: https://github.com/sverweij/dependency-cruiser
 */
module.exports = {
  forbidden: [
    {
      name: "no-circular",
      severity: "error",
      comment: "循环依赖 = 边界失败，必须拆模块",
      from: {},
      to: { circular: true },
    },
    {
      name: "no-orphans",
      severity: "warn",
      comment: "孤立文件通常是残留死代码",
      from: { orphan: true, pathNot: ["^src/index\\.ts$"] },
      to: {},
    },
    // 边界规则随模块地图演进（docs/ARCHITECTURE.md 仓库布局）。
    // 当前模块：src/contracts（契约常量，零依赖叶子）。
    {
      name: "entry-only-imports",
      severity: "error",
      comment: "跨模块只能 import 入口文件（index.ts），不得深入实现",
      from: { pathNot: ["^src/([^/]+)/index\\.ts$", "^src/index\\.ts$"] },
      to: { path: "^src/([^/]+)/(.+)$", pathNot: ["^src/$1/index\\.ts$"] },
    },
    {
      name: "contracts-is-leaf",
      severity: "error",
      comment: "契约常量模块是零依赖叶子：不得 import src 内其他模块",
      from: { path: "^src/contracts/" },
      to: { path: "^src/", pathNot: ["^src/contracts/"] },
    },
  ],
  options: {
    doNotFollow: { path: "node_modules" },
    tsConfig: { fileName: "tsconfig.json" },
    reporterOptions: { dot: { theme: { graph: { rankdir: "TB" } } } },
  },
};
