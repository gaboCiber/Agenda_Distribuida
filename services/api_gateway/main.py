from settings import settings
from routers import users_router, health_router
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from datetime import datetime
from typing import Optional, List

# Importar tu EventService existente
from services.event_service import event_service

# Crear aplicación FastAPI
app = FastAPI(
    title=settings.app_title,
    version=settings.app_version
)

# Incluir routers
app.include_router(users_router.router)
app.include_router(health_router.router)

# Modelos de datos
class EventCreateRequest(BaseModel):
    title: str
    description: str
    start_time: datetime
    end_time: datetime
    user_id: str

class EventResponse(BaseModel):
    id: str
    title: str
    description: str
    start_time: datetime
    end_time: datetime
    user_id: str
    created_at: datetime

class EventListResponse(BaseModel):
    events: List[EventResponse]
    total: int

class UserProfileResponse(BaseModel):
    id: str
    email: str
    username: str
    is_active: bool

class HealthResponse(BaseModel):
    status: str
    service: str
    timestamp: datetime
    version: str
    dependencies: dict

# Info adicional
@app.on_event("startup")
async def startup_event():
    print(f"🚀 {settings.app_title} v{settings.app_version} iniciando...")

@app.on_event("shutdown") 
async def shutdown_event():
    print("👋 API Gateway apagándose...")

# Endpoints
@app.post("/api/v1/events")
async def create_event(event_data: EventCreateRequest):
    """Crear evento - Publica evento asíncrono"""
    
    # Usar tu event_service en lugar de redis_client directamente
    result = event_service.publish_event(
        channel="events_events",
        event_type="event_creation_requested",
        payload={
            "title": event_data.title,
            "description": event_data.description,
            "start_time": event_data.start_time.isoformat(),
            "end_time": event_data.end_time.isoformat(),
            "user_id": event_data.user_id
        }
    )
    
    if not result.get("published", False):
        raise HTTPException(
            status_code=503, 
            detail=result.get("error", "Message bus unavailable")
        )
    
    return {
        "status": "processing",
        "message": "Event creation request received and queued",
        "event_id": result["event_id"],
        "timestamp": result["timestamp"]
    }

@app.get("/api/v1/events", response_model=EventListResponse)
async def get_events(user_id: Optional[str] = None, limit: int = 50, offset: int = 0):
    """Obtener eventos - Placeholder hasta que Events Service implemente GET"""
    # Datos de ejemplo para probar la interfaz
    sample_events = [
        EventResponse(
            id="event_1",
            title="Reunión de equipo",
            description="Reunión semanal del equipo de desarrollo",
            start_time=datetime.fromisoformat("2024-09-28T10:00:00"),
            end_time=datetime.fromisoformat("2024-09-28T11:00:00"),
            user_id=user_id or "user123",
            created_at=datetime.now()
        ),
        EventResponse(
            id="event_2", 
            title="Almuerzo con cliente",
            description="Almuerzo de negocios con cliente importante",
            start_time=datetime.fromisoformat("2024-09-28T13:00:00"),
            end_time=datetime.fromisoformat("2024-09-28T14:30:00"),
            user_id=user_id or "user123",
            created_at=datetime.now()
        )
    ]
    
    # Filtrar por user_id si se especifica
    if user_id:
        filtered_events = [event for event in sample_events if event.user_id == user_id]
    else:
        filtered_events = sample_events
    
    return EventListResponse(
        events=filtered_events[offset:offset+limit],
        total=len(filtered_events)
    )

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
            "users_service": "unknown"
        }
    )