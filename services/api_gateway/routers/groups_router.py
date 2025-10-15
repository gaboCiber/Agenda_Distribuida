from fastapi import APIRouter, HTTPException, status, Depends, Path
from pydantic import BaseModel
from typing import Optional, List
import httpx
from datetime import datetime
from fastapi import Request
import uuid
from services.event_service import event_service
import asyncio
import json
import time

# Importar group_responses desde el módulo de almacenamiento
from services.response_store import group_responses

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

    # Asegurarse de que siempre haya un user_id
    if not user_id:
        raise ValueError("Se requiere user_id para autenticación con el servicio de grupos")
        
    headers = {
        "Content-Type": "application/json",
        "X-User-ID": user_id  # Siempre incluir el user_id en los headers
    }
    
    print(f"🔧 Enviando {method} a {groups_service_url}{endpoint} con headers: {headers}")

    try:
        async with httpx.AsyncClient(timeout=30.0) as client:
            if method == "GET":
                # CORRECCIÓN: No pasar data en GET, solo headers
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



# Función auxiliar para obtener user_id desde el header X-User-ID
async def get_current_user_id(request: Request):
    """Obtiene el ID del usuario actual desde el header X-User-ID"""
    user_id = request.headers.get('X-User-ID')
    if not user_id:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Se requiere autenticación"
        )
    return user_id
    
# ========== ENDPOINTS DE GRUPOS ==========

@router.post("", response_model=dict, status_code=status.HTTP_202_ACCEPTED)
async def create_group(group_data: GroupCreateRequest, request: Request):
    """Crear un nuevo grupo - Pub/Sub"""
    user_id = await get_current_user_id(request)

    # Publicar evento en Redis
    result = event_service.publish_event(
        channel="groups",
        event_type="group_created",
        payload={
            "name": group_data.name,
            "description": group_data.description,
            "created_by": user_id,
            "is_hierarchical": False,
            "parent_group_id": None,
            "response_channel": "groups",
            "source": "api_gateway"
        }
    )

    if not result.get("published", False):
        raise HTTPException(status_code=503, detail="Message bus unavailable")

    return {
        "status": "processing",
        "message": "Group creation request received and queued",
        "event_id": result["event_id"],
        "timestamp": result["timestamp"]
    }

# @router.get("", response_model=GroupListResponse)
# async def list_user_groups(request: Request, page: int = 1, page_size: int = 20):
#     """Listar grupos del usuario actual - HTTP directo (lectura)"""
    
#     user_id = await get_current_user_id(request)

#     Publicar evento en Redis
#     result = event_service.publish_event(
#         channel="groups",
#         event_type="list_user_groups",
#         payload={
#             "user_id": user_id
#         }
#     )

#     if not result.get("published", False):
#         raise HTTPException(status_code=503, detail="Message bus unavailable")

#     return {
#         "status": "processing",
#         "message": "Group list request received and queued",
#         "event_id": result["event_id"],
#         "timestamp": result["timestamp"]
#     }
    
#     try:
#         # Obtener el ID del usuario autenticado
#         user_id = await get_current_user_id(request)

#         # Construir la URL con los parámetros de paginación
#         endpoint = f"/groups/user/{user_id}?page={page}&page_size={page_size}"

#         print(f"🔍 Solicitando grupos para el usuario {user_id}")

#         # Hacer la petición al servicio de grupos
#         response = await make_groups_service_request(endpoint, "GET", user_id=user_id)

#         # Si la respuesta es exitosa
#         if response.status_code == 200:
#             data = response.json()
#             print(f"✅ Respuesta del servicio de grupos: {data}")
#             return GroupListResponse(
#                 groups=data.get("groups", []),
#                 page=data.get("page", page),
#                 total=data.get("total", 0)
#             )

#         # Si no hay grupos, devolver lista vacía
#         print(f"⚠️ No se encontraron grupos para el usuario {user_id}")
#         return GroupListResponse(groups=[], page=page, total=0)

