"""Componentes para formularios de eventos"""

import streamlit as st
from datetime import datetime
import time
from utils.api import make_api_request
from utils.events import load_events

def render_selected_day_events():
    """Mostrar eventos del día seleccionado con opción para eliminar - CORREGIDA"""
    import time

    if not st.session_state.selected_date:
        return

    st.markdown("---")
    st.subheader(f"📋 Eventos para el {st.session_state.selected_date.strftime('%d/%m/%Y')}")

    day_events = get_events_for_day(st.session_state.selected_date)

    if st.session_state.get('deleting_event'):
        event_to_delete = st.session_state.deleting_event
        st.markdown("---")
        st.warning(f"⚠️ ¿Estás seguro de que quieres eliminar el evento **'{event_to_delete['title']}'**?")

        confirm_col1, confirm_col2 = st.columns(2)

        with confirm_col1:
            if st.button("✅ Sí, eliminar", key="final_confirm_yes", use_container_width=True, type="primary"):
                success = delete_event(event_to_delete['id'])
                st.session_state.deleting_event = None
                if success:
                    st.success("✅ Eliminación completada")
                    load_events()

        with confirm_col2:
            if st.button("❌ Cancelar", key="final_confirm_no", use_container_width=True):
                st.session_state.deleting_event = None


        if not day_events:
            st.info("📭 No hay eventos programados para este día")
    else:
        for i, event in enumerate(day_events):
            # Usar un expander para cada evento
            with st.expander(f"📅 {event['title']}", expanded=False):
                col1, col2, col3 = st.columns([3, 1, 1])

                with col1:
                    st.write(f"**Descripción:** {event['description'] or 'Sin descripción'}")

                    # Mostrar horas
                    start_time = datetime.fromisoformat(event['start_time'].replace('Z', '+00:00'))
                    end_time = datetime.fromisoformat(event['end_time'].replace('Z', '+00:00'))
                    st.write(f"**Horario:** {start_time.strftime('%H:%M')} - {end_time.strftime('%H:%M')}")

                with col2:
                    duration = (end_time - start_time).total_seconds() / 3600
                    st.write(f"**Duración:** {duration:.1f} h")

                with col3:
                    # 🔥 CORREGIDO: Botón que actualiza estado y hace rerun inmediato
                    delete_key = f"delete_{event['id']}_{i}"
                    if st.button("🗑️ Eliminar", key=delete_key, use_container_width=True):
                        # Guardar evento a eliminar y hacer rerun inmediato
                        st.session_state.deleting_event = event
                        st.rerun()

                    # Nuevo botón para agregar evento a grupo
                    if st.button("👥 Grupo", key=f"group_{event['id']}_{i}", use_container_width=True, help="Agregar a grupo"):
                        st.session_state.adding_to_group = event
                        st.rerun()

    # Agregar evento a grupo si está seleccionado
    if st.session_state.get('adding_to_group'):
        print("")
        #render_add_to_group_form(st.session_state.adding_to_group)

    # Crear evento de grupo si está seleccionado
    if st.session_state.get('creating_group_event'):
        print("")
        #render_create_group_event_form()

    # Sección para crear nuevo evento (mantener existente)
    st.markdown("---")
    st.subheader("➕ Crear Nuevo Evento")

    with st.form(f"create_event_{st.session_state.selected_date.strftime('%Y%m%d')}", clear_on_submit=True):
        event_title = st.text_input("Título del evento*", placeholder="Reunión, Cita, Recordatorio...")
        event_description = st.text_area("Descripción", placeholder="Detalles del evento...")


        col1, col2 = st.columns(2)
        with col1:
            event_start_time = st.time_input(
                "Hora de inicio*",
                value=datetime.strptime("00:00", "%H:%M").time(),
                step=60,
                help="Puede escribir cualquier hora o usar los controles"
            )
        with col2:
            event_end_time = st.time_input(
                "Hora de fin*",
                value=datetime.strptime("00:15", "%H:%M").time(),
                step=60,  # 🔥 Paso de 1 minuto (60 segundos)
                help="Puede escribir cualquier hora o usar los controles"
            )

        event_start_datetime = datetime.combine(st.session_state.selected_date.date(), event_start_time)
        event_end_datetime = datetime.combine(st.session_state.selected_date.date(), event_end_time)

        create_button = st.form_submit_button("Crear Evento", use_container_width=True)

        if create_button:
            current_time = time.time()
            if (st.session_state.form_submitted or
                current_time - st.session_state.last_submission_time < 3):
                return

            st.session_state.form_submitted = True
            st.session_state.last_submission_time = current_time

            if not event_title:
                st.error("❌ El título del evento es obligatorio")
                st.session_state.form_submitted = False
            elif event_end_datetime <= event_start_datetime:
                st.error("❌ La hora de fin debe ser después de la hora de inicio")
                st.session_state.form_submitted = False
            else:
                event_data = {
                    "title": event_title,
                    "description": event_description,
                    "start_time": event_start_datetime.isoformat(),
                    "end_time": event_end_datetime.isoformat(),
                    "user_id": st.session_state.user_id or "user_test"
                }

                print(f"🔧 DEBUG: Enviando evento: {event_data}")

                with st.spinner("🔄 Verificando disponibilidad..."):
                    response = make_api_request("/api/v1/events", "POST", event_data)

                if response and response.status_code == 200:
                    response_data = response.json()
                    print(f"🔧 DEBUG: Respuesta del API: {response_data}")

                    status = response_data.get("status")

                    if status == "success":
                        st.success("✅ " + response_data.get("message", "Evento creado exitosamente!"))
                        time.sleep(1)
                        load_events()
                        st.session_state.form_submitted = False
                        st.rerun()

                    elif status == "error":
                        error_message = response_data.get("message", "Conflicto de horario detectado")
                        conflicting_events = response_data.get("conflicting_events", [])

                        st.error(f"🚫 **{error_message}**")

                        if conflicting_events:
                            st.warning("📅 **Eventos que entran en conflicto:**")
                            for conflict in conflicting_events:
                                conflict_start = datetime.fromisoformat(conflict['start_time'].replace('Z', '+00:00'))
                                conflict_end = datetime.fromisoformat(conflict['end_time'].replace('Z', '+00:00'))

                                st.write(f"• **{conflict['title']}**: {conflict_start.strftime('%H:%M')} - {conflict_end.strftime('%H:%M')}")
                        else:
                            st.info("ℹ️ No se pudieron obtener detalles de los eventos conflictivos")

                        st.session_state.form_submitted = False

                    else:
                        #st.info("⏳ " + response_data.get("message", "Evento en proceso..."))
                        time.sleep(2)
                        load_events()
                        st.session_state.form_submitted = False
                        st.rerun()

                else:
                    error_detail = "Error desconocido"
                    if response:
                        try:
                            error_data = response.json()
                            error_detail = error_data.get("detail", "Error desconocido")
                        except:
                            error_detail = response.text
                    st.error(f"❌ Error al crear el evento: {error_detail}")
                    st.session_state.form_submitted = False

def get_events_for_day(date: datetime):
    """Obtener eventos para un día específico"""
    from utils.events import get_events_for_day
    return get_events_for_day(date)

def delete_event(event_id):
    """Eliminar un evento específico"""
    from utils.events import delete_event
    return delete_event(event_id)