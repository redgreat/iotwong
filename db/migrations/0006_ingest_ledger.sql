-- 0006_ingest_ledger.sql
-- MQTT 摄入账本与毒消息表（docs/03-data-design.md 写入事务与生命周期）。
-- 由 T04 引入；跟随既有版本化迁移机制执行。

CREATE TABLE ingest_events (
    tenant_id    uuid NOT NULL,
    device_id    uuid NOT NULL,
    event_id     text NOT NULL,
    payload_hash text NOT NULL,               -- sha256(raw bytes) hex
    received_at  timestamptz NOT NULL,
    status       text NOT NULL CHECK (status IN ('ok', 'conflict', 'poison')),
    PRIMARY KEY (tenant_id, device_id, event_id),
    FOREIGN KEY (tenant_id, device_id) REFERENCES devices (tenant_id, id) ON DELETE CASCADE
);
CREATE INDEX ingest_events_received_idx ON ingest_events (received_at);

-- 毒消息：错误类别 + 受限长度/脱敏摘要 + 接收时间；不存完整原始负载。
CREATE TABLE ingest_poison (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id   uuid,
    device_external_id text,
    reason      text NOT NULL,
    payload_digest text,
    received_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ingest_poison_received_idx ON ingest_poison (received_at);