#     except Exception as e:
#         print(f"❌ Error al listar grupos: {str(e)}")
@router.get("", response_model=GroupListResponse)
async def list_user_groups(request: Request, page: int = 1, page_size: int = 20):
    """
    Lista los grupos del usuario actual usando el patrón Pub/Sub de Redis.
    
    1. Publica un mensaje en el canal 'groups' solicitando los grupos
    2. Espera la respuesta en un canal específico
    3. Devuelve la respuesta recibida
    """
    try:
        # Obtener el ID del usuario autenticado
        user_id = await get_current_user_id(request)
        
        # Generar un ID de correlación único
        correlation_id = str(uuid.uuid4())
        response_channel = f"groups:response:{correlation_id}"
        
        # 1. Publicar el mensaje de solicitud usando el servicio de eventos
        message = {
            "event_id": str(uuid.uuid4()),
            "type": "list_user_groups",
            "timestamp": datetime.utcnow().isoformat() + "Z",
            "version": "1.0",
            "payload": {
                "user_id": user_id,
                "page": page,
                "page_size": page_size,
                "correlation_id": correlation_id,
                "response_channel": response_channel
            }
        }
        
        # Publicar el mensaje en Redis usando el servicio de eventos
        result = await event_service.redis.publish_event(
            "groups",
            json.dumps(message)
        )
        
        if not result:
            print(f"❌ No se pudo publicar el mensaje en Redis")
            return GroupListResponse(groups=[], page=page, total=0)
            
        print(f"📤 Publicada solicitud de listado de grupos (ID: {correlation_id}) en canal 'groups'")
        print(f"🔄 Esperando respuesta en canal: {response_channel}")
        
        # 2. Configurar el manejador de respuesta
        response_received = None
        
        # 3. Crear un nuevo cliente Redis para esta solicitud
        try:
            # Crear un nuevo cliente Redis
            redis_client = event_service.redis.client
            pubsub = redis_client.pubsub()
            
            # Suscribirse al canal de respuesta
            await pubsub.subscribe(response_channel)
            print(f"🔔 Suscrito al canal: {response_channel}")
            
            # Esperar mensaje de confirmación de suscripción
            sub_message = await pubsub.get_message(ignore_subscribe_messages=False, timeout=5.0)
            if not sub_message or sub_message['type'] != 'subscribe':
                print("❌ Error al suscribirse al canal de respuesta")
                await pubsub.close()
                return GroupListResponse(groups=[], page=page, total=0)
                
            # Publicar el mensaje después de suscribirse
            result = await redis_client.publish("groups", json.dumps(message))
            if not result:
                print("❌ No se pudo publicar el mensaje en Redis")
                await pubsub.close()
                return GroupListResponse(groups=[], page=page, total=0)
                
            print(f"📤 Publicada solicitud de listado de grupos (ID: {correlation_id}) en canal 'groups'")
            
            # Esperar la respuesta
            start_time = time.time()
            while time.time() - start_time < 10.0:  # Timeout de 10 segundos
                try:
                    message = await pubsub.get_message(ignore_subscribe_messages=True, timeout=1.0)
                    if message and message.get('type') == 'message':
                        try:
                            data = json.loads(message["data"])
                            print(f"📥 Mensaje recibido: {data}")
                            if data.get("type") == "list_user_groups_response":
                                payload = data.get("payload", {})
                                # El payload ya es el diccionario que necesitamos
                                print(f"✅ Respuesta recibida para la solicitud {correlation_id}")
                                response_received = payload
                                break
                        except Exception as e:
                            print(f"❌ Error procesando mensaje: {e}")
                            continue
                except asyncio.TimeoutError:
                    continue
            
            # 4. Procesar la respuesta
            if response_received:
                try:
                    # El payload ya está parseado, extraer los grupos directamente
                    groups = response_received.get("groups", [])
                    total = response_received.get("total", len(groups))
                    
                    # Convertir los grupos al formato de respuesta
                    valid_groups = []
                    for group in groups:
                        try:
                            # Convertir las fechas de string a datetime
                            created_at = group.get("created_at")
                            if isinstance(created_at, str):
                                created_at = datetime.fromisoformat(created_at.replace('Z', '+00:00'))
                            
                            updated_at = group.get("updated_at")
                            if isinstance(updated_at, str):
                                updated_at = datetime.fromisoformat(updated_at.replace('Z', '+00:00'))
                            
                            valid_group = GroupResponse(
                                id=group.get("id"),
                                name=group.get("name", "Sin nombre"),
                                description=group.get("description"),
                                created_by=group.get("created_by"),
                                created_at=created_at,
                                updated_at=updated_at,
                                member_count=group.get("member_count", 0)
                            )
                            valid_groups.append(valid_group)
                        except Exception as e:
                            print(f"⚠️ Error procesando grupo: {e}")
                            import traceback
                            print(f"🔍 Stack trace: {traceback.format_exc()}")
                            continue
                    
                    print(f"✅ Obtenidos {len(valid_groups)} grupos (de {total} totales)")
                    return GroupListResponse(
                        groups=valid_groups,
                        page=page,
                        total=total
                    )
                except Exception as e:
                    print(f"❌ Error procesando la respuesta: {e}")
                    import traceback
                    print(f"🔍 Stack trace: {traceback.format_exc()}")
                    return GroupListResponse(groups=[], page=page, total=0)
            else:
                print("⚠️ No se recibió respuesta del servicio de grupos")
                return GroupListResponse(groups=[], page=page, total=0)
                
        except Exception as e:
            print(f"❌ Error en el manejador de mensajes: {e}")
            import traceback
            print(f"🔍 Stack trace: {traceback.format_exc()}")
            return GroupListResponse(groups=[], page=page, total=0)
            
        finally:
            try:
                if 'pubsub' in locals():
                    await pubsub.unsubscribe(response_channel)
                    await pubsub.close()
                    print(f"👋 Desconectado del canal: {response_channel}")
            except Exception as e:
                print(f"⚠️ Error al cerrar la conexión: {e}")
            
    except Exception as e:
        import traceback
        print(f"❌ Error en list_user_groups: {str(e)}")
        print(f"🔍 Stack trace: {traceback.format_exc()}")
        return GroupListResponse(groups=[], page=page, total=0)

