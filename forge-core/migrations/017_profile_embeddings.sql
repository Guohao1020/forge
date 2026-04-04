-- 017_profile_embeddings.sql
-- Vector embeddings for project profile semantic search (S16).
-- Requires pgvector extension.

-- Enable pgvector (idempotent)
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS engine.profile_embeddings (
    id          BIGSERIAL PRIMARY KEY,
    project_id  BIGINT NOT NULL REFERENCES engine.projects(id) ON DELETE CASCADE,
    dimension   VARCHAR(50) NOT NULL,   -- api_catalog, db_schema, module_graph, etc.
    chunk_id    VARCHAR(100) NOT NULL,  -- unique identifier within dimension (e.g., endpoint path, table name)
    chunk_text  TEXT NOT NULL,          -- the text that was embedded
    embedding   vector(1536),           -- OpenAI text-embedding-3-small dimension
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_profile_embedding UNIQUE (project_id, dimension, chunk_id)
);

-- HNSW index for fast approximate nearest neighbor search
CREATE INDEX IF NOT EXISTS idx_profile_embeddings_vector
    ON engine.profile_embeddings
    USING hnsw (embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 200);

CREATE INDEX IF NOT EXISTS idx_profile_embeddings_project
    ON engine.profile_embeddings(project_id, dimension);
