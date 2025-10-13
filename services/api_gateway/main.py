import httpx
import json
import redis
from settings import settings
from routers import users_router, health_router, groups_router
from fastapi import FastAPI, HTTPException, BackgroundTasks
from pydantic import BaseModel
from typing import Optional, List, Dict
from datetime import datetime, timezone

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
app.include_router(groups_router.router)

# Almacenamiento temporal de respuestas de eventos
event_responses = {}

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

class EventDeleteRequest(BaseModel):
    event_id: str
    user_id: str

class GroupEventCreateRequest(BaseModel):
    group_id: str
    title: str
    description: str
    start_time: datetime
    end_time: datetime
    user_id: str

class GroupEventResponse(BaseModel):
    event_id: str
    group_id: str
    title: str
    description: str
    start_time: datetime
    end_time: datetime
    created_by: str
    created_at: datetime

# Función para procesar respuestas de eventos
def process_event_response(message):
    """Procesar respuestas del Events Service - ACTUALIZADA"""
    try:
        event_data = json.loads(message['data'])
        event_type = event_data.get('type')
        correlation_id = event_data.get('payload', {}).get('correlation_id')
        response_data = event_data.get('payload', {}).get('response', {})
        
        if correlation_id and response_data:
            event_responses[correlation_id] = {
                'success': response_data.get('success', False),
                'message': response_data.get('message', ''),
                'data': response_data,
                'timestamp': datetime.now(),
                'type': event_type  # 🔥 NUEVO: Guardar tipo de evento
            }
            print(f"✅ Respuesta procesada para {event_type} {correlation_id}: {response_data}")
    except Exception as e:
        print(f"❌ Error procesando respuesta de evento: {e}")

# Iniciar listener de respuestas en segundo plano
@app.on_event("startup")
async def startup_event():
    print(f"🚀 {settings.app_title} v{settings.app_version} iniciando...")
    
    # Suscribirse a respuestas de eventos
    if event_service.redis and event_service.redis.is_connected():
        try:
            # Usar el executor de threads para Redis (ya que no es async)
            import asyncio
            from concurrent.futures import ThreadPoolExecutor
            
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



