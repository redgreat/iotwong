-- 0001_tenants_users_sessions.sql
-- 租户/用户/成员关系/会话（docs/03-data-design.md）。
-- 由迁移 runner 在单事务内执行；所有对象归迁移连接用户（iotwong_owner）所有。

CREATE TABLE tenants (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    login         text NOT NULL UNIQUE,
    password_hash text NOT NULL,           -- Argon2id
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE memberships (
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id   uuid NOT NULL REFERENCES users(id)   ON DELETE CASCADE,
    role      text NOT NULL CHECK (role IN ('viewer', 'admin')),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, user_id)
);
CREATE INDEX memberships_user_idx ON memberships (user_id);

CREATE TABLE sessions (
    token_hash text PRIMARY KEY,           -- 只存哈希，明文 token 不落库
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL
);
CREATE INDEX sessions_expires_idx ON sessions (expires_at);
