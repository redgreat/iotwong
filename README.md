# iotwong

定位设备管理平台，技术栈 Svelte 5 + Go + Mosquitto + PostgreSQL/PostGIS，通过 Docker Compose 部署。

当前阶段：T01 工程基线完成（容器内实测通过：Go vet/test/build、Svelte 类型/静态检查、静态 SPA 构建、健康 API 与信封契约可运行）。数据库与业务模块按 docs/TASKS.yaml 继续推进。

快速验证（需要 Go ≥1.24、Node ≥22，或 Docker）：

```sh
make check   # web typecheck/lint + go vet
make test    # go test ./...
make build   # go binaries + 静态 SPA
make e2e     # 启动 server 并做健康接口冒烟（脚本: scripts/e2e-health.sh）
make compose-check  # T07 交付 compose 前提示“未交付”
```

- [文档导航](docs/README.md)
- [AI 开发规范](AGENTS.md)
- [DeepSeek Harness 启动提示词](docs/08-harness-prompt.md)
- [实施任务](docs/TASKS.yaml)
- [进度记录](docs/PROGRESS.md)
- [第三方许可](THIRD_PARTY_NOTICES.md)