@router.get("/{group_id}", response_model=GroupResponse)
async def get_group(group_id: str, request: Request):
    
    # Publicar evento en Redis
    result = event_service.publish_event(
        channel="groups",
        event_type="get_group",
        payload={
            "group_id": group_id
        }
    )

    if not result.get("published", False):
        raise HTTPException(status_code=503, detail="Message bus unavailable")

    return {
        "status": "processing",
        "message": "Group get request received and queued",
        "event_id": result["event_id"],
        "timestamp": result["timestamp"]
    }

    # """Obtener detalles de un grupo"""
    # user_id = await get_current_user_id(request)

    # response = await make_groups_service_request(f"/groups/{group_id}", "GET", user_id=user_id)

    # if response.status_code == 200:
    #     return response.json()
    # else:
    #     raise HTTPException(status_code=response.status_code, detail=response.text)

@router.put("/{group_id}", response_model=GroupResponse)
async def update_group(group_id: str, group_data: GroupCreateRequest, request: Request):
    
    # Publicar evento en Redis
    result = event_service.publish_event(
        channel="groups",
        event_type="update_group",
        payload={
            "group_id": group_id,
            "group_data": group_data.dict()
        }
    )

    if not result.get("published", False):
        raise HTTPException(status_code=503, detail="Message bus unavailable")

    return {
        "status": "processing",
        "message": "Group update request received and queued",
        "event_id": result["event_id"],
        "timestamp": result["timestamp"]
    }

    # """Actualizar un grupo"""
    # user_id = await get_current_user_id(request)

    # data = {
    #     "name": group_data.name,
    #     "description": group_data.description
    # }

    # response = await make_groups_service_request(f"/groups/{group_id}", "PUT", data, user_id)

    # if response.status_code == 200:
    #     return response.json()
    # else:
    #     raise HTTPException(status_code=response.status_code, detail=response.text)

@router.delete("/{group_id}", status_code=status.HTTP_204_NO_CONTENT)
async def delete_group(group_id: str, request: Request):
    
    user_id = await get_current_user_id(request)

    # Publicar evento en Redis
    result = event_service.publish_event(
        channel="groups",
        event_type="group_deleted",
        payload={
            "group_id": group_id,
            "deleted_by": user_id,
            "response_channel": "groups",
            "source": "api_gateway"
        }
    )
    
    if not result.get("published", False):
        raise HTTPException(status_code=503, detail="Message bus unavailable")

    return {
        "status": "processing",
        "message": "Group delete request received and queued",
        "event_id": result["event_id"],
        "timestamp": result["timestamp"]
    }
    
    # """Eliminar un grupo"""
    # user_id = await get_current_user_id(request)

    # response = await make_groups_service_request(f"/groups/{group_id}", "DELETE", user_id=user_id)

    # if response.status_code != 204:
    #     raise HTTPException(status_code=response.status_code, detail=response.text)

