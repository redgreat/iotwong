# 数据模型与导入规范

## 已有环境与角色

本地库 iotwong 已就绪，无业务表；owner 凭据在 `.env.local`。T02 通过一次性管理员初始化脚本创建独立 `iotwong_app`（owner本身无CREATEROLE，不能冒充管理员建角色），随后由owner授予最小对象权限。`iotwong_app`（仅 CONNECT、schema USAGE、指定表 CRUD/序列权限），owner 仅迁移使用。扩展已由管理员安装，应用不得在启动时携带 postgres 管理密码。不要把 owner 密码用于前端。

## 逻辑表（由 T02 产出实际 SQL migration）

| 表 | 关键字段 / 键 | 索引与约束 |
|---|---|---|
| tenants | id uuid PK, name, created_at | 首版一个本地租户，仍保留隔离 |
| users / memberships | user id, login, password_hash；tenant_id,user_id,role | login 唯一；成员联合唯一 |
| sessions | token_hash PK, user_id, expires_at | expires_at；不上链明文 token |
| provider_accounts | id,tenant_id,provider,external_subject,secret_ciphertext,expires_at | 外部账号与租户绑定；秘密不得进入查询 DTO |
| projects | id,tenant_id,source,external_id,name | (tenant_id,source,external_id) 唯一 |
| devices | id,tenant_id,project_id,source,external_id,name,expected_report_interval_s,metadata | (tenant_id,source,external_id) 唯一；(tenant_id,id) 唯一供组合 FK |
| device_latest | tenant_id,device_id PK,last_seen_at,last_position_at,point,speed_mps,battery_pct,location_source,quality | FK (tenant_id,device_id)；GiST(point)；租户/last_seen_at |
| ingest_events | tenant_id,device_id,event_id,payload_hash,received_at,status | PK (tenant_id,device_id,event_id)；普通表实现跨时间去重 |
| positions | tenant_id,device_id,recorded_at,position_id,event_id,received_at,point,speed_mps,heading_deg,accuracy_m,source,raw_ref | hypertable recorded_at；PK(recorded_at,position_id)，设备时间索引与 GiST |
| outbox_events | id bigint identity PK,tenant_id,type,device_id,payload,created_at | (tenant_id,id)，只存最小通知 DTO |
| import_runs | id,source_batch_id,status,last_source_id,rows_read,rows_written,rows_rejected,config_hash | 唯一导入策略/批次；可恢复 |
| import_rejections | run_id,source_id,reason | 不存完整敏感轨迹 |
| fences / fence_devices | tenant_id,id,name,kind,geometry,radius_m,enabled；device bindings | P1；空间校验；租户组合外键 |
| fence_states / alarms | fence/device,last_state,last_event_time；transition,occurred_at,acked_by | P1，转换事件幂等 |
| audit_logs | tenant_id,actor,action,target,created_at,redacted_diff | 禁止记录凭据 |

所有跨设备引用都用 tenant_id+device_id 组合外键或等价严格约束，防止把别租户设备写入当前租户记录。point 为 `geometry(Point,4326)`，经度 X、纬度 Y。位置非空且经纬度范围合法，速度 nullable 且 >=0，航向 [0,360)，精度 >=0；无 fix 的通信写事件/last_seen，不写假位置。

建议时序迁移核心（示意，完整表定义由 T02 生成并测试）：

```sql
-- 必须先定义 positions 表以及包含 recorded_at 的主键。
SELECT create_hypertable('positions', by_range('recorded_at', INTERVAL '1 day'));
CREATE INDEX positions_device_time ON positions
  (tenant_id, device_id, recorded_at DESC, position_id DESC);
CREATE INDEX positions_point_gist ON positions USING gist(point);
```

chunk 初值一天，经样本负载再调整；切忌只给 position_id 单列 UNIQUE。原生分区备用实现保留相同 API，建立未来时间分区、历史导入范围分区和自动维护，不同时启用两种分区机制。数据库保存纳秒原字段用于追溯，PG 时间只保留微秒，幂等不依赖微秒精度。

## 写入事务与生命周期

一次事务：插入 ingest_events（冲突比较 payload_hash）→ 插入有效位置 → 更新 device_latest → 写 outbox → COMMIT → MQTT ACK。相同 event_id 相同载荷直接返回重复；不同载荷记冲突，不覆写历史。最新位置只接受更晚 recorded_at；相等时用稳定 position_id 排序。last_seen_at 是自建链路有效通信的接收时间，单独更新；历史导入不刷新真实通信状态。晚到事件可以进历史但不能把地图位置倒退。

默认未来超过服务端5分钟的数据进入隔离；自建在线消息最大迟到7天（可配置），更老数据使用明确回补入口。position 保留180天只是初始策略，导入历史演示租户默认不自动清理。幂等账本保留至少覆盖迟到窗口+7天，删除后过期消息仍必须因时间规则拒收；不能清账后重新接受任意历史消息。outbox 保留24h，短于此范围的客户端可续传，否则 resync。

距离 ST_Distance / 围栏距离用 point::geography，单位米；不要用经纬度 geometry 的度当米。区域 polygon 必须合法、闭合、限制顶点数；轨迹线按时间顺序，同一设备跨5分钟空洞默认断段，跳点按精度/速度阈值标异常不删除原始数据。抽稀仅改变显示数据，不用于里程精算。

## RaceBox 导入

不能全量导入839万行作为首次演示。默认选择一个批次，最多100000行，`WHERE id > last_id ORDER BY id LIMIT 5000` 分批；查询列显式列出、设置 statement_timeout、只读事务，不建原库索引。默认 dry-run，写入须显式 `--apply`。提供 `--batch`、`--max-rows`、`--source-timezone`、`--speed-unit` 参数和 JSON 汇总。

| 源字段 | 目标 / 规则 |
|---|---|
| id + imp_stamp | event_id=`racebox:<batch>:<id>`，稳定可重跑 |
| imp_stamp | 演示设备 `racebox-<batch>`；是演示映射，不宣称真实硬件身份 |
| longitude / latitude | 样本为十进制度；已知数据源确认为 WGS84 后直接入库，不再缩放 |
| year…second + nanoseconds | 采集时间；显式验证时区，保留 raw 纳秒和 UTC 微秒时间 |
| insert_time | source_inserted_at 元数据，不替代 recorded_at |
| speed | 未确认单位前 null；核验协议/原处理程序后才配置转换系数 |
| fix_status / accuracy | 含义需协议确认，不擅自把所有非0值当有效 GNSS |
| itow | 可用于核对采样连续性，不替代完整日期，需处理周回绕 |

T03 先核验日期有效性、负/超范围纳秒的归一化、fix 状态、坐标系、速度单位。时区未知必须要求导入参数，不静默按服务器时区。若只能暂定，记录 assumption 标记且 UI 不输出速度/里程结论。导入时 received_at 是本次导入时间，source_inserted_at 单列元数据；地图默认展示该设备真实历史日期，不能伪造为今天。

公开演示默认使用合成坐标或经明确脱敏的样本；真实 RaceBox 轨迹只用于本地验证，不能进 git/公开截图。单位验证完成前允许先展示已确认的坐标点，但速度与里程保持未知。
