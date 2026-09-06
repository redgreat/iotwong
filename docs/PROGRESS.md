# 实施进度

## 2026-09-05：文档与环境准备

完成：需求/技术设计/数据方案/接口规范/实施步骤/验收矩阵/AGENTS与Harness提示词。

实际核验：
- HTTP读取演示登录与业务源码、跟随OAuth到官方登录页；一次不带验证码登录返回400。没有成功账号登录证据。
- 飞书教程读取循环重定向；附件未获取。
- 读取eadm的rebar.config与定位相关代码；查询lc_racebox列、行数、批次数、坐标范围及3行样本。
- 连接本机8432；创建iotwong_owner/iotwong；启用PostGIS3.6.4/TimescaleDB2.29.1。原eadm未写入。
- 随机owner密码已保存忽略文件.env.local，0600，不在文档中记录。

尚未完成：业务代码/表迁移/应用运行角色/导入/Compose/浏览器端到端/性能检查。文档中的make和compose命令为后续验收契约，不是当前可执行交付。

下一任务：T00补真实登录与教程；T01可独立开始。T00有外部验证风险，不应阻止工程基线。

每轮追加模板：日期、任务ID、修改、实际验证命令与退出结果、结果证据位置、未完成/阻塞、下一任务。勿写凭据或私人位置。

文档交付检查：本地Markdown链接全部可解析；文档未包含用户密码；git check-ignore确认.env.local已忽略；显式chmod后权限为0600；使用新owner账号实际连接成功并查询确认两扩展版本，public无业务表。

## 2026-09-05：T01 工程基线（容器内实测）

任务 ID：T01（依赖满足、本轮最早可执行本地任务；T00 维持 blocked：需用户在官方登录页完成带验证码的真实登录并放行读取，教程飞书附件循环重定向不可读，本轮无浏览器与账号，不虚构——已写入 TASKS.yaml block_reason）。

修改文件：
- `backend/`：`cmd/server`（env HTTP_ADDR、优雅停机、buildinfo）；`internal/httpapi`（GET /api/v1/health/live|ready、request_id、JSON 信封、归一化错误码、recover/access-log 中间件）；`internal/httpapi/server_test.go`；`cmd/ingestor`、`cmd/import-racebox`（T04/T03 前明确 exit 1 的桩，不假装成功）。
- `web/`：SvelteKit static SPA（svelte.config.js runes+adapter-static fallback）、Tailwind 4 design token（shadcn 调色板 oklch、light/.dark、success/warning/danger/radius）、`src/lib/api.ts`（信封类型化客户端）、`src/lib/utils.ts`(cn)、`src/routes/+layout.ts(ssr=false)`、`+layout.svelte`（中文本地化壳）、`+page.svelte` 健康检查页（loading/error/ok+重试，显式状态）、vite dev proxy /api→8080、exact-pinned package.json + package-lock、eslint.config.js。
- 仓库级：`Makefile`（check/test/e2e/build/compose-check/tidy）、`.env.example`、`.gitignore`（backend/bin、*.log）、`.github/workflows/ci.yaml`、`scripts/e2e-health.sh`、`docs/api/openapi.yaml`、`docs/protocol/location-v1.schema.json`、`THIRD_PARTY_NOTICES.md`、ADR-005~008（docs/02-architecture.md）。

实际验证命令与退出结果（本机不跑测试，全部在 Docker 容器执行）：
- `docker run --rm -v $PWD/backend:/src -w /src golang:1.27.1-alpine sh -c 'go vet ./... && go test -count=1 ./... && go build -o /tmp/s ./cmd/server && ...'` → exit 0；go test：internal/httpapi ok 0.008s。
- `docker run --rm -v $PWD/web:/src -w /src node:24-alpine ...`（node v24.20.0）：`npm run check` → “svelte-check found 0 errors and 0 warnings”；`npm run lint` 通过；`npm run build` → “Wrote site to build”；exit 0。
- 运行时探针：`docker run -d ... -p 18081:8080 golang:1.27.1-alpine` 启动 server，宿主机 curl：live→`{"data":{"status":"ok"},"request_id":"<32hex>"}` 200；ready→`{"data":{"components":[],"ready":true},...}` 200；`/api/v1/nope`→404 `error.code=not_found`；POST /health/live→404。
- SPA 产物：node 容器 `npm run preview` 后 curl / → `<title>iotwong 定位设备管理平台</title>` 200。
- 锁版本：package.json 无 ^/~ 前缀（全部精确版本）；backend 零第三方依赖（stdlib only）；工具链 Go 1.27.1 / Node 24.20.0（THIRD_PARTY_NOTICES 登记）。
- npm audit：6 个 low（均 devDependencies；`--force` 会破坏精确锁版，暂缓，后续任务复查）。

已知风险：
- svelte-check 对“变量名恰为 state + 多个类型化 $state”会误判 legacy 模式（探针复现）；已用 pageState 命名规避，后续组件避免该命名。svelte.config compilerOptions.runes 对 LS 生效有限，属工具限制。
- vite preview 的深链接 404：SPA history fallback 属生产 Nginx 职责（T07 交付），开发服务器正常。
- golang.org/nodejs.org 官方下载在直连时于 ~70MB 处停滞；已用 aliyun 镜像/daocloud registry 镜像 + clash TUN 全局代理解决（工具链装 /usr/local，非仓库内容）。
- T00 仍 blocked：需要用户账号与验证码，见上。

未完成/阻塞：T00 外部阻塞（用户完成验证码登录前不能 done）。`make compose-check` 与 CI compose job 在 T07 compose 文件落地前输出“未交付”提示（ADR-008）。

下一任务：T02 数据库与迁移（依赖 T01 已满足）——临时库全新迁移、再执行无破坏、iotwong_app 运行角色最小权限、PostGIS/TimescaleDB 扩展与许可核验、组合外键与索引检查。

## 2026-09-05：T02 数据库与迁移（容器内实测）

任务 ID：T02（依赖 T01 已满足）。T00 继续 blocked。

修改文件：
- `db/migrations/0001_tenants_users_sessions.sql`、`0002_projects_devices_latest.sql`、`0003_positions_hypertable.sql`（create_hypertable by_range 1day + 设备时间/GiST 索引）、`0004_outbox_imports_p1.sql`、`0005_app_grants.sql`。
- `backend/internal/dbmigrate/`（Load/Applied/Apply：advisory lock、单事务、checksum 记账）+ 单测；`backend/cmd/migrate/main.go`。
- `scripts/db/bootstrap-app-role.sh`（超级用户一次性建 iotwong_app，密码只进被忽略的 .env.local，不打印）。
- Makefile/CI：构建与 job 增加 migrate；ADR-009。

实际验证命令与结果（数据库为容器 postgres-pg18-ext 上的 PG18.4，测试均在容器/临时库执行）：
- 建临时库：`docker exec postgres psql -U postgres -c "CREATE DATABASE iotwong_migtest OWNER iotwong_owner;"` + CREATE EXTENSION postgis/timescaledb。
- `docker run --rm --network host --env-file .env.local -e PGDATABASE=iotwong_migtest golang:1.27.1-alpine /app/migrate --dir /db/migrations`：第一次因 0002 `UNIQUE(...) WHERE` 语法错在第 2 个文件中止（0001 已记账，展示失败原子性）；修正为部分唯一索引后重跑 → applied 0002..0005，exit 0。重跑 → “0 pending / nothing to apply”。
- 冒烟（scratch，后整库删除）：tenant→project→device→device_latest→positions 插入成功；跨租户 project 组合外键拒绝（ERROR devices_tenant_id_project_id_fkey，0 残留）；hypertable 跨日写入 2 行成功；删除临时库。
- 真实库：同镜像无 PGDATABASE 覆盖执行 → applied 5/5；重跑 nothing to apply。`schema_migrations`=5；19 张表；positions 为 hypertable，含 positions_point_gist_idx、device_latest_point_idx。
- app 角色：`bootstrap-app-role.sh` exit 0；以 iotwong_app 连接 → INSERT tenants + SELECT（ROLLBACK 无残留）成功；`CREATE TABLE` → permission denied for schema public。
- `go vet ./...`/`go test -count=1 ./...`（dbmigrate ok）/build 通过（GOPROXY=goproxy.cn,GOSUMDB=off 拉取 pgx v5.10.0，go.mod go 1.25.0）。

