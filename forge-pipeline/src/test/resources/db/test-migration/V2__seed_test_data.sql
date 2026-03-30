INSERT INTO environment (tenant_id, name, env_type, namespace, bound_branch, status)
VALUES (1, 'dev', 'FIXED', 'forge-dev', 'develop', 'ACTIVE');

INSERT INTO environment (tenant_id, name, env_type, namespace, bound_branch, status)
VALUES (1, 'staging', 'FIXED', 'forge-staging', 'release', 'ACTIVE');
