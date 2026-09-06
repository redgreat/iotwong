-- 0005_app_grants.sql
-- 应用运行角色最小权限（docs/03-data-design.md：CONNECT、schema USAGE、
-- 指定表 CRUD、序列权限）。
-- 前置：iotwong_app 角色已由一次性管理员脚本创建（owner 无 CREATEROLE，
-- 不能在这里建角色）。由迁移连接用户（owner）执行。

DO $$
BEGIN
    EXECUTE format('GRANT CONNECT ON DATABASE %I TO iotwong_app', current_database());
END
$$;

GRANT USAGE ON SCHEMA public TO iotwong_app;

GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO iotwong_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO iotwong_app;

-- 后续迁移新建的表/序列自动授予 app 角色
ALTER DEFAULT PRIVILEGES FOR ROLE iotwong_owner IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO iotwong_app;
ALTER DEFAULT PRIVILEGES FOR ROLE iotwong_owner IN SCHEMA public
    GRANT USAGE, SELECT ON SEQUENCES TO iotwong_app;