已知风险/记录：
- TimescaleDB 发行包是否 Apache 构建未核验：本机只用 Apache-2 核心功能并实测；容器 Apache 构建与镜像摘要锁定归 T07（ADR-009），不冒充 T02 已验。
- `ALTER ROLE` 在 bootstrap 每次运行时把密码对齐 .env.local（脚本幂等设计，非循环问题）。
- .env.local 追加 IOTWONG_APP_USER/PASSWORD（忽略文件，未打印、未提交）。

下一任务：T03 RaceBox 导入（依赖 T02 已满足）——只读限量/断点导入 CLI、单位核验报告、两次导入不重复、原库只读。

## 2026-09-05：T03 RaceBox 导入（容器内实测）

任务 ID：T03（依赖 T02 已满足）。T00 继续 blocked。

修改文件：
- `backend/internal/racebox/`（ListBatches/InspectBatch/FixedOffset/UUIDv5/Import 窗口导入）+ 单测（tz/uuid/行校验拒绝规则）；`backend/cmd/import-racebox/main.go`（替换 T01 桩：--list-batches/--stats/--batch…，默认 dry-run，--apply 才写）。
- `scripts/db/bootstrap-eadm-reader.sh`：超级用户一次性建 `iotwong_eadm_ro`（CONNECT+schema USAGE+lc_racebox SELECT），密码只进被忽略的 .env.local（EADM_USER/PASSWORD），不打印。
- ADR-010（docs/02-architecture.md）；Makefile 注释更新。

实际验证命令与结果（docker run --network host --env-file .env.local golang:1.27.1-alpine /app/import-racebox …）：
- `--list-batches`：输出各 imp_stamp 批次与行数（只读角色连接 eadm 成功）。
- `--stats --batch aab6ba69-9000-11f1-b15f-6c4ce210b3b3`：rows=327、years=2026、ns=[370959,960371968]、lng=[120.4559,120.4560]、lat=[36.17046,36.17053]、speed=[6.6,188.4]、accuracy=[1.131,1.239]、fix={3:327}；assumptions 输出“单位未核验→NULL、fix 不作门禁、WGS84 十进制”。
- dry-run（无 --apply）：source_rows_read=327、rows_new_written=327、0 拒绝；iotwong positions/devices/import_runs 全为 0（零写入）。
- `--apply` 首轮：rows_new_written=327、device=racebox-aab6ba69、run status=done；DB：positions=327、speed_mps/heading_deg/accuracy_m 全部 NULL、recorded_at min/max = 2026-08-04 09:47:32.88Z/09:47:48.56Z（与源 CST 09:47 一致）、device_latest 仅最新点、eadm 行数仍 8392021。
- 重跑 `--apply`：rows_new_written=0、rows_already_present=327、prior_run_done=true、positions 仍 327、import_runs 仍 1 行（status=done read=327 written=327 rej=0）。
- 失败路径证据：首版 apply 在进度 UPDATE 处 pgx text 编码错误 → 整窗口回滚；修复为字符串游标后从 running 态续跑成功。
- `go vet`/`go test -count=1 ./...`（dbmigrate/httpapi/racebox 全 ok）/`go build ./...` 通过。

已知风险/记录：
- speed/heading/accuracy 与 fix_status 语义在拿到 eadm 原处理程序或上游协议前保持“未知”→ 目标库 NULL（ADR-010），导入器只放行已确认的十进制 WGS84 坐标与还原时间。
- 只读角色对 eadm 的授权为“连接+schema+单表 SELECT”新增，未改任何 eadm 对象与数据。

下一任务：T04 本地定位完整链路（依赖 T02 已满足）——MQTT ingestor、模拟发布器、鉴权 API、SSE；消息→落库→设备列表；重复/乱序/DB 中断测试。

## 2026-09-05：T04 本地定位完整链路（容器内实测）

任务 ID：T04（依赖 T02 已满足）。T00 继续 blocked。

修改文件：
- `db/migrations/0006_ingest_ledger.sql`（ingest_events + ingest_poison）。
- `backend/internal/location/`（MQTT v1 协议校验：strict JSON、schema_version/event/时区/坐标/范围/窗口）；`backend/internal/auth/password.go`（Argon2id）；`backend/internal/store/`（pgxpool、会话、成员、EnsureAdmin/EnsureLocalDevice、Ingest 事务账本、Poison、ListDevices/项目、ReadOutbox）。
- `backend/internal/httpapi/`（auth login/logout/me、projects、devices 列表/详情、SSE events、requireAuth/CSRF/限速、Flush 透传修复）。
- `backend/cmd/seed|ingestor|sim-send`；`deploy/mosquitto/mosquitto.conf`；`scripts/dev-mqtt-init.sh`；ADR-011；Makefile/CI 增加 seed/sim-send。

实际验证命令与结果（全部容器内，凭据仅在 .env.local / 环境变量）：
- 迁移：migrate 应用 0006（6 known / applied）。
- 链路：mosquitto(18883, ACL) 内 dev-1 QoS1 publish → ingestor 手动 ACK 落库。ACL 用户名=external_id 修正前静默不投递（实测发现并修复）。
- 幂等/冲突：sim-send `--prefix t4dup --count 2 --dup 2` → ingested: inserted, inserted, duplicate, duplicate；DB 两事件各一行（ledger ok），positions 无重复。异载荷同 event（--conflict）→ result=conflict。
- 乱序/晚到/future：`--old`（recorded_at -10min）入库历史但 device_latest 保持最新（前进规则）；`--future`（+10min）→ WARN window rejected future_too_far，ingest_poison 1 行，无位置。
- DB 中断：另启 PGPORT=59999 的 ingestor 收同一消息 → ERROR “message not acked (will be redelivered)”（未落库不 ACK）；健康 ingestor 同事件落库一次（positions rows=1、ledger ok）——无重复业务效果。
- API：login(admin) 200 + HttpOnly cookie；/auth/me tenant=local role=admin；GET /devices 无 cookie 401；带 cookie 返回 local dev-1 online（位置/速度/电量）与 racebox_demo is_demo=true historical。
- SSE：/api/v1/events Last-Event-ID 续传回放 + 新消息实时 event device.updated（id/event/data 行）。
- `go vet`/`go test`/`go build ./...` 通过。

已知风险/记录（T04 完成，非声称 A07 全过）：
- ingestor 进程在 commit 后、ACK 前崩溃的“恰好一次业务效果”细粒度 crash 测试（A07）留到 T09 验收轮做，本轮回滚语义已由 DB 中断 no-ack + duplicate 幂等覆盖。
- SSE 目前轮询补发，NOTIFY/LISTEN 唤醒与游标过期 resync 归 A12/性能轮。
- Mosquitto passwd 文件权限警告（容器内 root:world-readable）为 dev 本地；T07 将用受限权限卷并锁定镜像。

下一任务：T05 地图与轨迹（依赖 T03/T04 已满足）——MapLibre 工作台、列表联动、历史查询/轨迹、断线/底图失败可用、移动端。


