import asyncio
from concurrent.futures import ThreadPoolExecutor
from datetime import datetime

from settings import settings
from routers import users_router, health_router, groups_router, events_router, group_events_router
from models import HealthResponse, UserProfileResponse
from services.event_service import event_service
from services.event_handlers import process_event_response

from fastapi import FastAPI

# Crear aplicación FastAPI
app = FastAPI(
    title=settings.app_title,
    version=settings.app_version
)

# Incluir routers
app.include_router(users_router.router)
app.include_router(health_router.router)
app.include_router(groups_router.router)
app.include_router(events_router.router)
app.include_router(group_events_router.router)

# Iniciar listener de respuestas en segundo plano
@app.on_event("startup")
async def startup_event():
    print(f"🚀 {settings.app_title} v{settings.app_version} iniciando...")

    # Suscribirse a respuestas de eventos
    if event_service.redis and event_service.redis.is_connected():
        try:
            def start_redis_listener():
                pubsub = event_service.redis.client.pubsub()
                pubsub.subscribe('events_events_response')
                print("👂 Escuchando respuestas de eventos en events_events_response...")

                for message in pubsub.listen():
                    if message['type'] == 'message':
                        process_event_response(message)

            # Ejecutar en segundo plano
            executor = ThreadPoolExecutor(max_workers=1)
            loop = asyncio.get_event_loop()
            loop.run_in_executor(executor, start_redis_listener)

        except Exception as e:
            print(f"❌ Error iniciando listener de respuestas: {e}")

@app.on_event("shutdown")
async def shutdown_event():
    print("👋 API Gateway apagándose...")

@app.get("/api/v1/users/me", response_model=UserProfileResponse)
async def get_current_user():
    """Obtener perfil del usuario actual - Placeholder"""
    return UserProfileResponse(
        id="user_123",
        email="usuario@ejemplo.com",
        username="usuario_ejemplo",
        is_active=True
    )

@app.get("/health", response_model=HealthResponse)
async def health_check():
    """Health check mejorado para la interfaz"""
    redis_status = "connected" if event_service.redis.is_connected() else "disconnected"

    return HealthResponse(
        status="healthy",
        service="api_gateway",
        timestamp=datetime.now(),
        version=settings.app_version,
        dependencies={
            "redis": redis_status,
            "events_service": "unknown",
            "users_service": "unknown",
            "groups_service": "unknown"
        }
    )