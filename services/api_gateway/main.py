import asyncio
import json
from concurrent.futures import ThreadPoolExecutor
from datetime import datetime

from settings import settings
from routers import users_router, health_router, groups_router, events_router, group_events_router
from models import HealthResponse, UserProfileResponse
from services.event_service import event_service
from services.event_handlers import process_event_response
from services.response_store import event_responses, group_responses

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

# Función para procesar respuestas de grupos
def process_group_response(message):
    """Procesar respuestas del Groups Service"""
    try:
        group_data = json.loads(message['data'])
        event_type = group_data.get('type')
        correlation_id = group_data.get('payload', {}).get('correlation_id')
        response_data = group_data.get('payload', {}).get('response', {})

        if correlation_id and response_data:
            group_responses[correlation_id] = {
                'success': response_data.get('success', False),
                'message': response_data.get('message', ''),
                'data': response_data,
                'timestamp': datetime.now(),
                'type': event_type
            }
            print(f"✅ Respuesta procesada para {event_type} {correlation_id}: {response_data}")
    except Exception as e:
        print(f"❌ Error procesando respuesta de grupo: {e}")

# Iniciar listener de respuestas en segundo plano
@app.on_event("startup")
async def startup_event():
    print(f"🚀 {settings.app_title} v{settings.app_version} iniciando...")

    # Suscribirse a respuestas de eventos y grupos
    if event_service.redis and await event_service.redis.is_connected():
        try:
            pubsub = event_service.redis.pubsub
            
            async def listen_for_messages():
                await pubsub.subscribe('events_events_response', 'groups_responses')
                print("👂 Escuchando respuestas en events_events_response y groups_responses...")
                
                try:
                    while True:
                        message = await pubsub.get_message(ignore_subscribe_messages=True, timeout=1.0)
                        if message:
                            channel = message['channel']
                            if isinstance(channel, bytes):
                                channel = channel.decode('utf-8')

                            if channel == 'events_events_response':
                                process_event_response(message)
                            elif channel == 'groups_responses':
                                process_group_response(message)
                        await asyncio.sleep(0.1)
                except asyncio.CancelledError:
                    await pubsub.unsubscribe('events_events_response', 'groups_responses')
                    await pubsub.close()

            # Iniciar el listener en el bucle de eventos
            asyncio.create_task(listen_for_messages())

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
    redis_status = "connected" if (event_service.redis and await event_service.redis.is_connected()) else "disconnected"

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