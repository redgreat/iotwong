# Docker 与环境落地规范

本轮没有应用代码，因此不提供假装能启动的 compose；T07 必须交付真实文件与验证记录。本文件定义交付契约。

## 外置 PostgreSQL（默认本机开发）

`compose.yaml` 包含 web、api、ingestor、mosquitto、migrate；web 对外8080，broker开发绑定127.0.0.1:1883，api仅内部网络。PG使用宿主已有8432。容器内 **不能用127.0.0.1访问宿主PG**，api/ingestor/migrate使用 `host.docker.internal` 加 `extra_hosts: ["host.docker.internal:host-gateway"]`；先确认数据库监听和pg_hba允许Docker网段。若实际NAS不支持此别名，设置明确可达宿主IP，不能直接改全局PG监听/认证来碰运气。

宿主 `.env.local` 是PG客户端配置（PGHOST=127.0.0.1），不是直接喂容器的配置；T07生成独立忽略的 `.env`，映射宿主地址、运行角色密码、MQTT凭据。owner密码仅migrate使用，api/ingestor用app角色。已有库扩展不重复由app创建。

## 全容器模式

交付独立 `compose.standalone.yaml`，包括 db + 上述服务；PG地址db:5432，不把PG端口默认暴露公网。DB镜像必须实际同时支持PostGIS与时序方案，PG主版本与扩展构建需在T02验证并固定摘要。不能假设任意 postgis 镜像都带timescaledb。Timescale需要 shared_preload_libraries 配置；严格开源部署遵循ADR-004。

两个 Compose 文件各自能独立运行，不依赖易混淆的覆盖合并。相同的 web/api/ingestor 镜像与环境变量，只有DB连接/服务依赖差异。migrate 是一次性服务，db健康后执行，成功退出后api/ingestor启动；失败不得继续业务启动。外置模式migrate用有界重试检测PG可达。web可启动，但API未就绪应明确维护状态。

## 最终命令验收契约（后续实现，当前不可执行）

```sh
cp .env.example .env
./scripts/init-secrets.sh
# 本机已有PG：填写host/app角色与migrate连接，secret生成器不覆盖已有值
docker compose --env-file .env -f compose.yaml up -d --build
# 或全容器模式，使用独立环境文件和项目名
docker compose --env-file .env.standalone -p iotwong-standalone -f compose.standalone.yaml up -d --build
```

外置模式初始化脚本不得覆盖现有 `.env.local` 或重新建已存在同名库/角色；碰到已有对象要检查所有者和权限，而不是重置密码。管理员初始化采用一次性CLI从环境读取初始密码，生产不带默认admin/admin。

## 服务构建与配置

- web：多阶段构建静态SPA，Nginx处理SPA fallback，`/api/`反代；SSE关闭buffer/cache，长读超时，API路径不能落到index.html。浏览器只请求同源。
- Go：固定builder镜像、CGO_ENABLED=0（依赖允许时）、非root运行，带CA证书；同一镜像用不同入口/参数启动server和ingestor。不能依赖shell去实现不存在的健康检测命令。
- mosquitto：固定版本、allow_anonymous=false、password_file、ACL、persistence=true、持久化目录权限正确。密码文件通过初始化生成，不内嵌镜像。真实设备接入需8883/TLS和每设备凭据。
- volumes：PG data、mosquitto data、可选region maps；不将源码目录整体挂载生产。secret只读挂载/环境注入，限制宿主权限。
- 环境示例只含占位：APP_ENV、HTTP_PORT、DB_HOST/PORT/NAME/USER/PASSWORD、MIGRATION_*、MQTT_URL/USER/PASSWORD、SESSION_KEY、LUATOS_ENABLED、MAP_STYLE_URL、DISPLAY_TIMEZONE。
- Nginx HTTPS可接用户已有反代，生产会话Secure；健康端点不输出DB凭据。限制容器日志大小，设置重启策略、资源限制和优雅停机等待。

## 验证与备份

T07执行 compose config、image build、空卷启动、迁移失败测试、web/API/MQTT smoke、container重启、宿主PG可达检查。外置PG不要容器化替换现有服务，不重启它来试错。

备份用匹配PG主版本的pg_dump custom格式，凭据从PGPASSFILE读取；先测带扩展的还原流程，再说明灾难恢复。恢复到独立临时库，核对设备/位置数量、最新状态与轨迹查询，不覆盖eadm或正在使用的iotwong。卷删除属于显式破坏性维护，日常脚本不得默认 `down -v`。本地真实定位备份不上传仓库。