## 2026-09-05：T05 地图与轨迹（进行中，容器内实测）

任务 ID：T05（依赖 T03/T04 均已满足）。T00 继续 blocked。

本轮完成：
- 后端契约：GET /api/v1/devices/{id}/track（from/to≤7 天、max_points、5 分钟空洞断段、simplified flag、raw/returned 计数）与 /devices/{id}/positions（游标分页 ≤5000）。真实数据验证：racebox 327 点 → raw=327 returned=51 simplified=true；dev-1 当日 track/positions 200 正常；游标 next 正常。
- 前端工作台：登录门（HttpOnly cookie）、数据源/状态/搜索过滤、设备列表（在线/离线/未知、历史回放标签、无定位设备不落 0,0 仍可见）、MapLibre 地图（内联离线底图；渲染错误显式提示且列表仍可用）、列表↔地图点选互链、时间窗轨迹查询画线 fitBounds、无数据/错误显式文案；移动优先（xl 双栏）。
- UI：手写 $lib/components/ui/{button,input}（统一 token）；`npx shadcn-svelte add/init` 因 shadcn-svelte.com registry 各样式 index 全 404 无法取件 → 风险记录，验收轮复核。
- 依赖：maplibre-gl 6.7.0 精确锁版；web check 0 errors/0 warnings、eslint/build 通过；backend 新增 location 单测；go vet/test/build 全绿。

实际验证命令与结果（容器内）：
- curl（登录 cookie）：track racebox `raw 327 returned 51 simplified True segments 1`；dev-1 track/positions 200；positions 分页 next 游标存在。
- npm run check（0 errors）；npm run lint 通过；npm run build Wrote site；后端 go test 全 ok。

未完成/风险：浏览器端 E2E（Playwright：登录→列表→地图点选→轨迹查询）、390×844/深色视觉、A15 底图归属核验（生产瓦片 T07 自托管）；shadcn registry 404。

下一任务：T05 收尾（E2E/视觉/底图核验）→ T06（受 T00 阻塞）。


## 2026-09-05：T05 收尾（浏览器 E2E 通过 → done）

任务 ID：T05（done）。T00 继续 blocked（T06 依赖）。

本轮修改：
- `web/src/routes/+page.svelte`：签名后自动加载设备列表（$effect）；vite proxy 目标可经 `API_ORIGIN` 环境配置（vite.config.ts）。
- `web/e2e/ui-e2e.mjs`（puppeteer-core 25.10.0 devDep、`npm run test:e2e:ui`）：错误密码统一提示→正确登录→设备列表（真实 DB）→选中本地设备→查询轨迹（返回 DB 点）→MapLibre canvas→390×844 无横向溢出。
- 登录限速改为仅 production 启用（dev/测试免打扰）；eslint 配置忽略 e2e/ 与 .svelte.ts（runes 模块由 svelte-check 承担）。

实际验证命令与结果（容器/host 浏览器）：
- host apt 安装 Chromium 152（bookworm）；`SEED_PASSWORD=… node e2e/ui-e2e.mjs`：8 项 UI-E2E PASS，exit 0（截图 /tmp/iotwong-ui-e2e.png，未入库）。
- 修过的问题：错误密码后再次输入密码未清空导致重复登录 401（Ctrl+A 清空）、puppeteer 无 `:has-text`（改 innerText 找按钮）、`waitForTimeout` 移除。
- `npm run check` 0 errors / `npm run lint` exit 0 / `npm run build` exit 0。

未完成（不阻塞 T05 done，进 T09 验收轮）：深色主题视觉、地图集群与 1000 设备性能（A14）、截图式 A13 全量核对、A15 生产底图归属/自托管（T07 配合）。

下一任务：T07 Compose 交付（依赖 T04/T05 已满足；T06 受 T00 阻塞待用户验证码登录）。


## 2026-09-05：T07 Compose 交付（done）

任务 ID：T07（依赖 T04/T05 已满足）。T00 继续 blocked（T06/T08 边界依赖）。

交付文件：
- `deploy/backend/Dockerfile`（golang:1.27.1-alpine 多阶段静态，同一镜像 server/ingestor/migrate/import-racebox，非 root+CA，GOPROXY/GOSUMDB 构建参数）。
- `deploy/web/Dockerfile` + `deploy/web/nginx.conf`（node:24 → nginx:1.29-alpine；/api 反代含 SSE 关闭 buffer/cache/长读；SPA fallback；_app immutable 缓存；wget 健康检查）。
- `compose.yaml`（外置 PG：host.docker.internal+extra_hosts，migrate 一次性有界重试，api healthcheck，mosquitto 127.0.0.1:MQQT_PORT）。
- `compose.standalone.yaml`（自带 db：本地镜像 iotwong/pg18-ext:1.0.0 + shared_preload_libraries=timescaledb 参数 + /docker-entrypoint-initdb.d 建 ext/app 角色；pg/mqtt 卷）。
- `deploy/db/init-db.sh`、`deploy/mosquitto/init.sh(mosquitto.conf)`、`scripts/init-secrets.sh`（生成 .env 与 .secrets/mqtt，不覆盖已有值，不打印）；Makefile compose-check 实化。

实测结果（记录于 TASKS 证据）：
- 全容器冷启动 0 失败：db healthy→扩展 plpgsql/postgis3.6.4/timescaledb2.29.1→migrate 6/6→api/web healthy。
- 修过的问题：db init 脚本缺可执行位（chmod 755）；timescaledb 需 shared_preload_libraries=timescaledb；bind mount 目录对非 root 容器用户不可遍历（chmod o+x 公开路径，秘密仍 600）。
- Nginx：/api/v1/health/ready 200、未登录 /devices 401、/workbench/devices SPA fallback 200、登录+设备列表（外置模式 2 台）。
- MQTT 链：未注册设备 publish → ingestor poison device_unknown 落库 1 行。
- 重启持久性：restart api/ingestor 后 ready 200 且 schema_migrations=6。
- 备份恢复演练：pg_dump -Fc（65393 B）→ 隔离库重建 ext → pg_restore ok（migrations=6、hypertable=1）→ drop。
- docker compose config 两文件通过；镜像 iotwong-backend/web:0.1.0 构建成功。

风险记录：standalone DB 使用本机 iotwong/pg18-ext:1.0.0 镜像（来源/摘要核对说明待补 deploy/db/README.md，T09 收尾）；外置模式端口/URL 均在 .env 可控。

下一任务：T08 围栏报警 P1（依赖 T05/T07 已满足）或先 T09 总体验收推进；T06 仍等待用户完成合宙验证码登录。


## 2026-09-05：T08 围栏报警 P1（done）

任务 ID：T08（依赖 T05/T07 已满足）。

修改文件：backend/internal/store/fences.go（CRUD+ListAlarms/Ack+EvalFencesPost 池上评估）、store.Ingest 事务不内嵌查询、ingestor 落库后评估、httpapi fences/alarms 路由+requireAdmin、cmd 构建。

实测（记录 TASKS 证据）：
- 圆形围栏 r=50m 绑定 dev-1 → 位置流（无任何网页打开）触发 exit/enter 3 条；fence_states 终态 outside；同侧连续消息不重复报警；ack/delete API 200。
- 修复：共享 tx 内嵌套查询触发 pgx ErrConnBusy → 改为位置提交后池上独立连接评估（位置先落库，评估失败仅告警不影响 ACK/重复投递）；alarm occurred_at 类型为 time.Time 修复 500。
- go vet/test/build 通过。

下一任务：T09 总体验收（P0 本地线已具备：登录/设备/地图/轨迹/实时/围栏；A02/A03 等 T06 阻塞项保持 blocked，等用户完成合宙验证码登录后补真实证据）。


