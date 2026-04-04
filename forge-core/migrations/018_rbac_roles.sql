-- 018_rbac_roles.sql
-- Seed the 5-level RBAC role hierarchy.
-- These roles are used by the RequireRole middleware for access control.
--
-- Hierarchy: VIEWER < DEVELOPER < PROJECT_ADMIN < ORG_ADMIN < PLATFORM_ADMIN

INSERT INTO auth.roles (tenant_id, code, name, scope)
VALUES
    (1, 'VIEWER', '查看者', 'PLATFORM'),
    (1, 'DEVELOPER', '开发者', 'PLATFORM'),
    (1, 'PROJECT_ADMIN', '项目管理员', 'PLATFORM'),
    (1, 'ORG_ADMIN', '组织管理员', 'PLATFORM')
ON CONFLICT DO NOTHING;

-- PLATFORM_ADMIN already exists from initial seed data.
-- Verify all 5 roles exist:
-- SELECT code, name FROM auth.roles WHERE code IN
--   ('VIEWER','DEVELOPER','PROJECT_ADMIN','ORG_ADMIN','PLATFORM_ADMIN');
