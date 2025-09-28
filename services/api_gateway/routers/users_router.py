from fastapi import APIRouter, HTTPException, status
from schemas import UserRegistration, UserLogin
from services.event_service import event_service

# Crear router
router = APIRouter(prefix="/api/v1/users", tags=["users"])

@router.post("/register", status_code=status.HTTP_202_ACCEPTED)
async def register_user(user_data: UserRegistration):
    """
    Registro de usuario - Publica evento asíncrono
    """
    result = event_service.publish_event(
        channel="users_events",
        event_type="user_registration_requested",
        payload=user_data.dict()
    )
    
    if not result.get("published", False):
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail=result.get("error", "Message bus unavailable")
        )
    
    return {
        "status": "processing",
        "message": "Registration request received and queued",
        "event_id": result["event_id"],
        "timestamp": result["timestamp"]
    }

@router.post("/login", status_code=status.HTTP_202_ACCEPTED)
async def login_user(login_data: UserLogin):
    """
    Login de usuario - Publica evento asíncrono
    """
    result = event_service.publish_event(
        channel="users_events",
        event_type="user_login_requested", 
        payload=login_data.dict()
    )
    
    if not result.get("published", False):
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail=result.get("error", "Message bus unavailable")
        )
    
    return {
        "status": "processing",
        "message": "Login request received and queued",
        "event_id": result["event_id"],
        "timestamp": result["timestamp"]
    }