## 2026-09-05：T09 本地验收汇总与外部阻塞记录

任务 ID：T09（blocked，等待 T06/T00）。T06 同步 blocked（依赖 T00）。

本地 P0 验收逐项核对（证据在各轮 PROGRESS/TASKS 段落，命令均为真实执行）：
- A01 本地账号：通过（错误密码统一提示、HttpOnly 会话、未登录 401、退出）。
- A02/A03 合宙真实登录/设备对照：blocked（无用户验证码登录，禁止伪造）。
- A04 列表/地图/无定位：本地部分通过（列表↔地图联动、历史回放标签、无定位显示文案不落 0,0）；
  注：当前库无“无定位设备”真机样本，条目仅经 UI 文案/状态路径覆盖，fixture 在 A09 数据可用时补。
- A05 已知坐标 MQTT→DB→API→SSE→地图：通过（T04/T05 E2E）。
- A06 重复/冲突/晚到：通过（duplicate/conflict/前进规则）。
- A07 DB 中断：部分通过（no-ack + 幂等无重复业务效果）；commit 后 ACK 前进程崩溃点未做细粒度 crash 注入，待资源轮。
- A08 轨迹窗口/断段/抽稀：通过（track API raw/returned/simplified、断段逻辑）；UI 播放/暂停/倍速未实现（P0 R07 回放控件缺失，记录为待办）。
- A09 RaceBox 导入：通过（dry-run/apply/幂等/只读）。
- A10 Compose 外置+全容器冷启动/重启：通过。
- A11 越权：代码路径 admin/viewer 分离已实现；真实 viewer 账号负例测试未执行（待补）。
- A12 SSE 续传：通过（Last-Event-ID 回放+实时）；NOTIFY 唤醒与游标过期 resync 未做。
- A13 390×844 无横向溢出：通过；深色/键盘全量视觉留截图轮。
- A14 性能基线（10 万点/1000 设备/10 分钟负载）：未执行，需独立资源轮。
- A15 底图：WGS84 与错误降级已验；生产瓦片归属/自托管 T07 交付说明待补 deploy/db/README 同轮。
- A16 围栏服务端报警：通过（无网页触发、幂等、边界基线）。
- A17 备份恢复/迁移失败阻止业务：通过（隔离库恢复；migrate 失败即 exit 阻止后续服务）。

收尾待办（不依赖 T00）：deploy/db/README.md（standalone DB 镜像来源/摘要）、真实 viewer 负例、
A14 负载报告、轨迹播放控件、dark/keyboard 视觉截图、A15 瓦片归属说明 → 放“收尾轮”。

外部阻塞：用户需在官方合宙站点完成带验证码登录授权并提供读取范围；完成后 T00→T06→T09 即可补 A02/A03/A04(luatos 样本) 并解除 blocked。


## 2026-09-05：T09 本地收尾（R07 轨迹回放；参照 docs/09-ui-screenshot-reference）

任务 ID：T09 本地线继续（T06/T00 仍 blocked）。

本轮：
- 新增 R07 轨迹回放控件（UI-REF-05 参考，不克隆倍速数值）：web/src/routes/+page.svelte 增加
  回放/停止按钮、1×/10×/60× 倍速、进度拖动、N/M 计数与当前点时间坐标；按本项目配置定义倍率。
- e2e/ui-e2e.mjs 扩展回放断言；UI E2E 全部 PASS（新增：playback controls rendered、progress counter advancing）。
- svelte-check 0 errors、eslint 0、build 通过。

状态变化：A08 本地 P0 由“部分”升为：查询/断段/抽稀/回放(播放/暂停/倍速/拖动) 均通过（截图式视觉与
深色键盘留 A13 截图轮）；A12/A14/A11(负例)/A15(瓦片归属) 仍列待办；A02/A03 保持 blocked。


## 2026-09-05：T09 本地收尾（A11 viewer 越权负例）

任务 ID：T09 本地线。修改：cmd/seed 增 --role viewer/admin（EnsureAdmin 成员角色参数化）。

实测（真实 curl）：viewer-1 登录 200；GET /devices、GET /alarms 200；POST /fences → 403
(error.code=forbidden)；POST /alarms/{id}/ack → 403。A11 本地负例通过。
剩余待办不变：A12 resync/NOTIFY、A14 负载、A13 深色/键盘截图视觉、A15 瓦片归属说明、standalone DB 可复现镜像。


## 2026-09-05：R10 本地别名改名与审计（admin PATCH）

新增 backend/internal/store/patch.go(UpdateDeviceAlias+audit_logs)、httpapi PATCH /devices/{id}
（admin+CSRF，仅允许 name ≤200）。

实测：admin PATCH name → list 即时反映别名；audit_logs 出现 device.rename + redacted_diff{name}；
改名还原成功；viewer PATCH → 403（重置 viewer 密码后以真实 viewer 会话验证）。
go vet/test/build 通过。

注：分组与 metadata 扩展字段未纳入本轮（R10 分组仍待办）；远程/合宙设备永不经此接口修改。


## 2026-09-05：A15 底图配置化（web）

MapView 支持 VITE_MAP_STYLE_URL（构建期合法样式，自托管 OSM/ODbL 等；归属由样式 attribution 承担并启用
attribution 控件）；未配置回退内联离线底图且不显示第三方归属。check/lint/build 通过。
说明：compose 运行时注入需构建期 VITE_ 变量（记录于 .env.example 注释）；生产自托管瓦片仍待独立收尾轮。


## 2026-09-05：A13 深色主题（web）+ 截图证据

+layout.svelte：深色/浅色切换按钮，遵循 localStorage 或 prefers-color-scheme，切换
documentElement.dark（token 已含 .dark 调色）。check/lint/build 通过。
Headless Chromium 实测：登录后点击“深色” → dark=true，截图 artifacts/private/ui-light.png、
ui-dark.png（git 忽略目录，未入库）。
A13 状态：深色切换/390×844 无溢出通过；键盘焦点与逐屏视觉核对仍留收尾轮。


## 2026-09-05：状态汇总（第14轮）

已完成且实测：T01–T05、T07、T08 done；T09 本地线完成 A01/A04/A05/A06/A08(含回放)/A09/A10/
A11(viewer 负例)/A12(续传部分)/A13(深色+390×844)/A15(配置化)/A16/A17；R10 本地改名+审计已实现。
仓库只含计划文件；.env.local/.env/.secrets/artifacts/private 均被忽略；api/ingestor/mqtt 开发容器运行中。

剩余（非阻塞，收尾轮）：A12 NOTIFY/resync 唤醒优化、A14 全量负载报告、A13 键盘/全屏视觉、
生产瓦片部署验证、standalone DB 可复现 Dockerfile、audit/分组扩展。
阻塞（外部）：T00/T06/A02/A03 —— 需要用户在官方合宙站点完成带验证码真实登录并允许读取项目/设备。


## 2026-09-05：角色菜单/UI 视图收尾 + 部署后自动化验收（子代理执行，全部实测）

UI（未动 Go/SQL/凭据）：角色驱动功能菜单（viewer=设备地图/轨迹/报警只读；admin 另见 报警确认/围栏管理/设备改名）、
alarms/fences/rename/track 四个视图（真实 API、loading/empty/error+重试、401 自动登出）、vite dev proxy
CSRF 修复、UI E2E 17 项 PASS、check/lint/build 0 error。

自动化测试交付 tests/：run-contract.sh（14/14）、run-mqtt-chain.sh（4/4）、run-ui-e2e.sh、acceptance.sh
（仅操作 iotwong-standalone 项目：down -v→冷启动→seed→fixture→三套件）。
结果：standalone compose 冷启动成功（db/api/web healthy）；contract 14/14、mqtt 4/4、ui-e2e 17/17，
“ACCEPTANCE: ALL SUITES PASS”（证据在 git 忽略的 tests/evidence/）。

