from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    # Temporal
    temporal_host: str = "localhost:7233"
    temporal_namespace: str = "default"
    task_queue: str = "ai-worker"

    # LLM API Keys
    anthropic_api_key: str = ""
    openai_api_key: str = ""
    dashscope_api_key: str = ""
    deepseek_api_key: str = ""

    # Redis (for streaming code output)
    redis_host: str = "localhost"
    redis_port: int = 6379
    redis_password: str = "forge_redis_2026"

    # Forge Core API
    forge_api_url: str = "http://localhost:8080"
    forge_api_token: str = ""

    # Model defaults
    default_model: str = "claude-sonnet-4-20250514"
    default_provider: str = "anthropic"

    model_config = {"env_file": ".env", "env_file_encoding": "utf-8"}


settings = Settings()
