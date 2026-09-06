# iotwong/pg18-ext:1.0.0 — standalone DB 镜像说明

`compose.standalone.yaml` 使用本地镜像 `iotwong/pg18-ext:1.0.0`（本机 PostgreSQL 18.4 +
PostGIS 3.6.4 + TimescaleDB 2.29.1 的既有开发镜像重打标，未含可复现 Dockerfile）。

- 摘要核对：由本机 `postgres-pg18-ext:latest` 打标而来；构建来源/内容物需在独立收尾轮补充
  官方基础镜像加扩展的可复现 Dockerfile 与 digest 锁定后才可用于外部分发（ADR-004/009）。
- TimescaleDB 需 `shared_preload_libraries=timescaledb`：compose 通过 db 服务
  `command: ["postgres", "-c", "shared_preload_libraries=timescaledb"]` 固化。
- 首次启动经 `/docker-entrypoint-initdb.d/10-iotwong-init.sh` 建扩展与应用角色
  （`deploy/db/init-db.sh`），迁移由一次性 migrate 服务执行（6 个版本已验证）。