@app.post("/api/v1/events")
async def create_event(event_data: EventCreateRequest):
    """Crear evento - Versión CORREGIDA que maneja zonas horarias"""
    
    print(f"🔍 Recibiendo evento: {event_data.title} para usuario {event_data.user_id}")
    
    # 1. Verificar conflictos DIRECTAMENTE con el Events Service
    try:
        # Obtener eventos existentes del usuario
        async with httpx.AsyncClient() as client:
            # Obtener eventos existentes del usuario
            events_response = await client.get(
                f"http://agenda-events-service:8002/api/events?user_id={event_data.user_id}",
                timeout=10.0
            )
        
        print(f"🔍 Response status de Events Service: {events_response.status_code}")
        
        if events_response.status_code == 200:
            events_data = events_response.json()
            existing_events = events_data.get('events', [])
            if existing_events: print(f"🔍 Eventos existentes: {len(existing_events)}")
            
            # ✅ CORRECCIÓN: Convertir las fechas del nuevo evento a UTC para comparar
            new_start_utc = event_data.start_time.replace(tzinfo=timezone.utc) if event_data.start_time.tzinfo is None else event_data.start_time
            new_end_utc = event_data.end_time.replace(tzinfo=timezone.utc) if event_data.end_time.tzinfo is None else event_data.end_time
            
            print(f"🔍 Nuevo evento - Inicio: {new_start_utc}, Fin: {new_end_utc}")
            
            # Inicializar lista de eventos existentes si es None
            if existing_events is None:
                existing_events = []
                print("ℹ️  No hay eventos existentes para verificar conflictos")
            
            conflicting_events = []
            for event in existing_events:
                # Verificar que el evento tenga los campos necesarios
                if not all(key in event for key in ['start_time', 'end_time']):
                    print(f"⚠️  Evento inválido: {event}")
                    continue
                    
                # Convertir fechas existentes a datetime con zona horaria
                existing_start_str = event['start_time'].replace('Z', '+00:00')
                existing_end_str = event['end_time'].replace('Z', '+00:00')
                
                existing_start = datetime.fromisoformat(existing_start_str)
                existing_end = datetime.fromisoformat(existing_end_str)
                
                # Asegurar que las fechas existentes tengan zona horaria UTC
                if existing_start.tzinfo is None:
                    existing_start = existing_start.replace(tzinfo=timezone.utc)
                if existing_end.tzinfo is None:
                    existing_end = existing_end.replace(tzinfo=timezone.utc)
                
                print(f"🔍 Comparando con evento: {event['title']} ({existing_start} - {existing_end})")
            
                overlap_condition_1 = new_start_utc < existing_end and new_end_utc > existing_start
                overlap_condition_2 = new_start_utc >= existing_start and new_start_utc < existing_end
                overlap_condition_3 = new_end_utc > existing_start and new_end_utc <= existing_end
                
                has_overlap = overlap_condition_1 or overlap_condition_2 or overlap_condition_3
                
                if has_overlap:
                    print(f"🚫 CONFLICTO detectado con: {event['title']}")
                    conflicting_events.append({
                        "id": event["id"],
                        "title": event["title"], 
                        "start_time": event["start_time"],
                        "end_time": event["end_time"],
                        "description": event.get("description", "")
                    })
            
            print(f"🔍 Total eventos conflictivos: {len(conflicting_events)}")
            
            if conflicting_events:
                return {
                    "status": "error",
                    "message": f"Conflicto de horario detectado con {len(conflicting_events)} evento(s)",
                    "conflicting_events": conflicting_events,
                    "event_id": None,
                    "timestamp": datetime.now().isoformat()
                }
            else:
                print("✅ No hay conflictos de horario")
                
    except Exception as e:
        print(f"❌ Error verificando conflictos: {e}")
        import traceback
        print(f"❌ Traceback: {traceback.format_exc()}")
        return {
            "status": "error", 
            "message": f"Error verificando disponibilidad: {str(e)}",
            "conflicting_events": [],
            "event_id": None,
            "timestamp": datetime.now().isoformat()
        }
    
    # 2. Si no hay conflictos, publicar evento en Redis
    print("✅ Publicando evento en Redis...")
    
    result = event_service.publish_event(
        channel="events_events",
        event_type="event_creation_requested",
        payload={
            "title": event_data.title,
            "description": event_data.description, 
            "start_time": event_data.start_time.replace(tzinfo=timezone.utc).isoformat(),
            "end_time": event_data.end_time.replace(tzinfo=timezone.utc).isoformat(),
            "user_id": event_data.user_id
        }
    )
    
    if not result.get("published", False):
        raise HTTPException(status_code=503, detail="Message bus unavailable")
    
    print("✅ Evento publicado exitosamente en Redis")
    
    return {
        "status": "success",
        "message": "Evento creado exitosamente",
        "event_id": result["event_id"],
        "timestamp": result["timestamp"]
    }






# Agregar después del endpoint POST /api/v1/events
@app.delete("/api/v1/events/{event_id}")
async def delete_event(event_id: str, user_id: str):
    """Eliminar un evento específico"""
    print(f"🔍 Solicitando eliminación del evento {event_id} para usuario {user_id}")
    
    # Publicar evento de eliminación en Redis
    result = event_service.publish_event(
        channel="events_events",
        event_type="event_deletion_requested", 
        payload={
            "event_id": event_id,
            "user_id": user_id
        }
    )
    
    if not result.get("published", False):
        raise HTTPException(
            status_code=503, 
            detail=result.get("error", "Message bus unavailable")
        )
    
    return {
        "status": "processing",
        "message": "Event deletion request received and queued",
        "event_id": event_id,
        "timestamp": datetime.now().isoformat()
    }




# Endpoint para verificar estado de evento
@app.get("/api/v1/events/{event_id}/status")
async def get_event_status(event_id: str):
    """Obtener el estado de un evento específico"""
    if event_id in event_responses:
        response = event_responses[event_id]
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
            "message": "Event still being processed"
        }


