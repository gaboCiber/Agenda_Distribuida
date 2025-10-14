# Router para endpoints de eventos de grupo
from fastapi import APIRouter, HTTPException
from datetime import datetime, timezone
from typing import List

from models import GroupEventCreateRequest, GroupEventResponse
from services.http_client import make_api_request, make_groups_service_request

router = APIRouter(prefix="/api/v1/group-events", tags=["group-events"])

@router.post("", response_model=dict)
async def create_group_event(group_event_data: GroupEventCreateRequest):
    """Crear evento de grupo - DIFERENTE a eventos individuales"""
    print(f"🔍 Creando evento de grupo: {group_event_data.title} en grupo {group_event_data.group_id}")

    # 1. Verificar que el usuario es miembro del grupo
    try:
        group_response = await make_groups_service_request(
            f"/groups/{group_event_data.group_id}",
            "GET",
            headers={"X-User-ID": group_event_data.user_id}
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
        members_response = await make_groups_service_request(
            f"/groups/{group_event_data.group_id}/members",
            "GET",
            headers={"X-User-ID": group_event_data.user_id}
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