import os
from pydantic_settings import BaseSettings

class Settings(BaseSettings):
    # Redis configuration
    redis_host: str = os.getenv("REDIS_HOST", "agenda-bus-redis")
    redis_port: int = int(os.getenv("REDIS_PORT", 6379))
    redis_decode_responses: bool = True
    
    # App configuration
    app_title: str = "API Gateway (Pub/Sub)"
    app_version: str = "1.0.0"
    
    class Config:
        env_file = ".env"

settings = Settings()