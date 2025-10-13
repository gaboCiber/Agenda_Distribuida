from fastapi import APIRouter, HTTPException, status, Depends
from pydantic import BaseModel
from typing import Optional, List
import httpx
from datetime import datetime

# Crear router
router = APIRouter(prefix="/api/v1/groups", tags=["groups"])

# Modelos de datos
class GroupCreateRequest(BaseModel):
    name: str
    description: Optional[str] = None

class GroupResponse(BaseModel):
    id: str
    name: str
    description: Optional[str] = None
    created_by: str
    created_at: datetime
    updated_at: datetime
    member_count: int

class GroupListResponse(BaseModel):
    groups: List[GroupResponse]
    page: int
    total: int

class MemberResponse(BaseModel):
    id: str
    user_id: str
    role: str
    joined_at: datetime

class InvitationCreateRequest(BaseModel):
    group_id: str
    user_id: str

class InvitationResponse(BaseModel):
    id: str
    group_id: str
    group_name: str
    user_id: str
    invited_by: str
    status: str
    created_at: datetime
    responded_at: Optional[datetime] = None

class GroupEventRequest(BaseModel):
    event_id: str

# Función auxiliar para hacer requests al Groups Service
async def make_groups_service_request(endpoint: str, method: str = "GET", data: dict = None, user_id: str = None):
    """Hace una petición al Groups Service"""
    groups_service_url = "http://agenda-groups-service:8003"

    headers = {"Content-Type": "application/json"}
    if user_id:
        headers["X-User-ID"] = user_id

    try:
        async with httpx.AsyncClient(timeout=30.0) as client:
            if method == "GET":
                response = await client.get(f"{groups_service_url}{endpoint}", headers=headers)
            elif method == "POST":
                response = await client.post(f"{groups_service_url}{endpoint}", json=data, headers=headers)
            elif method == "PUT":
                response = await client.put(f"{groups_service_url}{endpoint}", json=data, headers=headers)
            elif method == "DELETE":
                response = await client.delete(f"{groups_service_url}{endpoint}", headers=headers)
            else:
                raise HTTPException(status_code=500, detail=f"Unsupported method: {method}")

            return response

    except httpx.ConnectError:
        raise HTTPException(status_code=503, detail="Groups Service no disponible")
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Error interno: {str(e)}")

# Función auxiliar para obtener user_id (placeholder - debería venir de JWT)
def get_current_user_id() -> str:
    """Obtiene el ID del usuario actual - PLACEHOLDER"""
    # TODO: Implementar extracción de JWT
    return "user_test"

# ========== ENDPOINTS DE GRUPOS ==========

@router.post("", response_model=GroupResponse, status_code=status.HTTP_201_CREATED)
async def create_group(group_data: GroupCreateRequest):
    """Crear un nuevo grupo"""
    user_id = get_current_user_id()

    data = {
        "name": group_data.name,
        "description": group_data.description
    }

    response = await make_groups_service_request("/groups", "POST", data, user_id)

    if response.status_code == 201:
        return response.json()
    else:
        raise HTTPException(status_code=response.status_code, detail=response.text)

@router.get("", response_model=GroupListResponse)
async def list_user_groups(page: int = 1, page_size: int = 20):
    """Listar grupos del usuario actual"""
    user_id = get_current_user_id()

    endpoint = f"/groups/user/{user_id}?page={page}&page_size={page_size}"
    response = await make_groups_service_request(endpoint, "GET", user_id=user_id)

    if response.status_code == 200:
        data = response.json()
        return GroupListResponse(
            groups=data.get("groups", []),
            page=data.get("page", page),
            total=data.get("total", 0)
        )
    else:
        raise HTTPException(status_code=response.status_code, detail=response.text)

@router.get("/{group_id}", response_model=GroupResponse)
async def get_group(group_id: str):
    """Obtener detalles de un grupo"""
    user_id = get_current_user_id()

    response = await make_groups_service_request(f"/groups/{group_id}", "GET", user_id=user_id)

    if response.status_code == 200:
        return response.json()
    else:
        raise HTTPException(status_code=response.status_code, detail=response.text)

@router.put("/{group_id}", response_model=GroupResponse)
async def update_group(group_id: str, group_data: GroupCreateRequest):
    """Actualizar un grupo"""
    user_id = get_current_user_id()

    data = {
        "name": group_data.name,
        "description": group_data.description
    }

    response = await make_groups_service_request(f"/groups/{group_id}", "PUT", data, user_id)

    if response.status_code == 200:
        return response.json()
    else:
        raise HTTPException(status_code=response.status_code, detail=response.text)

@router.delete("/{group_id}", status_code=status.HTTP_204_NO_CONTENT)
async def delete_group(group_id: str):
    """Eliminar un grupo"""
    user_id = get_current_user_id()

    response = await make_groups_service_request(f"/groups/{group_id}", "DELETE", user_id=user_id)

    if response.status_code != 204:
        raise HTTPException(status_code=response.status_code, detail=response.text)

