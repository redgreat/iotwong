-- 0004_outbox_imports_p1.sql
-- 事件通知 outbox / 导入账本 / P1 围栏与报警 / 审计（docs/03-data-design.md）。

CREATE TABLE outbox_events (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id  uuid NOT NULL,
    type       text NOT NULL CHECK (type IN ('device.updated', 'alarm.created', 'sync.status', 'resync')),
    device_id  uuid,                          -- 通知最小 DTO 引用
    payload    jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX outbox_created_idx ON outbox_events (created_at);
CREATE INDEX outbox_tenant_idx  ON outbox_events (tenant_id, id);

CREATE TABLE import_runs (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    source         text NOT NULL CHECK (source IN ('racebox_demo')),
    source_batch_id text NOT NULL,
    status         text NOT NULL CHECK (status IN ('running', 'done', 'failed', 'interrupted')),
    last_source_id text,                      -- 断点续传游标
    rows_read      integer NOT NULL DEFAULT 0,
    rows_written   integer NOT NULL DEFAULT 0,
    rows_rejected  integer NOT NULL DEFAULT 0,
    config_hash    text,
    started_at     timestamptz NOT NULL DEFAULT now(),
    finished_at    timestamptz,
    UNIQUE (source, source_batch_id)
);

CREATE TABLE import_rejections (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    run_id     uuid NOT NULL REFERENCES import_runs(id) ON DELETE CASCADE,
    source_id  text NOT NULL,                 -- 拒绝行的源 id，不含敏感轨迹
    reason     text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX import_rejections_run_idx ON import_rejections (run_id);

CREATE TABLE fences (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name        text NOT NULL,
    kind        text NOT NULL CHECK (kind IN ('circle', 'polygon')),
    geometry    geometry(Polygon, 4326),      -- polygon 用；circle 用 center+radius
    center      geometry(Point, 4326),        -- circle 用
    radius_m    double precision CHECK (radius_m IS NULL OR radius_m > 0),
    enabled     boolean NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now(),
    CHECK (kind = 'circle'  OR geometry IS NOT NULL),
    CHECK (kind = 'polygon' OR (center IS NOT NULL AND radius_m IS NOT NULL))
);
CREATE INDEX fences_tenant_idx ON fences (tenant_id);

CREATE TABLE fence_devices (
    fence_id   uuid NOT NULL,
    tenant_id  uuid NOT NULL,
    device_id  uuid NOT NULL,
    PRIMARY KEY (fence_id, device_id),
    FOREIGN KEY (fence_id) REFERENCES fences(id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, device_id) REFERENCES devices (tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE fence_states (
    tenant_id     uuid NOT NULL,
    fence_id      uuid NOT NULL,
    device_id     uuid NOT NULL,
    last_state    text NOT NULL CHECK (last_state IN ('inside', 'outside', 'unknown')),
    last_event_time timestamptz,
    PRIMARY KEY (tenant_id, fence_id, device_id)
);

CREATE TABLE alarms (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    uuid NOT NULL,
    fence_id     uuid NOT NULL,
    device_id    uuid NOT NULL,
    transition   text NOT NULL CHECK (transition IN ('enter', 'exit')),
    occurred_at  timestamptz NOT NULL,
    acked_by     uuid REFERENCES users(id),
    acked_at     timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, fence_id, device_id, occurred_at, transition)
);
CREATE INDEX alarms_tenant_time_idx ON alarms (tenant_id, occurred_at DESC);

CREATE TABLE audit_logs (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id     uuid NOT NULL,
    actor         uuid,                       -- 可为系统动作（NULL）
    action        text NOT NULL,
    target        text,
    created_at    timestamptz NOT NULL DEFAULT now(),
    redacted_diff jsonb                       -- 已脱敏差异；禁止记录凭据/坐标原文
);
CREATE INDEX audit_logs_tenant_time_idx ON audit_logs (tenant_id, created_at DESC);
