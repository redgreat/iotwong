# iotwong 项目文档

版本：0.1，调研日期：2026-09-05。目标：把合宙定位设备查看与自建 MQTT 定位管理放进统一的、可自行部署的 Web 管理平台。本轮交付是需求和可执行开发规范，不是已实现的网站。

| 文件 | 用途 |
|---|---|
| [00-research.md](00-research.md) | 参考站源码观察、登录限制、本地数据库实测、来源 |
| [01-requirements.md](01-requirements.md) | 功能边界、交互、活动验收目标 |
| [02-architecture.md](02-architecture.md) | 技术选型、模块、可靠性与开源策略 |
| [03-data-design.md](03-data-design.md) | 表、索引、导入、时空数据规则 |
| [04-contracts.md](04-contracts.md) | 自建 HTTP/MQTT/SSE 契约与合宙适配边界 |
| [05-implementation-plan.md](05-implementation-plan.md) | 分阶段实施与依赖 |
| [06-deployment.md](06-deployment.md) | Docker Compose、已有 PG、备份与初始化 |
| [07-acceptance.md](07-acceptance.md) | 可验证验收清单 |
| [TASKS.yaml](TASKS.yaml) / [PROGRESS.md](PROGRESS.md) | 机器任务表与跨轮进度 |
| [08-harness-prompt.md](08-harness-prompt.md) | 可复制给 DeepSeek Harness 的启动提示词 |
| [09-ui-screenshot-reference.md](09-ui-screenshot-reference.md) | 六页原图、离线文字说明和功能范围映射；不能识图也可读取 |
| [截图机器索引](references/luatos/manifest.json) | 图片路径、尺寸、SHA-256 |
| [../AGENTS.md](../AGENTS.md) | 仓库级 AI 执行规范 |

已准备：本机 127.0.0.1:8432 的 `iotwong` 数据库和 `iotwong_owner` 账号；PostGIS 3.6.4、TimescaleDB 2.29.1 已启用。密码只保存在仓库忽略的 `.env.local`，权限 0600。业务表、应用运行角色、数据导入和网站均尚未创建。

优先级：先跑通本地定位完整链路，同时尽早验证合宙登录与回调部署条件；合宙联调失败不妨碍本地实现，但会阻塞活动提交。活动是否仍有效、名额及主办方主观判断不在技术保证范围内。
