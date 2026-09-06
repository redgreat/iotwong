-- 0002_projects_devices_latest.sql
-- 外部账号绑定/项目/设备/最新状态（docs/03-data-design.md）。
-- 组合唯一约束支撑“租户内唯一 + 租户组合外键”，防止跨租户引用。

CREATE TABLE provider_accounts (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    provider          text NOT NULL CHECK (provider IN ('luatos')),
    external_subject  text NOT NULL,
    secret_ciphertext text,                -- 应用外部密钥加密；不出现在查询 DTO
    expires_at        timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, provider, external_subject)
);

CREATE TABLE projects (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    source      text NOT NULL CHECK (source IN ('local', 'luatos', 'racebox_demo')),
    external_id text,                      -- 上游项目键；local 为 NULL
    name        text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),                -- 供组合外键
    UNIQUE (tenant_id, source, external_id)
);
-- 每个 (tenant, source) 至多一个无 external_id（local 默认项目）：
-- 部分唯一索引（表约束的 UNIQUE 不支持 WHERE）
CREATE UNIQUE INDEX projects_noexternal_unique
    ON projects (tenant_id, source) WHERE external_id IS NULL;

CREATE TABLE devices (
    id                        uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id                 uuid NOT NULL,
    project_id                uuid NOT NULL,
    source                    text NOT NULL CHECK (source IN ('local', 'luatos', 'racebox_demo')),
    external_id               text NOT NULL,          -- 自建=MID 主题段；上游=设备键
    name                      text NOT NULL DEFAULT '',
    expected_report_interval_s integer,
    metadata                  jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at                timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),                -- 供组合外键
    UNIQUE (tenant_id, source, external_id),
    FOREIGN KEY (tenant_id, project_id) REFERENCES projects (tenant_id, id) ON DELETE CASCADE
);
CREATE INDEX devices_project_idx ON devices (tenant_id, project_id);
CREATE INDEX devices_name_idx   ON devices (tenant_id, name);

CREATE TABLE device_latest (
    tenant_id          uuid NOT NULL,
    device_id          uuid NOT NULL,
    last_seen_at       timestamptz,                 -- 自建链路有效通信接收时间
    last_position_at   timestamptz,                 -- 最近一次真实定位时间
    point              geometry(Point, 4326),       -- 无 fix 时为 NULL，不放 0,0
    location_source    text CHECK (location_source IN ('gnss', 'lbs', 'wifi', 'unknown')),
    speed_mps          double precision CHECK (speed_mps IS NULL OR speed_mps >= 0),
    heading_deg        double precision CHECK (heading_deg IS NULL OR (heading_deg >= 0 AND heading_deg < 360)),
    accuracy_m         double precision CHECK (accuracy_m IS NULL OR accuracy_m >= 0),
    battery_pct        double precision CHECK (battery_pct IS NULL OR (battery_pct >= 0 AND battery_pct <= 100)),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, device_id),
    FOREIGN KEY (tenant_id, device_id) REFERENCES devices (tenant_id, id) ON DELETE CASCADE
);
CREATE INDEX device_latest_point_idx     ON device_latest USING gist (point);
CREATE INDEX device_latest_last_seen_idx ON device_latest (tenant_id, last_seen_at);
