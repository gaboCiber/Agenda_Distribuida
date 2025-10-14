from fastapi import APIRouter, HTTPException, status, Depends
from schemas import UserRegistration, UserLogin, UserResponse
from services.event_service import event_service
import httpx
import os
from typing import Optional

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

@router.get("/email/{email}", response_model=UserResponse)
async def get_user_by_email(email: str):
    """
    Obtener información de un usuario por su correo electrónico
    
    Args:
        email: Correo electrónico del usuario a buscar
    """
    try:
        # Hacer la petición al servicio de usuarios
        users_service_url = os.getenv("USERS_SERVICE_URL", "http://agenda-users-service:8002")
        async with httpx.AsyncClient() as client:
            response = await client.get(f"{users_service_url}/api/v1/users/email/{email}")
            
            if response.status_code == 200:
                return response.json()
            elif response.status_code == 404:
                raise HTTPException(status_code=404, detail="Usuario no encontrado")
            else:
                raise HTTPException(status_code=response.status_code, detail=response.text)
                
    except httpx.RequestError as e:
        raise HTTPException(status_code=503, detail=f"Error al conectar con el servicio de usuarios: {str(e)}")
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Error inesperado: {str(e)}")