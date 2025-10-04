from settings import settings
from routers import users_router, health_router
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from datetime import datetime
from typing import Optional

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

class EventCreateRequest(BaseModel):
    title: str
    description: str
    start_time: datetime
    end_time: datetime
    user_id: str

# Info adicional
@app.on_event("startup")
async def startup_event():
    print(f"🚀 {settings.app_title} v{settings.app_version} iniciando...")

@app.on_event("shutdown") 
async def shutdown_event():
    print("👋 API Gateway apagándose...")

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