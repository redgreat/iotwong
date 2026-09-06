# 接口与消息契约

本文件定义本项目新接口，不是对合宙接口的声称。T01/T04 产出 `docs/api/openapi.yaml` 和 `docs/protocol/location-v1.schema.json`，实现/测试以它们与本文一致为准。

## HTTP v1

同源 `/api/v1`，JSON，ISO8601 UTC 时间，ID UUID 字符串。成功 `{data:...,request_id:...}`，列表增加 `page:{next_cursor:null,total:null}`；total 允许未知。错误 `{error:{code,message,details?},request_id}`，message 不泄露上游凭据。HTTP 400 参数、401 会话、403 角色、404 不存在/跨租户、409 冲突、429 限流、502/503 上游不可用。上游业务码通过规范化 error code 映射，不直接当 HTTP 状态。

| Method / 路径 | 请求和语义 |
|---|---|
| POST /auth/login | `{login,password}` 本地认证；设置 cookie |
| POST /auth/logout | 删除本地/绑定会话；CSRF |
| GET /auth/me | 用户、租户、角色；不返回秘密 |
| GET /auth/luatos/start | 待 T00 确认回调协议后实现，不能伪造认证 |
| GET /auth/luatos/callback | 处理合法回调、验证关联 state/一次性使用（以官方支持协议为准） |
| GET /projects | source 可选 |
| GET /devices | source/project_id/q/status/cursor/limit；limit默认50最大200 |
| GET /devices/{id} | 详情、最新位置、capabilities |
| PATCH /devices/{id} | admin，本地 name/group 允许字段；不透传任意远程修改 |
| GET /devices/{id}/positions | from/to 必填，UTC，[from,to)，最多7天；cursor、limit<=5000 |
| GET /devices/{id}/track | from/to<=7天，max_points<=5000；segments和原始点数/是否抽稀 |
| GET /events | SSE；cookie 会话，不接受 query token |
| GET /health/live | 进程存活，无认证，无内部连接信息 |
| GET /health/ready | 必要DB/迁移状态；503 未就绪 |

P1 围栏 GET/POST/PATCH/DELETE `/fences`（单个使用 /{id}），GET `/alarms`，POST `/alarms/{id}/ack`。写接口 admin、Origin/CSRF 检查；viewer 只读。原始坐标查询与抽稀轨迹是两个接口，不能静默截断原始结果。

用户管理（全部 admin；会话按租户隔离，不跨租户操作）：

| Method / 路径 | 请求和语义 |
|---|---|
| GET /users | admin；当前会话租户下的本地账号列表 `{id,login,tenant_name,role,created_at}`；viewer/未登录 403/401 |
| POST /users | admin；`{login,password,role}`（role=admin\|viewer）；登录名 1-64 位 `[A-Za-z0-9_.-]` 且字母/数字开头，全局唯一冲突 409；创建后自动加入当前租户成员 |
| POST /users/{id}/reset-password | admin；`{password}`（6-128 位）；仅能重置本租户成员（其他租户/不存在 404）；密码永不回传 |

设备 DTO 最小字段：`id,source,external_id,name,project_id,communication_status,last_seen_at,last_position_at,position:{longitude,latitude,crs},location_source,accuracy_m,battery_pct,speed_mps,is_demo,sync_status,capabilities`，缺失为 null。source 枚举 local/luatos/racebox_demo，位置crs统一WGS84。接口以 m/s 输出，UI显示km/h时乘3.6。

cursor 编码排序值及过滤条件指纹，位置排序 `(recorded_at,position_id)`，设备排序 `(name,id)`，拒绝篡改/跨查询使用。轨迹每段含 points，点有 recorded_at 和 coordinates=[lng,lat]；保留起终点、明确段间空洞，返回 `raw_count,returned_count,simplified`。

## 自建 MQTT 协议 v1

topic：`iotwong/v1/devices/{device_external_id}/telemetry`。设备ID限 `[A-Za-z0-9_-]{1,64}`；Broker 凭据映射固定设备及租户，不从 payload 接受租户。设备只能 publish 自己 topic；ingestor 只订阅 telemetry。生产TLS8883，开发1883仅本机/可信网络；retain=false，QoS1。此协议不宣称 Air8202 原生兼容，需要固件/适配器对齐。

```json
{
  "schema_version": 1,
  "event_id": "boot_a-seq_42",
  "recorded_at": "2026-09-05T08:00:00.123Z",
  "position": {"longitude": 116.4, "latitude": 39.9, "crs": "WGS84", "fix": true},
  "speed_mps": 1.2,
  "heading_deg": 90,
  "accuracy_m": 5,
  "battery_pct": 80,
  "location_source": "gnss"
}
```

必填 schema_version、event_id（<=128字符）、recorded_at（带时区）；position 可 null 表示有效心跳但无定位。有 position 时经纬度、crs、fix 必填；fix=false 不写位置。location_source 枚举 gnss/lbs/wifi/unknown。NaN/Infinity、越界、超长、未知版本拒绝；0,0 不能默认代表无定位，有效性由 fix 决定，对当前国内设备可标可疑。payload 原始字节哈希用于幂等冲突判断，发送端重试需保持相同 event_id 与载荷；如果需要重新编码则统一 canonical JSON 哈希并更新契约。

消费者收到 retained 遥测不能当新的实时通信；默认拒绝并记录原因。心跳与历史回补分开，历史回补入口需要管理员权限。future/late 窗口见数据设计。

## SSE

事件类型 `device.updated`、`alarm.created`、`sync.status`、`resync`；id 使用 outbox id，data 只含授权设备最小字段，15s心跳。浏览器 Last-Event-ID 续传，服务端验证租户和登录有效性。游标过期发 resync 后前端重拉设备快照；网络恢复先补事件再校准快照。退避1–30s加抖动，页面销毁取消连接；登出关闭连接。

## 合宙适配冻结条件

T00 必须保存脱敏 fixture 和接口矩阵：请求字段、分页、响应shape、上游错误码、时间时区、坐标系、定位Tag、状态字段、限流、超时、会话到期、授权回调部署条件。未验证字段显示未知，不能以随机数据替代。默认轮询30s、并发<=3、超时15s仅为本项目初值，需根据上游规则调整；429遵循 Retry-After，没有时指数退避。明确来源和最后成功同步时间。不得直接向上游大规模压测。
