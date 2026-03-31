-- forge_main is already created by POSTGRES_DB env var
-- Create additional database for Temporal
SELECT 'CREATE DATABASE forge_temporal' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'forge_temporal')\gexec

-- Connect to forge_main and create schemas
\c forge_main;
CREATE SCHEMA IF NOT EXISTS auth;
CREATE SCHEMA IF NOT EXISTS engine;
CREATE SCHEMA IF NOT EXISTS specs;
CREATE SCHEMA IF NOT EXISTS pipeline;
CREATE SCHEMA IF NOT EXISTS billing;