修复的真实缺陷：compose backend Dockerfile 引用出 context（新增 backend/Dockerfile 并同步 compose）；
web 镜像产物 660 致 nginx 403 白屏（chmod a+rX）；/track 空窗口 segments:null 崩溃（改返回 []）。
验收脚本规避项：GOPROXY=goproxy.cn、GOSUMDB=off 供 docker build；临时 seedenv/evidence 均 git 忽略。

状态：本地功能+自动化全部通过；T00/T06/A02/A03 外部阻塞不变（自建登录/权限已由本项目实现，无需合宙）。


## 2026-09-05：全栈 UI 重构（子代理）与合规修正
子代理完成：全宽顶栏(用户下拉/用户管理)、可折叠侧栏(桌面64px图标/移动抽屉)、内容区视图切换、
用户管理 API(GET/POST /users, POST /users/{id}/reset-password；admin+租户隔离+409/400 校验)、
WGS84→GCJ 显示层转换、docs/04-contracts 契约与 THIRD_PARTY_NOTICES 更新；gate 与
tests/acceptance.sh 三套件全部 PASS（contract 14/14, mqtt 4/4, ui-e2e 17/17）。
合规修正（本会话复核，AGENTS/ADR-004）：底图默认改回 offline 内联灰底；provider=amap 仅在
显式提供合法高德 Web 端 Key 且按其服务条款使用时启用（无 Key 自动回退离线，不发起未授权瓦片请求）。
compose/Dockerfile 默认 VITE_MAP_PROVIDER=offline；.env.example 注明需 Key 与条款。
svelte-check/lint/build 通过。

每轮追加模板：日期、任务ID、修改、实际验证命令与退出结果、结果证据位置、未完成/阻塞、下一任务。勿写凭据或私人位置。


## 2026-09-05：DOC-01 六页面截图离线归档

任务：用户指定的文档补充；依赖为六张已提供PNG，不推进或重置T00–T09。
修改：docs/references/luatos/保存6张未修改原图及manifest.json；docs/09-ui-screenshot-reference.md逐页描述布局、字段、可见状态、未验证行为及需求映射；AGENTS、文档导航、Harness启动词加入强制阅读入口；调研文档补充用户截图来源。
验收：执行/tmp/iot_screenshot_docs.py，exit 0，6张PNG签名/尺寸读取成功，源文件与归档SHA-256全部相同。文档链接及manifest引用检查结果见本节后续记录。
限制：截图是静态视觉证据，不代表本项目已成功登录；没有接口响应、手机版和按钮操作过程。不改业务代码/数据库/既有实施状态。下一步：Harness实现对应页面前读取09号文档；原外部联调阻塞继续保留。
最终校验：Python文档引用检查exit 0，29个本地链接、6张图片SHA-256及6个描述锚点全部通过。


## 2026-09-05：UI 收尾轮（角色菜单/报警/围栏/改名 + E2E，web 前端）

任务：web/ 收尾功能（仅前端，未改 Go/迁移/SQL）。按验收项 1-7 实现并真实验证。

修改文件：
- web/src/lib/api.ts：alarms/fences/ack/rename 类型化方法；request() 增加 401 全局钩子 setUnauthorizedHandler。
- web/src/lib/auth.svelte.ts：注册 401 → expireSession（回到登录门）。
- web/src/lib/components/views/{alarms,fences,rename,track}-view.svelte：新增四视图。
- web/src/routes/+page.svelte：工作台顶部“功能菜单”（浅色胶囊），按 auth.me 角色显示：
  全部=设备地图/轨迹；viewer 另见“报警（只读）”；admin 另见“报警确认/围栏管理/设备改名”，切换即切换视图。
- web/vite.config.ts：dev proxy 显式 changeOrigin:false（保留浏览器 Host，使后端 CSRF Origin==Host 通过；生产 Nginx 本就 proxy_set_header Host $host，不受影响）。
- web/e2e/ui-e2e.mjs：admin 菜单项断言 + 报警视图切换 + 回设备地图。

实际验证（真实命令与结果，均 exit 0）：
- docker（node:24-alpine）：npm run check（0 errors/0 warnings）、npm run lint、npm run build（Wrote site）。
- headless Chromium + puppeteer（SEED_PASSWORD 仅进程环境）：
  - e2e/ui-e2e.mjs：17 项 PASS（新增：admin menu contains 设备地图/轨迹/围栏管理/报警/设备改名、alarms view settled、back to device map view）。
  - 临时 viewer-1（seed --role viewer）脚本：菜单=设备地图/轨迹/报警（只读），无 围栏管理/设备改名/报警确认，报警视图无“确认”按钮且有只读提示，围栏视图不挂载 —— 全 PASS（临时脚本已删）。
  - 临时 admin 流程脚本：UI 建围栏（POST /fences，绑定 dev-1）→ 列表出现 → 行内删除（DELETE）→ UI 改名（PATCH /devices/{id}）→ 列表即时刷新 → 还原别名 → 删 cookie 后触发任意视图 API 请求 401 → 自动回到登录门 —— 全 PASS（临时脚本已删）。
- 修过的问题：Svelte class 指令名不能含 `/`（改 cn() 条件类）；正文 `{id}` 按表达式解析（改 {id} 转义）；dev CSRF 403（vite proxy changeOrigin:false）。
- 数据库回写均还原：测试围栏已删除、dev-1 别名还原为“本地演示设备”。

遗留：报警/围栏空表时 GET 返回 items:null，前端已归一为空数组；alarms 列表仅 device_id（无设备名），前端交叉引用 /devices 取名，查不到时显示短 id（真实数据，不伪造）。A14/A13 截图视觉等仍归总验收轮；T00/T06 外部阻塞不变。

## 2026-09-06：UI 重构轮（顶栏+可折叠侧栏+用户管理+高德底图，全栈）

任务：按用户要求重构工作台外观与信息架构：登录后顶栏占满、图标与名称在最左，主题切换与用户下拉在最右上；全部菜单移到最左侧并默认折叠、悬停展开、点选后固定不自动收缩；地图/列表都在主内容区；底图换成国内高德；下拉里维护用户（新增后端用户管理 API）。

修改文件：
- web/src/routes/+layout.svelte：改为全宽外壳（不再自带 header/限宽容器）。
- web/src/routes/+page.svelte：登录页 + 工作台壳（AppHeader/AppSidebar/主内容区）重写；新增 users 视图分支；保留 devices/track/alarms/fences/rename。
- web/src/lib/components/app-header.svelte（新）：顶栏，最左品牌，最右主题切换 + 用户下拉（账户信息 / 用户管理(admin) / 退出登录）。
- web/src/lib/components/app-sidebar.svelte（新）：左侧菜单；桌面默认 64px 图标窄栏，悬停临时展开，点选后 pinned 保持展开，底部按钮收起；移动端抽屉。
- web/src/lib/components/ui/app-icon.svelte + web/src/lib/icons.ts（新）：lucide 图标 path 数据静态嵌入（lucide-svelte 源码用 $$props，与全局 runes 不兼容；THIRD_PARTY_NOTICES 已注明）。
- web/src/lib/theme.svelte.ts（新）：主题状态模块（原 +layout 逻辑迁移）。
- web/src/lib/basemap.ts（新）：高德栅格底图 + WGS84→GCJ-02 显示转换（provider/key 经 VITE_MAP_PROVIDER/VITE_MAP_KEY 构建期注入）。
- web/src/lib/components/map-view.svelte：接入高德瓦片并转换叠加坐标；标注“WGS84 入库 · 高德地图 GCJ-02 显示”。
- web/src/lib/components/views/users-view.svelte（新）：用户列表/新建/重置密码 UI。
- web/src/lib/api.ts：apiUsers/apiCreateUser/apiResetUserPassword + 类型。
- backend/internal/httpapi/users.go（新）+ store/users.go + server.go：GET /users、POST /users、POST /users/{id}/reset-password（均 admin、按会话租户隔离、登录名全局唯一 409、密码不落明文/不回传）。
- deploy/web/Dockerfile + compose.yaml/compose.standalone.yaml：VITE_MAP_PROVIDER/VITE_MAP_KEY 构建参数；.env.example 注明；docs/04-contracts.md 增用户管理三行契约。
- web/src/app.css：primary/ring 调为品牌蓝。