# ========== ENDPOINTS DE MIEMBROS ==========

@router.get("/{group_id}/members", response_model=dict)
async def list_group_members(group_id: str, request: Request):
    """Listar miembros de un grupo"""
    user_id = await get_current_user_id(request)

    response = await make_groups_service_request(f"/groups/{group_id}/members", "GET", user_id=user_id)

    if response.status_code == 200:
        return response.json()
    else:
        raise HTTPException(status_code=response.status_code, detail=response.text)

@router.post("/{group_id}/members", status_code=status.HTTP_201_CREATED)
async def add_group_member(group_id: str, member_data: dict, request: Request):
    
    user_id = await get_current_user_id(request)

    # Publicar evento en Redis
    result = event_service.publish_event(
        channel="groups",
        event_type="member_added",
        payload={
            "group_id": group_id,
            "userID": member_data["userID"],
            "role": member_data["role"],
            "added_by": user_id,
            "response_channel": "groups",
            "source": "api_gateway"
        }
    )
    
    if not result.get("published", False):
        raise HTTPException(status_code=503, detail="Message bus unavailable")

    return {
        "status": "processing",
        "message": "Group member add request received and queued",
        "event_id": result["event_id"],
        "timestamp": result["timestamp"]
    }
   
   
    # """Agregar un miembro a un grupo"""
    # user_id = await get_current_user_id(request)

    # response = await make_groups_service_request(f"/groups/{group_id}/members", "POST", member_data, user_id)

    # if response.status_code != 201:
    #     raise HTTPException(status_code=response.status_code, detail=response.text)

@router.delete("/{group_id}/members/{member_id}", status_code=status.HTTP_204_NO_CONTENT)
async def remove_group_member(group_id: str, member_id: str, request: Request):
    """Remover un miembro de un grupo"""
    user_id = await get_current_user_id(request)

    response = await make_groups_service_request(f"/groups/{group_id}/members/{member_id}", "DELETE", user_id=user_id)

    if response.status_code != 204:
        raise HTTPException(status_code=response.status_code, detail=response.text)

@router.get("/{group_id}/admins", response_model=List[MemberResponse])
async def get_group_admins(group_id: str, request: Request):
    """Obtener administradores de un grupo"""
    user_id = await get_current_user_id(request)

    response = await make_groups_service_request(f"/groups/{group_id}/members/admins", "GET", user_id=user_id)

    if response.status_code == 200:
        return response.json()
    else:
        raise HTTPException(status_code=response.status_code, detail=response.text)

# ========== ENDPOINTS DE INVITACIONES ==========

@router.post("/invitations", status_code=status.HTTP_202_ACCEPTED)
async def create_invitation(invitation_data: InvitationCreateRequest, request: Request):
    """Crear una invitación para unirse a un grupo - Pub/Sub"""
    user_id = await get_current_user_id(request)

    # Publicar evento en Redis
    result = event_service.publish_event(
        channel="groups",
        event_type="invitation_created",
        payload={
            "invitation_id": str(uuid.uuid4()),
            "group_id": invitation_data.group_id,
            "user_id": invitation_data.user_id,
            "invited_by": user_id,
            "response_channel": "groups_responses"
        }
    )

    if not result.get("published", False):
        raise HTTPException(status_code=503, detail="Message bus unavailable")

    return {
        "status": "processing",
        "message": "Invitation creation request received and queued",
        "event_id": result["event_id"],
        "timestamp": result["timestamp"]
    }

@router.get("/invitations/{invitation_id}/status")
async def get_invitation_status(invitation_id: str):
    """Obtener el estado de una invitación específica"""
    if invitation_id in group_responses:
        response = group_responses[invitation_id]
        return {
            "status": "completed",
            "success": response['success'],
            "message": response['message'],
            "data": response['data'],
            "timestamp": response['timestamp']
        }
    else:
        return {
            "status": "processing",
            "message": "Invitation still being processed"
        }

