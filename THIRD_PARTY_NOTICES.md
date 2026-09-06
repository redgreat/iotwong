# THIRD_PARTY_NOTICES

iotwong 应用本体沿用仓库 MIT 许可。下表列出当前直接依赖；完整传递依赖清单与精确版本见
`web/package-lock.json` 与 `backend/go.sum`（T01 后端仅标准库，无第三方 Go 依赖；
后续任务引入依赖时同步更新本表）。镜像与二进制摘要锁定在 T07 交付文件中登记。

| 组件 | 版本 | 许可证 | 用途 |
|---|---|---|---|
| svelte | 5.57.0 | MIT | 前端框架（Svelte 5，runes 模式） |
| @sveltejs/kit | 2.70.3 | MIT | SvelteKit 路由/构建 |
| @sveltejs/adapter-static | 3.0.10 | MIT | 静态 SPA 适配 |
| @sveltejs/vite-plugin-svelte | 7.3.0 | MIT | Vite 插件 |
| vite | 8.2.2 | MIT | 前端构建工具 |
| typescript | 6.0.3 | Apache-2.0 | 类型检查 |
| svelte-check | 4.7.6 | MIT | Svelte 类型/静态检查 |
| tailwindcss | 4.3.3 | MIT | CSS 工具（设计 token 基础） |
| @tailwindcss/vite | 4.3.3 | MIT | Tailwind 4 Vite 插件 |
| shadcn-svelte | 1.6.1 | MIT | shadcn-svelte 组件 CLI/配置 |
| bits-ui | 2.19.0 | MIT | shadcn-svelte 底层原语 |
| lucide-svelte | 1.0.1 | ISC | 统一图标（Lucide）。说明：本仓库以全局 runes 编译 Svelte，lucide-svelte 源码仍使用 $$props 而不兼容；因此其图标 path 数据已静态提取到 `web/src/lib/icons.ts`（见文件头注释），许可随 lucide-svelte 一并保留。 |
| clsx | 2.1.1 | MIT | class 合并 |
| tailwind-merge | 3.6.0 | MIT | Tailwind class 去冲突 |
| class-variance-authority | 0.7.1 | Apache-2.0 | 组件变体 |
| tw-animate-css | 1.4.0 | MIT | 微动画 |
| eslint | 9.39.5 | MIT | 静态检查 |
| eslint-plugin-svelte | 3.23.0 | MIT | Svelte ESLint 规则 |
| typescript-eslint | 8.69.0 | MIT | TS ESLint 规则 |
| @eslint/js | 9.39.5 | MIT | ESLint 核心规则包 |
| globals | 17.12.0 | MIT | ESLint 全局声明 |
| @types/node | 24.13.3 | MIT | Node 类型（工具链） |

工具链（构建环境，非运行时依赖）：

| 组件 | 版本 | 许可证 | 说明 |
|---|---|---|---|
| Go | 1.27.1 | BSD-3-Clause | 后端编译（go.mod `go 1.24.0` 保证向前兼容） |
| Node.js | 24.20.0 (LTS) | MIT | 前端工具链 |
| Go 模块代理 | goproxy.cn / proxy.golang.org | — | 仅网络通道，非交付组件 |

待办：MapLibre GL JS 已在 package-lock 登记；国内底图默认高德栅格瓦片（© 高德地图，
attribution 已随底图样式展示）。正式使用请在高德开放平台申请 Web端(JS API) Key，经
构建期 `VITE_MAP_KEY` 注入（见 .env.example），并遵守高德服务条款。Mosquitto、PostgreSQL、
PostGIS、TimescaleDB 镜像摘要与许可见 T07 交付登记。