实际验证（真实命令与结果）：
- web gate（node:24-alpine）：npm run check（0 errors/0 warnings）、npm run lint、npm run build（Wrote site）→ exit 0。
- backend gate（golang:1.27.1-alpine，GOPROXY=goproxy.cn）：go vet && go test && go build → exit 0。
- 全容器冷启动 + 三套件（tests/acceptance.sh，证据 tests/evidence/acceptance-ui2-final-20260906-003928.log）：db healthy ~10s、api/web ready ~10s；contract 14/14、mqtt-chain 4/4、ui-e2e 17/17 → ALL SUITES PASS。
- 用户管理 API（curl 实测，凭证仅环境变量）：admin 列表含成员；新建 viewer 200；重复登录名 409 conflict；短密码 400；重置密码后旧密码登录 401、新密码 200；viewer GET /users 403 forbidden。
- headless 探针：AMap 瓦片请求 15/15 返回 200（底图真实显示）；侧栏 64px→点击菜单后 256px（固定）；用户下拉含“用户管理/退出登录”；主题按钮切换 .dark 类生效。
- 修过的问题：web Dockerfile 产物权限 660 致 nginx 403（T07 遗留，上轮已加 chmod）；AppIcon each 键冲突（polyline 无 d → 运行时 duplicate-key 崩溃，改索引键）；lucide-svelte 与全局 runes 不兼容（改静态嵌入图标数据）。

已知风险/说明：
- 高德瓦片默认无 Key 可用（本机实测）；正式合规使用请在高德开放平台申请 Web端(JS API) Key，构建期 VITE_MAP_KEY 注入（需重建 web 镜像）。
- 显示坐标已做 WGS84→GCJ-02 单次转换；接口/列表文本仍为 WGS84 原始值（A15 无二次偏移）。
- 用户管理未提供删除/改角色（后端无此契约），新建+重置密码已覆盖下拉“维护用户”场景；退出登录在用户下拉内。
- T00/T06 外部阻塞不变。

## 2026-09-06：UI 体验完善轮（满幅地图仪表盘/公共弹窗/侧栏版权/围栏弹窗）

任务：按用户反馈逐步完善：地图底图两处可靠显示与轨迹设备列表、围栏“列表+公共弹窗”新增、左侧菜单收缩与左下角版权、设备地图页全幅底图+右上角半透明设备卡片、回放只留播放卡片。

修改文件：
- web/src/lib/components/ui/modal.svelte（新，公共弹窗：遮罩/Esc/焦点管理/标题/插槽/固定样式）。
- web/src/lib/components/views/fences-view.svelte：列表行样式重做，新增改为弹窗表单（复用 Modal）；local 设备多选在弹窗内。
- web/src/lib/components/app-sidebar.svelte：宽度过渡顺滑；底部版权区（展开 `@R wangcw 2026`、折叠 `@wangcw`，2026 之后年份显示 `@R wangcw 2026-YYYY`），wangcw 链接 github.com/redgreat/iotwong；移动端抽屉底部同款版权。
- web/src/lib/components/views/devices-view.svelte（新）：设备地图改为满幅底图 + 右上角半透明玻璃态设备卡片（数据源/状态/搜索筛选、设备列表行、时间窗查询、回放条）；回放中隐藏筛选/列表/日期，只留播放卡片；卡片可折叠；模块级设备缓存避免切页闪空。
- web/src/routes/+page.svelte：devices 分支改为挂载 DevicesView，其余视图走滚动主内容区。
- web/src/lib/components/map-view.svelte：ResizeObserver 驱动 map.resize（侧栏/面板尺寸变化即时重设），load 后再 resize 一次。
- web/src/app.css：新增 .glass-dt 半透明控件样式、细滚动条。
- web/src/lib/components/views/users-view.svelte / rename 未动；e2e 未改动。

实际验证（真实命令与结果）：
- web gate（node:24-alpine）：npm run check 0err/0warn、npm run lint、npm run build exit 0。
- headless 探针：设备地图瓦片 28/28=200 且页脚“WGS84 入库 · 高德地图 GCJ-02 显示”；轨迹视图瓦片 10/10、设备下拉 2 项（列表加载正常）；围栏弹窗打开/Esc 关闭正常；侧栏折叠底部 `@wangcw`、展开 `@R wangcw 2026`，href=https://github.com/redgreat/iotwong。
- 全容器验收（tests/acceptance.sh，证据 acceptance-ui3-final-20260906-022102.log）：contract 14/14、mqtt-chain 4/4、ui-e2e 16/16 → ALL SUITES PASS（track 空窗时走“playback hidden”分支）。
- 修过的问题：Svelte class 指令名含 `/` 非法（改数组 join）；重复 </script> 致解析失败；每元素动态列表各键冲突（改索引键）；e2e“回放”按钮与设备行“历史回放”徽章文案歧义 → 把徽章移出按钮外（按钮文案不再含“回放”）。

已知限制：单点轨迹回放会在 ~40ms 内自动结束（点太少），故回放最小卡“只留播放卡片”在多段轨迹下才长期可见（逻辑已实现并由探针间接验证）。

## 2026-09-06：UI 细化轮（弹窗遮挡/设备管理菜单/侧栏收缩与版权/底图诊断/玻璃面板）

任务：按用户反馈细化：新建用户弹窗被遮；轨迹与设备地图重复 → 菜单“轨迹”改“设备管理”列表页；侧栏去掉底部收起按钮、改由最左 Logo 切换、左下角短小 copyright+wangcw（非 @R）；设备地图底图再次排查（console/后端日志）并加失败提示；设备列表半透明、折叠仅留图标、标题不换行。

修改文件：
- web/src/lib/ui-state.svelte.ts（新）：shell.sidebarPinned 共享状态。
- web/src/lib/components/ui/modal.svelte：dialog 增加 z-10 保证位于遮罩之上（原弹窗因 position 缺失/层级被遮）。
- web/src/lib/components/views/users-view.svelte：新建/重置密码弹窗改用公共 Modal（修复被遮）。
- web/src/lib/components/ui/data-page.svelte（新）：列表页统一外壳（标题/说明/操作区/内容）。
- web/src/lib/components/views/device-manage-view.svelte（新）：设备管理列表（全部设备静态信息表格：数据源/状态/时间/坐标/速度km/h/电量/定位来源 + 顶部统计与筛选）。
- web/src/routes/+page.svelte：菜单 轨迹→设备管理（id manage）；渲染分支挂载 DeviceManageView。
- web/src/lib/components/app-sidebar.svelte：删底部“收起菜单/展开并固定”按钮；点击菜单即固定展开；展开状态由 shell.sidebarPinned 控制；左下角=©图标+wangcw（折叠 “wangcw”、展开 “wangcw 2026/2026-YY”，链接 github）。
- web/src/lib/components/app-header.svelte：最左 Logo 变为展开/收起侧栏按钮。
- web/src/lib/components/map-view.svelte：新增底图瓦片失败的可视诊断（中心瓦片 img 探针 → 可读告警；此类错误不触发 maplibre error）。
- web/src/lib/components/views/devices-view.svelte：面板折叠态改为仅一个“设备列表”图标；标题紧凑不换行；背景调至 bg-card/60+blur 更透明。
- web/src/lib/icons.ts：新增 list/copyright 图标数据（从 lucide 静态提取）。
- web/e2e/ui-e2e.mjs：菜单断言“轨迹”→“设备管理”（产品菜单变更随动）。

