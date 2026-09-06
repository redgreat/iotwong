# 调研与证据

补充：用户后续提供了六个页面的截图，已存入 [离线截图参考](09-ui-screenshot-reference.md)。以下“未取得截图”是首次调研的历史记录；现已有用户截图，但仍没有本项目成功登录与真实接口响应证据。本补充不改变 T00/T06 验证状态。

## 证据等级

A = 实际命令/响应核验；B = 公共前端源码观察，未确认服务端契约；C = 用户提供的信息或设计假设。不得把 B/C 写成已通过端到端验收。

## 合宙站点（2026-09-05）

- A：`https://move.luatos.com` 实际跳转到 `https://iot.luatos.com/ai_app/luatos/HZIOT_Motion_sensor/login.html`，HTTP 200。
- A：OAuth authorize 请求跳转到 `https://iot.openluat.com/auth/login`，带 PKCE challenge 与 state。未完成账号认证，也没有取得真实设备列表。
- A：根据公开登录服务源码进行一次用户提供账号的登录请求（未提供验证码）返回 HTTP 400；不能由此判断密码失效。后续必须在浏览器完成正常流程，若有验证码由用户完成，不能绕过。
- B：登录页使用 OAuth authorize → 回调 token → POST `/iam/luat_oauth/v2/login?token=…` 换业务会话。页面 API 域名是 `api-iot.luatos.com`，页面域名是 `iot.luatos.com`。
- B：业务源码读取 `auth.token`、`auth.salt`、`service.sid`，请求头分别为 `authorization`、`salt`、`sid`，不是 Bearer。源码将 102/103/105 当登录失效。第三方业务码与本项目 HTTP 状态不能混用。
- B：可见设备管理、地图、轨迹回放、电子围栏、报警与性能监控实现。源码中的围栏/报警有浏览器 localStorage 存储实现；本项目自建设备围栏改为服务端持久化，以支持关闭网页后继续判断。
- B：源码同时包含腾讯地图 SDK 和高德瓦片 URL；不复制这些 key、URL 为默认底图，也不推定可再分发。
- B：源码有定位抽稀和时间转换逻辑，部分注释称平台时间为北京时间字面值。这只是样例实现，必须以实际接口样本验证，不照搬浏览器本地时区解析。

已识别候选接口（全部 B，均 POST，网关 `https://api-iot.luatos.com/iot/open_api`）：

| 路径 | 源码请求形状/作用 | 实施要求 |
|---|---|---|
| `/list_my_projects` | `{}`；项目列表 | 核验 project_key、空项目 |
| `/list_my_devices` | `{project,page,size,sort,desc}` | 核验分页和 deviceid 含义 |
| `/search_my_devices` | 上述加 imei_prefix | 不假定跨项目搜索 |
| `/aircloud/list_by_tags` | `{client_id,tags,page,size,filter}` | Tag 和单位必须来自教程/真实数据 |
| `/aircloud/latest_location` | 最近定位 | 请求/响应待验证 |
| `/aircloud/location_history` | 历史轨迹 | 时间窗口/坐标系/分页待验证 |
| `/aircloud/send_cmd` | 下行命令 | 当前范围不调用 |
| `/common/put,list,delete_by_id` | 通用存储，源码有额外签名 | 当前范围不调用 |

教程飞书链接经 HTTP 访问出现循环重定向，未拿到正文/附件；不能宣称已读教程。公共源码引用了 `aircloud (5).zip` 及 v5/v11 等规则，但附件内容本轮未获得。浏览器工具未就绪，未取得登录后页面截图，因此本文件不是完整视觉审计。后续 T00 必须补：教程附件版本、正常登录、设备状态、真实响应脱敏样本、回调域名/部署方式、CORS、会话到期、定位 Tag 与单位。

## 本地项目（A）

参考 `/vol1/1000/Code/eadm/rebar.config`：Erlang OTP 最低 27.2.3，Nova、epgsql、poolboy、lager、ecron，relx release 包含 ERTS。可以借鉴 DB 连接池、容器配置和模块分层；其 master/devel 依赖不作为新项目锁版本方式。`src/apis/api_watch.erl` 有定位入库代码，但并非本次新协议标准。未找到 lc_racebox 对应实现，不能用它推断所有单位。

## 本地数据库（A）

PostgreSQL 18.4，127.0.0.1:8432；`shared_preload_libraries=timescaledb`。
`eadm.public.lc_racebox`：8,392,021 行，106 个不同 imp_stamp（导入批次，不等于 106 台物理设备）。经度范围 116.5975905—120.7145369，纬度 35.3411311—42.2962301。insert_time 范围 2025-01-16 至 2026-09-04（+08）；它是入库时间，不是轨迹采集时间。

相关列：id integer、imp_stamp uuid、year/month/day/hour/minute/second integer、nanoseconds integer、itow integer、fix_status integer、longitude/latitude numeric、speed numeric、heading numeric、horizontal_accuracy numeric、insert_time timestamptz。样本纳秒为 300301183 / 400301141 / 500301099，速度原值约 12136，全集最大 21765.600；不应直接显示为 km/h。

已执行初始化：建立 `iotwong_owner` 和其拥有的 `iotwong` 库，撤销该库 PUBLIC 权限，安装 postgis 3.6.4、timescaledb 2.29.1，撤销 public schema 对 PUBLIC 的 CREATE 权限。未改 eadm 表、未复制真实轨迹、未建业务表。凭据见本地 `.env.local`，不可加入 git。

## 公开资料

- [合宙教程](https://e3zt58hesn.feishu.cn/wiki/CH08wNKqaimeYLkAsWncE96lnRy)：本轮未能读取。
- [演示登录页](https://iot.luatos.com/ai_app/luatos/HZIOT_Motion_sensor/login.html)、[业务页源码入口](https://iot.luatos.com/ai_app/luatos/HZIOT_Motion_sensor/index.html)：仅用功能与协议调研，未确认源码再分发许可，不整页复制。
- [shadcn-svelte 迁移说明](https://www.shadcn-svelte.com/docs/migration)：当前组件基线为 Svelte 5 / Tailwind 4。
- [MapLibre GL JS](https://maplibre.org/maplibre-gl-js/docs/) 与 [许可证](https://github.com/maplibre/maplibre-gl-js/blob/main/LICENSE.txt)：地图渲染库；底图数据另行选择。
- [Mosquitto 许可说明](https://mosquitto.org/blog/2015/02/version-1-4-released/)；[Paho Go](https://eclipse.dev/paho/clients/golang/)：Broker 与客户端候选。
- [TimescaleDB 版本许可区别](https://docs.timescale.com/about/latest/timescaledb-editions/)：Apache 2 版和 TSL Community 版不能混称。
- [Timescale 唯一索引约束](https://docs.timescale.com/use-timescale/latest/hypertables/hypertables-and-unique-indexes/)：唯一键必须包含时间分区列。