@router.get("/invitations", response_model=List[InvitationResponse])
async def list_user_invitations(request: Request):
    """Listar invitaciones del usuario actual - HTTP directo (lectura)"""
    print("🚀🚀🚀 CAMBIOS APLICADOS: Endpoint GET /api/v1/groups/invitations corregido 🚀🚀🚀")

    user_id = await get_current_user_id(request)

    print(f"🔍 DEBUG: Listando invitaciones para usuario {user_id}")

    # CORRECCIÓN: El endpoint correcto en Groups Service es /invitations con X-User-ID header
    response = await make_groups_service_request("/invitations", "GET", user_id=user_id)

    print(f"🔍 DEBUG: Respuesta del Groups Service: {response.status_code} - {response.text}")

    if response.status_code == 200:
        data = response.json()
        print(f"🔍 DEBUG: Datos de invitaciones: {data}")
        return data
    else:
        # Si no hay invitaciones, devolver una lista vacía en lugar de error 404
        if response.status_code == 404:
            print("🔍 DEBUG: No hay invitaciones (404), devolviendo lista vacía")
            return []
        print(f"❌ DEBUG: Error del Groups Service: {response.status_code} - {response.text}")
        raise HTTPException(status_code=response.status_code, detail=response.text)

@router.post("/invitations/{invitation_id}/respond", status_code=status.HTTP_200_OK)
async def respond_to_invitation(invitation_id: str, response_data: dict, request: Request):
    """Responder a una invitación (aceptar/rechazar)"""
    user_id = await get_current_user_id(request)

    response = await make_groups_service_request(f"/invitations/{invitation_id}/respond", "POST", response_data, user_id)

    if response.status_code != 200:
        raise HTTPException(status_code=response.status_code, detail=response.text)

# ========== ENDPOINTS DE USUARIOS ==========

@router.get("/users/{user_id}/groups", response_model=GroupListResponse)
async def list_groups_for_user(
    user_id: str = Path(..., description="ID del usuario cuyos grupos se quieren listar"),
    page: int = 1,
    page_size: int = 20,
    request: Request = None
):
    """Listar grupos de un usuario específico"""
    try:
        # Construir la URL con los parámetros de paginación
        endpoint = f"/groups/user/{user_id}?page={page}&page_size={page_size}"

        print(f"🔍 Solicitando grupos para el usuario {user_id}")

        # Hacer la petición al servicio de grupos
        response = await make_groups_service_request(endpoint, "GET", user_id=user_id)

        # Si la respuesta es exitosa
        if response.status_code == 200:
            data = response.json()
            print(f"✅ Respuesta del servicio de grupos: {data}")
            return GroupListResponse(
                groups=data.get("groups", []),
                page=data.get("page", page),
                total=data.get("total", 0)
            )

        # Si hay un error
        print(f"❌ Error del servicio de grupos: {response.text}")
        raise HTTPException(status_code=response.status_code, detail=response.text)

    except Exception as e:
        print(f"❌ Error al listar grupos: {str(e)}")
        raise HTTPException(status_code=500, detail=str(e))

# ========== ENDPOINTS DE EVENTOS DE GRUPOS ==========

@router.post("/{group_id}/events", status_code=status.HTTP_201_CREATED)
async def add_event_to_group(group_id: str, event_data: GroupEventRequest, request: Request):
    """Agregar un evento a un grupo"""
    user_id = await get_current_user_id(request)

    data = {
        "event_id": event_data.event_id,
        "added_by": user_id
    }

    response = await make_groups_service_request(f"/groups/{group_id}/events", "POST", data, user_id)

    if response.status_code != 201:
        raise HTTPException(status_code=response.status_code, detail=response.text)

@router.delete("/{group_id}/events/{event_id}", status_code=status.HTTP_204_NO_CONTENT)
async def remove_event_from_group(group_id: str, event_id: str, request: Request):
    """Remover un evento de un grupo"""
    user_id = await get_current_user_id(request)

    response = await make_groups_service_request(f"/groups/{group_id}/events/{event_id}", "DELETE", user_id=user_id)

    if response.status_code != 204:
        raise HTTPException(status_code=response.status_code, detail=response.text)

@router.get("/{group_id}/events", response_model=List[dict])
async def list_group_events(group_id: str, request: Request):
    """Listar eventos de un grupo"""
    user_id = await get_current_user_id(request)

    response = await make_groups_service_request(f"/groups/{group_id}/events", "GET", user_id=user_id)

    if response.status_code == 200:
        return response.json()
    else:
        raise HTTPException(status_code=response.status_code, detail=response.text)