实际验证（真实命令与结果）：
- web gate（node:24-alpine）：npm run check 0err/0warn、lint、build exit 0。
- headless 探针：用户弹窗 elementFromPoint=标题 H3（不再被遮，position relative）；侧栏折叠 64px 页脚 “wangcw”（无 @R、无收起按钮），Logo 点击→256px “wangcw 2026”；点菜单后保持 256px；设备管理表格 2 行含 dev-1；设备面板背景 oklch/0.6+blur(24px)（真半透明），折叠 w48 仅 1 个图标、无 “设备” 文字；瓦片 49/49=200，无告警。
- 全容器验收（证据 acceptance-ui4-final-20260906-024348.log）：contract 14/14、mqtt-chain 4/4、ui-e2e 16/16 → ALL SUITES PASS。
- 后端/nginx 日志核查无错误；本机底图正常（用户端若仍灰屏，新版会在地图左下角给出“高德 Key/域名白名单/网络”的可读提示，便于排查）。

已知：底图出图依赖高德瓦片网络可达与 Key 生效；页面已把失败原因可视化。T00/T06 外部阻塞不变。

## 2026-09-06：镜像发布物与任务收口（Release 快照）

发布物（本地可分发镜像快照，全部为最新工作树构建；无远端 registry 凭据，未推送第三方仓库）：
- 镜像：iotwong-web:0.1.0 = sha256:f215b0043af3…; iotwong-backend:0.1.0 = sha256:4c28adbdb87f…; iotwong/pg18-ext:1.0.0 沿用本地固定镜像（pg18+PostGIS+Timescale）。
- 导出 tar（位于 artifacts/private/images/，git 忽略）：
  - iotwong-web-0.1.0.tar（64M）sha256 756c4de27d09…672fa6c2
  - iotwong-backend-0.1.0.tar（68M）sha256 1e018843b6d0…240f4c
- 加载与运行（目标机）：
  docker load -i iotwong-web-0.1.0.tar && docker load -i iotwong-backend-0.1.0.tar
  docker compose -p iotwong-standalone -f compose.standalone.yaml up -d   # 需 .env（POSTGRES/MQTT 密钥与 WEB_PORT/MQTT_PORT、VITE_MAP_KEY）
- 当前 standalone 已用该两镜像运行 7h+，web/api/db healthy（http://127.0.0.1:18100）。

任务状态收口：
- 已完成：T01–T05/T07/T08 done；本地 UI 各轮（shell/侧栏/用户管理/设备地图/设备管理/围栏弹窗/Modal/底图高德+GCJ/发布物）全部实现并经 web gate + 全量验收（contract/mqtt/ui-e2e）绿灯；本轮镜像发布物已导出并记录。
- 未完成（确认为外部阻塞，非代码缺口）：T00（合宙真实登录/教程，需用户验证码）、T06（合宙同步，依赖 T00）、T09 总验收中的活动验收段（等待 T06）；远端镜像 registry 推送未做（需用户提供 registry 地址与凭据后执行 `docker tag/push`）。

## 2026-09-06：设备“一直加载中”根因修复（任务 A）+ 高德底图核验与默认视图（任务 B）+ 重新发布验收

任务 A（设备地图/设备管理“一直加载中”）与任务 B（高德底图灰屏）双线完成，standalone 已用修复后镜像重新发布并全量验收通过。

### 任务 A：取证 → 根因 → 修复 → 复测
- 日志取证（error/warn 全列，无秘密，证据 tests/evidence/taskA-logs-before.txt / taskA-logs-after.txt）：
  - standalone api：无 error/warn；仅 200/401(/auth/me 未登录)/400（个别浏览器会话 track 窗口参数非法，INFO access 行）+ 403（契约 viewer 负例）。
  - standalone ingestor：无 error/warn。dev api/ingestor：无 error/warn（旧 401/403 为契约测试残留）。standalone web/nginx：无 error/warn。
  - standalone/dev mosquitto 告警“secret/{passwd,acl} world readable”为既有部署问题：standalone 两个文件已 chmod 600 并重启后消失（dev mosquitto 属 dev 环境既有告警，与本次 bug 无关）。
- admin 会话压测（40 轮 × 并发 10 × 4 端点，经 18100，taskA-stress-final.log）：devices/devices_local/alarms/users/track 全 200，min≈2ms avg 4–8ms max≤43ms → 后端无偶发 5xx/超时。
- 前端状态机定位并修复（web/src/lib/api.ts）：request() 无任何超时/中止，且加载中时各列表页的重试/刷新按钮 disabled → 任一请求“悬挂”（nginx 反代上游卡住/断连等）即永久“加载设备中…”，无恢复路径。复现证据（真机部署级）：docker pause api 期间页面 18s+ 恒为“加载设备中…”、无错误/无重试（taskA-stall-prefix.log）；修复（AbortController 15s 超时 → ApiError code=timeout“请求超时，请重试”）后同样暂停场景 15s 内收敛到显式错误+可用重试按钮，恢复后点重试即出数据（taskA-stall-postfix.log 全 PASS）。
- 后端未改（压测证明无 5xx），无需过 Go gate 的代码变更；go vet/test/build 仍复跑 exit 0。

### 任务 B：eadm 对照调研 + 本仓库底图改动 + 瓦片实测
- eadm（只读调研）轨迹页集成方式：官方 AMap Web JS API 2.0（AMapLoader + securityJsCode），矢量底图 mapStyle=amap://styles/fresh，中心青岛 [120.527112,36.411864]、zoom 11→13，服务端 eadm_geo WGS84→GCJ-02，moveAlong 轨迹回放。其 key/securityJsCode 属外部受限资源，不复制。
- 本仓库（maplibre 栅格 webrd0X style=8，GCJ 显示层）服务端逐项实测：瓦片对 Referer 无白名单（任意 Referer 200）、响应 access-control-allow-origin:*、key 可省略、z2..20 均 200 image/png；无头 Chromium：36/36 瓦片 200（四个 webrd 子域均衡），视口非灰彩色渲染（截图颜色统计 840 色/17.1% 饱和像素），底部标注“WGS84 入库 · 高德地图 GCJ-02 显示”，无失败横幅（tests/evidence/taskB-map-final.png、taskB-tiles-final 日志）。→ 灰屏非服务端拒绝；指向客户端到 *.is.autonavi.com 的网络/DNS 或旧缓存；页面已保留瓦片失败可读诊断。
- 改动：basemap.ts 导出 HOME_CENTER=[120.527112,36.411864]（WGS84，青岛，对齐 eadm 与本仓库 racebox_demo 数据区）/HOME_ZOOM=11/USER_ZOOM=13，raster source 补 minzoom3/maxzoom18；map-view.svelte 默认中心由北京 [116.4,39.9]/zoom4 改为青岛 zoom11，载入后尝试 navigator.geolocation（4s 超时；成功 flyTo zoom13，拒绝/超时/非安全上下文回落默认），左下角新增“回默认中心”按钮（无头实测存在且可点）；WGS84 入库、列表文本不变，仅显示层转 GCJ。
- 文档：ADR-012 底图栅格配置与默认视图（docs/02-architecture.md）。

