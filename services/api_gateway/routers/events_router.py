# Router para endpoints de eventos individuales
from fastapi import APIRouter, HTTPException
from datetime import datetime, timezone
from typing import Optional

from models import (
    EventCreateRequest,
    EventResponse,
    EventListResponse,
    EventDeleteRequest
)
from services.event_service import event_service
from services.http_client import make_events_service_request
from services.event_handlers import get_event_response

router = APIRouter(prefix="/api/v1/events", tags=["events"])

@router.post("", response_model=dict)
async def create_event(event_data: EventCreateRequest):
    """Crear evento - Versión CORREGIDA que maneja zonas horarias"""

    print(f"🔍 Recibiendo evento: {event_data.title} para usuario {event_data.user_id}")

    # 1. Verificar conflictos DIRECTAMENTE con el Events Service
    try:
        # Obtener eventos existentes del usuario
        events_response = await make_events_service_request(
            f"/api/events?user_id={event_data.user_id}",
            "GET"
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

@router.delete("/{event_id}")
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

@router.get("/{event_id}/status")
async def get_event_status(event_id: str):
    """Obtener el estado de un evento específico"""
    return get_event_response(event_id)

@router.get("", response_model=EventListResponse)
async def get_events(user_id: Optional[str] = None, limit: int = 50, offset: int = 0):
    """Obtener eventos del Events Service real"""
    print(f"🎯 API_GATEWAY: get_events() llamado con user_id={user_id}, limit={limit}, offset={offset}")

    try:
        # Construir URL para el Events Service
        events_service_url = "/api/events"
        params = {}

        if user_id:
            params["user_id"] = user_id
            print(f"🔍 API_GATEWAY: Filtrando por user_id: {user_id}")
        if limit != 50:  # Solo agregar si no es el valor por defecto
            params["limit"] = limit
        if offset != 0:  # Solo agregar si no es el valor por defecto
            params["offset"] = offset

        print(f"🌐 API_GATEWAY: Llamando a Events Service: {events_service_url} con params: {params}")

        # Hacer request al Events Service
        response = await make_events_service_request(events_service_url, "GET", params=params)

        print(f"📡 API_GATEWAY: Respuesta de Events Service - Status: {response.status_code}")

        if response.status_code == 200:
            data = response.json()
            events_raw = data.get("events", [])
            print(f"📊 API_GATEWAY: Events Service devolvió {len(events_raw)} eventos")

            # Convertir los eventos al formato esperado
            events = []
            for event_data in events_raw:
                event_user_id = event_data.get("user_id")
                print(f"🔍 API_GATEWAY: Procesando evento {event_data['id']} - Usuario: {event_user_id}")

                # ⚠️ VERIFICACIÓN DE SEGURIDAD: Solo devolver eventos del usuario solicitado
                if user_id and event_user_id != user_id:
                    print(f"🚨 API_GATEWAY: ¡SEGURIDAD COMPROMETIDA! Evento {event_data['id']} pertenece a {event_user_id}, pero se pidió {user_id}")
                    continue  # Saltar este evento

                events.append(EventResponse(
                    id=event_data["id"],
                    title=event_data["title"],
                    description=event_data["description"],
                    start_time=datetime.fromisoformat(event_data["start_time"].replace('Z', '+00:00')),
                    end_time=datetime.fromisoformat(event_data["end_time"].replace('Z', '+00:00')),
                    user_id=event_user_id,
                    created_at=datetime.fromisoformat(event_data["created_at"].replace('Z', '+00:00'))
                ))

            print(f"✅ API_GATEWAY: Devolviendo {len(events)} eventos filtrados para usuario {user_id}")

            return EventListResponse(
                events=events,
                total=len(events)  # Usar la cantidad filtrada
            )
        else:
            print(f"❌ API_GATEWAY: Error del Events Service: {response.status_code} - {response.text}")
            raise HTTPException(
                status_code=response.status_code,
                detail=f"Error del Events Service: {response.text}"
            )

    except Exception as e:
        print(f"💥 API_GATEWAY: Error interno en get_events: {str(e)}")
        raise HTTPException(
            status_code=500,
            detail=f"Error interno: {str(e)}"
        )