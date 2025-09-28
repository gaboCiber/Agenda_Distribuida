from fastapi import APIRouter
from datetime import datetime
from schemas import HealthResponse
from services.event_service import redis_service

router = APIRouter(tags=["health"])

@router.get("/health", response_model=HealthResponse)
async def health_check():
    """Verifica salud del gateway y conexiones"""
    health_status = HealthResponse(
        service="api_gateway",
        status="healthy",
        timestamp=datetime.now(),
        dependencies={}
    )
    
    # Verificar Redis
    if redis_service.is_connected():
        health_status.dependencies["redis"] = "connected"
    else:
        health_status.dependencies["redis"] = "disconnected"
        health_status.status = "unhealthy"
    
    return health_status

@router.get("/")
async def root():
    return {
        "message": "API Gateway (Pub/Sub) is running",
        "mode": "event_publisher",
        "timestamp": datetime.now().isoformat()
    }