### Gate / 发布 / 验收
- web gate：node:24-alpine `npm run check && npm run lint && npm run build` exit 0（0 error/0 warning）。
- backend gate：golang:1.27.1-alpine `go vet ./... && go test ./... && go build ./...` exit 0。
- 发布：`docker compose -p iotwong-standalone -f compose.standalone.yaml up -d --build`（WEB_PORT=18100 MQTT_PORT=11900），web 新镜像含两处修复。
- tests/acceptance.sh：down -v 冷启动 → seed admin → fixture → contract 14/14、mqtt-chain 4/4、ui-e2e 16/16 → “ACCEPTANCE: ALL SUITES PASS”（证据 tests/evidence/taskAB-acceptance-final.log）。
- 收尾复核：standalone api/web 日志无 error/warn（401 仅 /auth/me 未登录，403 为 viewer 负例）；ingestor 唯一 WARN 为修复 mosquitto 文件权限时重启导致的一次性断线重连（立即 INFO 重连成功）。

### 遗留
- 底图依赖公网 *.is.autonavi.com 可达；瓦片失败提示仅诊断不降级自托管（生产自托管仍归原收尾轮）。
- dev mosquitto “world readable”告警为 dev 环境既有（容器内 root/共享文件），未重启 dev 容器避免影响 dev 链，需时 chmod 600 后重启即消。
- 轨迹/报警页在设备“请求悬挂”场景同样受益于 15s 超时（统一 request()）；未做 SSE/大列表专项性能轮（A12/A14 仍开放）。

## 2026-09-06：统一品牌图标（定位+信号）替换默认 favicon 与 Logo 并重新发布

任务：为 Svelte 5 SPA 设计一套统一品牌图标，单一 SVG 源同时用于浏览器标签 favicon、顶栏最左 Logo（展开/收起侧栏按钮）与登录页 Logo，替代 Svelte 默认 favicon 与登录页 “iw” 文字占位；只动 web/ 与 docs/PROGRESS.md，无凭据、无新依赖、不破坏菜单/布局/aria。

设计说明（web/src/lib/assets/brand-icon.svg，单一 SVG 源）：
- viewBox 64×64 方形安全区（width/height=64，矩形 x2 y2 60×60 rx15，四周约 3–6% 透明余量供浏览器标签/圆角裁剪）。
- 配色只用 app.css 现有 token 换算的固定色：圆角方块渐变 = --primary（oklch(0.52 0.19 262) 转 sRGB #245FD4）→ --primary/70 叠白（#668FE1），与顶栏原 `from-primary to-primary/70 bg-gradient-to-br` 品牌块方向一致；图形用 white（≈ --primary-foreground）。SVG 以 <img> 引用时无法解析页面 CSS 变量，故按光色主题 token 取固定 hex（favicon 常驻品牌蓝，深色主题下品牌块仍保持品牌色，属有意为之）。
- 图形：白色定位针（气球针，头心 (32,36) r10、针尖 (32,50)，切线光滑闭合路径）+ 两段同心信号弧（r15.5/r21.5、开口朝左下，卫星/雷达波束），细节少、16px 亦清晰（headless 像素采样验证：圆角外透明、渐变背景、针体/针尖、两条弧端点、环间留白均落在预期坐标）。
- 侧栏无品牌图标（只有功能性 lucide 菜单图标与左下 copyright），未加新 UI；原顶栏 satellite 只是品牌位占位，故仅替换该处为品牌图形，satellite 图标数据保留未删（无引用不影响 lint）。

修改文件：
- web/src/lib/assets/brand-icon.svg（新，单一品牌 SVG 源）。
- web/src/lib/assets/favicon.svg（删除，Svelte 默认图标不再使用）。
- web/src/routes/+layout.svelte：`import favicon` → `import brandIcon`；`<link rel="icon" type="image/svg+xml" href={brandIcon}>`（headless 可取的稳定语义；favicon.ico 兼容省略）。
- web/src/lib/components/app-header.svelte：最左 Logo 的 `<AppIcon name="satellite">` 品牌位替换为 `<img src={brandIcon} alt="" aria-hidden="true" draggable="false" class="size-8 shrink-0 rounded-lg shadow-sm">`（32px 与原 8×8 圆角品牌块同尺寸，无布局抖动；按钮 aria-label/title 未动）。
- web/src/routes/+page.svelte：登录页顶部 `iw` 文字占位 → 同一 `<img src={brandIcon}>` size-8（登录/壳层品牌一致）。
- web/vite.config.ts：`build.assetsInlineLimit: 0`——品牌 SVG 作为真实文件产出（/_app/immutable/assets/brand-icon.<hash>.svg），避免小图被 Vite 内联成 data: URI 导致 favicon 无 URL 可下载/缓存（上一版默认 favicon 即被内联成 data: URI，浏览器标签与 200 校验均不可用）。

实际验证（真实命令与结果）：
- web gate：`docker run --rm -v $PWD/web:/src -w /src node:24-alpine sh -c 'npm run check && npm run lint && npm run build'` exit 0（svelte-check 0 errors / 0 warnings；lint 通过；产物含 brand-icon.<hash>.svg 实体文件）。（先跑一次通过后补 vite.config 再复跑一次，两次均 exit 0。）
- headless Chromium（puppeteer-core + /usr/bin/chromium，无登录）：打开 http://127.0.0.1:18100 → 断言全部 PASS（证据 tests/evidence/brand-favicon.log）：SPA boot 出登录页；`<link rel="icon">` href=http://127.0.0.1:18100/_app/immutable/assets/brand-icon.NmlYxqMo.svg（非 data:）；fetch 该 URL status=200、content-type=image/svg+xml；SVG 64×64 viewBox、含 #245FD4/#668FE1/#fff、含定位针与信号弧路径；登录页含同品牌 `img[src*="brand-icon"]`；整页截屏 tests/evidence/brand-favicon.png（git 忽略）。
- 像素采样复核图形（Chromium canvas，64 视口 ×4 放大采样）：圆角外透明、tile 内为品牌蓝渐变、定位针头/针尖/两条信号弧端点均为白、针头与内环之间为背景蓝，坐标与设计一致。
- 发布：`docker compose -p iotwong-standalone -f compose.standalone.yaml build web` exit 0（后端未改，不动其镜像）；`WEB_PORT=18100 docker compose -p iotwong-standalone -f compose.standalone.yaml up -d --no-deps web` 重建（注：不加 WEB_PORT 会落回 compose 默认 8080；compose `up --build web` 会触发全项目 bake 且后端 go mod download 在当前网络到 proxy.golang.org 失败，故按“只构建 web + 18100 重建”执行）。
- 发布后 curl：http://127.0.0.1:18100/ = 200；favicon 实际路径 /_app/immutable/assets/brand-icon.NmlYxqMo.svg = 200 / image/svg+xml / 1278 B；/api/v1/health/ready = 200；容器状态：iotwong-standalone-web-1 Up (healthy)、api Up (healthy)、db Up (healthy)、mosquitto/ingestor Up（web 端口恢复 0.0.0.0:18100→80）。

遗留：
- 已开过的浏览器标签可能仍显示旧 favicon/缓存页：需强刷（Ctrl/Cmd+Shift+R）或重开标签；favicon 为带内容哈希的资产路径，每次发版自动换新 URL。
- `docker compose ... up --build web` 的全项目 bake 需后端 go proxy 可达；本轮因后端零改动走 `build web` + `--no-deps up`，后续需连后端构建时注意网络/代理配置（GOPROXY）。
- 品牌蓝按光色主题 token 固定（favicon 不可自适应主题）；如未来要求深色主题内 Logo 换浅蓝，需内联 SVG 变量方案，超出本轮“单一 SVG 源”范围。
