-- 测试租户
INSERT INTO identity_tenant (name, status) VALUES ('test-tenant', 1);

-- 测试角色
INSERT INTO identity_role (tenant_id, role_code, role_name) VALUES
(1, 'ADMIN', '管理员'),
(1, 'USER', '普通用户');

-- 测试管理员 (密码: admin123)
INSERT INTO identity_user (tenant_id, username, password_hash, nickname, status) VALUES
(1, 'admin', '$2a$10$N.zmdr9k7uOCQb376NoUnuTJ8iAt6Z5EHsM8lE9lBOsl7iKTVKIUi', '测试管理员', 1);

-- 测试普通用户 (密码: user123)
INSERT INTO identity_user (tenant_id, username, password_hash, nickname, status) VALUES
(1, 'testuser', '$2a$10$dXJ3SW6G7P50lGmMQoeVhOOoMZCFe0AvhldPOzdQQJXpSGgR4k5Ge', '测试用户', 1);

-- 角色绑定
INSERT INTO identity_user_role (user_id, role_id) VALUES (1, 1);
INSERT INTO identity_user_role (user_id, role_id) VALUES (2, 2);