@app.get("/api/v1/events", response_model=EventListResponse)
async def get_events(user_id: Optional[str] = None, limit: int = 50, offset: int = 0):
    """Obtener eventos del Events Service real"""
    
    try:
        # Construir URL para el Events Service
        events_service_url = "http://agenda-events-service:8002/api/events"
        params = {}
        
        if user_id:
            params["user_id"] = user_id
        if limit != 50:  # Solo agregar si no es el valor por defecto
            params["limit"] = limit
        if offset != 0:  # Solo agregar si no es el valor por defecto
            params["offset"] = offset
        
        # Hacer request al Events Service
        async with httpx.AsyncClient() as client:
            response = await client.get(events_service_url, params=params, timeout=30.0)
            
            if response.status_code == 200:
                data = response.json()
                
                # Convertir los eventos al formato esperado
                events = []
                for event_data in data.get("events", []):
                    events.append(EventResponse(
                        id=event_data["id"],
                        title=event_data["title"],
                        description=event_data["description"],
                        start_time=datetime.fromisoformat(event_data["start_time"].replace('Z', '+00:00')),
                        end_time=datetime.fromisoformat(event_data["end_time"].replace('Z', '+00:00')),
                        user_id=event_data["user_id"],
                        created_at=datetime.fromisoformat(event_data["created_at"].replace('Z', '+00:00'))
                    ))
                
                return EventListResponse(
                    events=events,
                    total=data.get("total", 0)
                )
            else:
                raise HTTPException(
                    status_code=response.status_code,
                    detail=f"Error del Events Service: {response.text}"
                )
                
    except httpx.ConnectError:
        raise HTTPException(
            status_code=503,
            detail="Events Service no disponible"
        )
    except Exception as e:
        raise HTTPException(
            status_code=500,
            detail=f"Error interno: {str(e)}"
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

# ========== ENDPOINTS PARA EVENTOS DE GRUPOS ==========

@app.post("/api/v1/group-events")
async def create_group_event(group_event_data: GroupEventCreateRequest):
    """Crear evento de grupo - DIFERENTE a eventos individuales"""
    print(f"🔍 Creando evento de grupo: {group_event_data.title} en grupo {group_event_data.group_id}")

    # 1. Verificar que el usuario es miembro del grupo
    try:
        async with httpx.AsyncClient() as client:
            group_response = await client.get(
                f"http://agenda-groups-service:8003/groups/{group_event_data.group_id}",
                headers={"X-User-ID": group_event_data.user_id},
                timeout=10.0
            )

        if group_response.status_code != 200:
            return {
                "status": "error",
                "message": "Usuario no autorizado para crear eventos en este grupo",
                "event_id": None,
                "timestamp": datetime.now().isoformat()
            }

    except Exception as e:
        print(f"❌ Error verificando membresía del grupo: {e}")
        return {
            "status": "error",
            "message": f"Error verificando permisos del grupo: {str(e)}",
            "event_id": None,
            "timestamp": datetime.now().isoformat()
        }

    # 2. Verificar conflictos de horario para el grupo
    try:
        # Obtener miembros del grupo
        async with httpx.AsyncClient() as client:
            members_response = await client.get(
                f"http://agenda-groups-service:8003/groups/{group_event_data.group_id}/members",
                headers={"X-User-ID": group_event_data.user_id},
                timeout=10.0
            )

        if members_response.status_code == 200:
            members_data = members_response.json()
            group_members = members_data.get("members", [])

            # Verificar conflictos para cada miembro del grupo
            conflicting_members = []
            for member in group_members:
                member_id = member["user_id"]

                # Obtener eventos del miembro
                events_response = await make_api_request(f"/api/v1/events?user_id={member_id}")

                if events_response and events_response.status_code == 200:
                    events_data = events_response.json()
                    member_events = events_data.get('events', [])

                    # Verificar conflictos
                    new_start_utc = group_event_data.start_time.replace(tzinfo=timezone.utc) if group_event_data.start_time.tzinfo is None else group_event_data.start_time
                    new_end_utc = group_event_data.end_time.replace(tzinfo=timezone.utc) if group_event_data.end_time.tzinfo is None else group_event_data.end_time

                    for event in member_events:
                        existing_start_str = event['start_time'].replace('Z', '+00:00')
                        existing_end_str = event['end_time'].replace('Z', '+00:00')

                        existing_start = datetime.fromisoformat(existing_start_str)
                        existing_end = datetime.fromisoformat(existing_end_str)

                        if existing_start.tzinfo is None:
                            existing_start = existing_start.replace(tzinfo=timezone.utc)
                        if existing_end.tzinfo is None:
                            existing_end = existing_end.replace(tzinfo=timezone.utc)

                        # Verificar solapamiento
                        overlap = (new_start_utc < existing_end and new_end_utc > existing_start)
                        if overlap:
                            conflicting_members.append({
                                "member_id": member_id,
                                "conflicting_event": {
                                    "id": event["id"],
                                    "title": event["title"],
                                    "start_time": event["start_time"],
                                    "end_time": event["end_time"]
                                }
                            })

            if conflicting_members:
                return {
                    "status": "error",
                    "message": f"Conflicto de horario detectado para {len(conflicting_members)} miembro(s) del grupo",
                    "conflicting_members": conflicting_members,
                    "event_id": None,
                    "timestamp": datetime.now().isoformat()
                }

    except Exception as e:
        print(f"❌ Error verificando conflictos de grupo: {e}")
        import traceback
        print(f"❌ Traceback: {traceback.format_exc()}")

    # 3. Crear el evento individual para cada miembro del grupo
    created_events = []
    failed_members = []

    for member in group_members:
        member_id = member["user_id"]

        # Crear evento individual para este miembro
        individual_event = {
            "title": f"[GRUPO] {group_event_data.title}",
            "description": f"Evento de grupo '{group_event_data.group_id}': {group_event_data.description}",
            "start_time": group_event_data.start_time.isoformat(),
            "end_time": group_event_data.end_time.isoformat(),
            "user_id": member_id
        }

        try:
            event_response = await make_api_request("/api/v1/events", "POST", individual_event)

            if event_response and event_response.status_code == 200:
                event_result = event_response.json()
                if event_result.get("status") == "success":
                    created_events.append({
                        "member_id": member_id,
                        "event_id": event_result["event_id"]
                    })
                else:
                    failed_members.append({
                        "member_id": member_id,
                        "error": event_result.get("message", "Error desconocido")
                    })
            else:
                failed_members.append({
                    "member_id": member_id,
                    "error": "Error en la respuesta del servicio"
                })

        except Exception as e:
            failed_members.append({
                "member_id": member_id,
                "error": str(e)
            })

    # 4. Registrar el evento en el grupo (para seguimiento)
    try:
        for created_event in created_events:
            await make_groups_service_request(
                f"/groups/{group_event_data.group_id}/events",
                "POST",
                {"event_id": created_event["event_id"]},
                group_event_data.user_id
            )
    except Exception as e:
        print(f"⚠️ Error registrando evento en grupo: {e}")

    # 5. Retornar resultado
    if failed_members:
        return {
            "status": "partial_success",
            "message": f"Evento creado para {len(created_events)} miembros, falló para {len(failed_members)}",
            "created_events": created_events,
            "failed_members": failed_members,
            "timestamp": datetime.now().isoformat()
        }
    else:
        return {
            "status": "success",
            "message": f"Evento de grupo creado exitosamente para {len(created_events)} miembros",
            "created_events": created_events,
            "timestamp": datetime.now().isoformat()
        }

# Función auxiliar para hacer requests internos al API Gateway
async def make_api_request(endpoint: str, method: str = "GET", data: dict = None):
    """Hace una petición interna al API Gateway"""
    api_gateway_url = "http://localhost:8000"  # URL interna del API Gateway

    headers = {"Content-Type": "application/json"}

    try:
        async with httpx.AsyncClient(timeout=30.0) as client:
            if method == "GET":
                response = await client.get(f"{api_gateway_url}{endpoint}", headers=headers)
            elif method == "POST":
                response = await client.post(f"{api_gateway_url}{endpoint}", json=data, headers=headers)
            elif method == "PUT":
                response = await client.put(f"{api_gateway_url}{endpoint}", json=data, headers=headers)
            elif method == "DELETE":
                response = await client.delete(f"{api_gateway_url}{endpoint}", headers=headers)
            else:
                raise HTTPException(status_code=500, detail=f"Unsupported method: {method}")

            return response

    except httpx.ConnectError:
        raise HTTPException(status_code=503, detail="API Gateway no disponible")
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Error interno: {str(e)}")

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