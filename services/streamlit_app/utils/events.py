"""Utilidades para el manejo de eventos"""

import streamlit as st
from datetime import datetime
from typing import List, Dict
import time
from utils.api import make_api_request

def load_events():
    """Cargar eventos del usuario actual - SOLO del usuario logueado"""
    if not st.session_state.user_id:
        st.session_state.events = []
        print("🔧 DEBUG: No hay usuario logueado, lista de eventos vacía")
        return

    user_id_to_use = st.session_state.user_id
    response = make_api_request(f"/api/v1/events?user_id={user_id_to_use}")

    if response and response.status_code == 200:
        data = response.json()
        st.session_state.events = data.get('events', [])

        # ✅ DEBUG: Mostrar información de eventos cargados
        print(f"🔧 DEBUG: Se cargaron {len(st.session_state.events)} eventos para usuario {user_id_to_use}")
        for event in st.session_state.events:
            print(f"🔧 DEBUG: Evento - {event['title']} a las {event['start_time']}")

    else:
        st.session_state.events = []
        print(f"🔧 DEBUG: Error cargando eventos - Status: {response.status_code if response else 'No response'}")

def get_events_for_day(date: datetime) -> List[Dict]:
    """Obtener eventos para un día específico"""
    day_events = []
    for event in st.session_state.events:
        event_date = datetime.fromisoformat(event['start_time'].replace('Z', '+00:00'))
        if (event_date.year == date.year and
            event_date.month == date.month and
            event_date.day == date.day):
            day_events.append(event)
    return day_events

def create_event_with_conflict_check(event_data):
    """Crear evento con verificación de conflictos en tiempo real"""
    import time

    # ✅ DEBUG: Ver datos antes de enviar
    print(f"🔧 DEBUG: Enviando evento - Título: {event_data['title']}")
    print(f"🔧 DEBUG: Horas en event_data - Inicio: {event_data['start_time']}, Fin: {event_data['end_time']}")

    # Enviar evento
    response = make_api_request("/api/v1/events", "POST", event_data)

    if response and response.status_code in [202, 200]:
        response_data = response.json()
        event_id = response_data["event_id"]
        st.info("⏳ Procesando evento...")

        print(f"🔧 DEBUG: Evento enviado con ID: {event_id}")

        # Verificar estado periódicamente
        max_attempts = 10
        for attempt in range(max_attempts):
            time.sleep(1)  # Esperar 1 segundo entre verificaciones

            print(f"🔧 DEBUG: Verificando estado (intento {attempt + 1}/{max_attempts})...")

            status_response = make_api_request(f"/api/v1/events/{event_id}/status", "GET")

            if status_response:
                print(f"🔧 DEBUG: Status response - Code: {status_response.status_code}")
                if status_response.status_code in [202, 200]:
                    status_data = status_response.json()
                    print(f"🔧 DEBUG: Status data: {status_data}")

                    if status_data["status"] == "completed":
                        if status_data["success"]:
                            print("🔧 DEBUG: Evento creado exitosamente")
                            return True, "✅ Evento creado exitosamente!"
                        else:
                            # Detectar conflicto de horario específico
                            error_message = status_data.get("message", "").lower()
                            print(f"🔧 DEBUG: Error del evento: {error_message}")
                            if "conflicto" in error_message or "conflict" in error_message:
                                return False, "🚫 **Conflicto de horario**: Ya tienes un evento programado en este horario"
                            else:
                                return False, f"❌ Error: {status_data.get('message', 'Error desconocido')}"
                    elif status_data["status"] == "processing":
                        continue  # Seguir esperando
                else:
                    print(f"🔧 DEBUG: Error en status response: {status_response.text}")

            # Si no hay respuesta de estado, continuar esperando
            if attempt == max_attempts - 1:
                return False, "⏰ Tiempo de espera agotado - No se pudo verificar el estado del evento"

        return False, "❓ Estado del evento desconocido"
    else:
        error_detail = "Servicio no disponible"
        if response:
            try:
                error_data = response.json()
                error_detail = error_data.get("detail", "Error desconocido")
                print(f"🔧 DEBUG: Error response: {error_data}")
            except:
                error_detail = response.text
                print(f"🔧 DEBUG: Error text: {error_detail}")
        return False, f"❌ Error al enviar el evento: {error_detail}"

def delete_event(event_id):
    """Eliminar un evento específico - CORREGIDA"""
    if not st.session_state.user_id:
        st.error("❌ Debes estar logueado para eliminar eventos")
        return False

    user_id = st.session_state.user_id

    print(f"🔧 DEBUG: Eliminando evento {event_id} para usuario {user_id}")

    with st.spinner("🗑️ Eliminando evento..."):
        response = make_api_request(f"/api/v1/events/{event_id}?user_id={user_id}", "DELETE")

    if response is None:
        st.error("❌ No se pudo conectar al servidor para eliminar el evento")
        return False

    if response.status_code == 200:
        response_data = response.json()
        status = response_data.get("status")

        if status == "processing":
            # ✅ EN PUB/SUB: La eliminación es asíncrona
            st.success("✅ Solicitud de eliminación enviada correctamente")
            st.info("🔄 La eliminación se está procesando en segundo plano...")

            # Esperar un poco para que el Events Service procese
            time.sleep(2)
            load_events()  # Recargar eventos para reflejar los cambios
            return True

        elif status == "success":
            st.success("✅ Evento eliminado exitosamente")
            load_events()
            return True

        else:
            # ❌ Error específico del servidor
            error_msg = response_data.get('message', 'Error desconocido')
            st.error(f"❌ Error al eliminar evento: {error_msg}")
            return False

    else:
        # ❌ Error HTTP
        try:
            error_data = response.json()
            error_message = error_data.get("error", "Error desconocido")
            st.error(f"❌ Error al eliminar evento: {error_message}")
        except:
            st.error(f"❌ Error al eliminar evento: {response.text}")
        return False

def confirm_event_deletion(event_title):
    """Mostrar diálogo de confirmación para eliminar evento - CORREGIDA"""
    # Usar un contenedor en lugar de columns anidados
    container = st.container()

    with container:
        st.warning(f"⚠️ ¿Estás seguro de que quieres eliminar el evento **'{event_title}'**?")

        # Usar buttons sin columns anidados
        confirm_col1, confirm_col2 = st.columns(2)

        with confirm_col1:
            if st.button("✅ Sí, eliminar", key=f"confirm_yes_{event_title}", use_container_width=True, type="primary"):
                return True

        with confirm_col2:
            if st.button("❌ Cancelar", key=f"confirm_no_{event_title}", use_container_width=True):
                return False

    return False