# ========== ENDPOINTS DE MIEMBROS ==========

@router.get("/{group_id}/members", response_model=dict)
async def list_group_members(group_id: str):
    """Listar miembros de un grupo"""
    user_id = get_current_user_id()

    response = await make_groups_service_request(f"/groups/{group_id}/members", "GET", user_id=user_id)

    if response.status_code == 200:
        return response.json()
    else:
        raise HTTPException(status_code=response.status_code, detail=response.text)

@router.post("/{group_id}/members", status_code=status.HTTP_201_CREATED)
async def add_group_member(group_id: str, member_data: dict):
    """Agregar un miembro a un grupo"""
    user_id = get_current_user_id()

    response = await make_groups_service_request(f"/groups/{group_id}/members", "POST", member_data, user_id)

    if response.status_code != 201:
        raise HTTPException(status_code=response.status_code, detail=response.text)

@router.delete("/{group_id}/members/{member_id}", status_code=status.HTTP_204_NO_CONTENT)
async def remove_group_member(group_id: str, member_id: str):
    """Remover un miembro de un grupo"""
    user_id = get_current_user_id()

    response = await make_groups_service_request(f"/groups/{group_id}/members/{member_id}", "DELETE", user_id=user_id)

    if response.status_code != 204:
        raise HTTPException(status_code=response.status_code, detail=response.text)

@router.get("/{group_id}/admins", response_model=List[MemberResponse])
async def get_group_admins(group_id: str):
    """Obtener administradores de un grupo"""
    user_id = get_current_user_id()

    response = await make_groups_service_request(f"/groups/{group_id}/members/admins", "GET", user_id=user_id)

    if response.status_code == 200:
        return response.json()
    else:
        raise HTTPException(status_code=response.status_code, detail=response.text)

# ========== ENDPOINTS DE INVITACIONES ==========

@router.post("/invitations", status_code=status.HTTP_201_CREATED)
async def create_invitation(invitation_data: InvitationCreateRequest):
    """Crear una invitación para unirse a un grupo"""
    user_id = get_current_user_id()

    data = {
        "group_id": invitation_data.group_id,
        "user_id": invitation_data.user_id
    }

    response = await make_groups_service_request("/invitations", "POST", data, user_id)

    if response.status_code != 201:
        raise HTTPException(status_code=response.status_code, detail=response.text)

@router.get("/invitations", response_model=List[InvitationResponse])
async def list_user_invitations():
    """Listar invitaciones del usuario actual"""
    user_id = get_current_user_id()

    response = await make_groups_service_request(f"/invitations/user/{user_id}", "GET", user_id=user_id)

    if response.status_code == 200:
        return response.json()
    else:
        raise HTTPException(status_code=response.status_code, detail=response.text)

@router.post("/invitations/{invitation_id}/respond", status_code=status.HTTP_200_OK)
async def respond_to_invitation(invitation_id: str, response_data: dict):
    """Responder a una invitación (aceptar/rechazar)"""
    user_id = get_current_user_id()

    response = await make_groups_service_request(f"/invitations/{invitation_id}/respond", "POST", response_data, user_id)

    if response.status_code != 200:
        raise HTTPException(status_code=response.status_code, detail=response.text)

# ========== ENDPOINTS DE EVENTOS DE GRUPOS ==========

@router.post("/{group_id}/events", status_code=status.HTTP_201_CREATED)
async def add_event_to_group(group_id: str, event_data: GroupEventRequest):
    """Agregar un evento a un grupo"""
    user_id = get_current_user_id()

    data = {
        "event_id": event_data.event_id,
        "added_by": user_id
    }

    response = await make_groups_service_request(f"/groups/{group_id}/events", "POST", data, user_id)

    if response.status_code != 201:
        raise HTTPException(status_code=response.status_code, detail=response.text)

@router.delete("/{group_id}/events/{event_id}", status_code=status.HTTP_204_NO_CONTENT)
async def remove_event_from_group(group_id: str, event_id: str):
    """Remover un evento de un grupo"""
    user_id = get_current_user_id()

    response = await make_groups_service_request(f"/groups/{group_id}/events/{event_id}", "DELETE", user_id=user_id)

    if response.status_code != 204:
        raise HTTPException(status_code=response.status_code, detail=response.text)

@router.get("/{group_id}/events", response_model=List[dict])
async def list_group_events(group_id: str):
    """Listar eventos de un grupo"""
    user_id = get_current_user_id()

    response = await make_groups_service_request(f"/groups/{group_id}/events", "GET", user_id=user_id)

    if response.status_code == 200:
        return response.json()
    else:
        raise HTTPException(status_code=response.status_code, detail=response.text)