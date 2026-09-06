-- 0003_positions_hypertable.sql
-- 时序位置表（docs/03-data-design.md）。
-- 必须先定义含 recorded_at 的主键，再调用 create_hypertable。

CREATE TABLE positions (
    tenant_id       uuid NOT NULL,
    device_id       uuid NOT NULL,
    recorded_at     timestamptz NOT NULL,     -- 设备采集时间（UTC）
    position_id     uuid NOT NULL DEFAULT gen_random_uuid(),
    event_id        text NOT NULL,            -- 关联 ingest_events
    received_at     timestamptz NOT NULL,     -- 服务端接收时间
    point           geometry(Point, 4326) NOT NULL,
    speed_mps       double precision CHECK (speed_mps IS NULL OR speed_mps >= 0),
    heading_deg     double precision CHECK (heading_deg IS NULL OR (heading_deg >= 0 AND heading_deg < 360)),
    accuracy_m      double precision CHECK (accuracy_m IS NULL OR accuracy_m >= 0),
    location_source text NOT NULL CHECK (location_source IN ('gnss', 'lbs', 'wifi', 'unknown')),
    raw_ref         text,                     -- 追溯引用（批次/外部事件），非敏感原始数据
    PRIMARY KEY (recorded_at, position_id),
    FOREIGN KEY (tenant_id, device_id) REFERENCES devices (tenant_id, id) ON DELETE CASCADE
);

-- chunk 初始 1 天，样本负载后再调（ADR-004：不依赖 TSL 压缩/策略）
SELECT create_hypertable('positions', by_range('recorded_at', INTERVAL '1 day'));

CREATE INDEX positions_device_time_idx ON positions (tenant_id, device_id, recorded_at DESC, position_id DESC);
CREATE INDEX positions_point_gist_idx   ON positions USING gist (point);
