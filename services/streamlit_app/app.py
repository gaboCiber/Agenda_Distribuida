import streamlit as st
import requests
from datetime import datetime, timedelta
import calendar
from typing import List, Dict
import time

# Configuración de la página
st.set_page_config(
    page_title="Agenda Distribuida",
    page_icon="📅",
    layout="wide",
    initial_sidebar_state="expanded")

# Configuración
API_GATEWAY_URL = "http://agenda-api-gateway:8000"

def init_session_state():
    # Inicializar el estado de la sesión - ACTUALIZADA
    if 'user_id' not in st.session_state:
        st.session_state.user_id = None
    if 'user_email' not in st.session_state:
        st.session_state.user_email = None
    if 'user_username' not in st.session_state:
        st.session_state.user_username = None
    if 'events' not in st.session_state:
        st.session_state.events = []
    if 'current_date' not in st.session_state:
        st.session_state.current_date = datetime.now()
    if 'selected_date' not in st.session_state:
        st.session_state.selected_date = None
    if 'form_submitted' not in st.session_state:
        st.session_state.form_submitted = False
    if 'last_submission_time' not in st.session_state:
        st.session_state.last_submission_time = 0
    if 'pending_events' not in st.session_state:
        st.session_state.pending_events = {}
    if 'deleting_event' not in st.session_state:
        st.session_state.deleting_event = None
    if 'groups' not in st.session_state:
        st.session_state.groups = []
    if 'selected_group' not in st.session_state:
        st.session_state.selected_group = None
    if 'group_invitations' not in st.session_state:
        st.session_state.group_invitations = []
    if 'show_group_management' not in st.session_state:
        st.session_state.show_group_management = False
    if 'show_create_group' not in st.session_state:
        st.session_state.show_create_group = False
        st.session_state.creating_group = False

def make_api_request(endpoint, method="GET", data=None):
    """Realizar peticiones al API Gateway con mejor manejo de errores - CORREGIDA"""
    url = f"{API_GATEWAY_URL}{endpoint}"
    headers = {
        "Content-Type": "application/json",
    }
    
    # Agregar el ID de usuario a los headers si está disponible
    if 'user_id' in st.session_state and st.session_state.user_id:
        headers["X-User-ID"] = st.session_state.user_id
    
    print(f"🔧 DEBUG: Haciendo {method} request a {url}")
    print(f"🔧 DEBUG: Headers: {headers}")
    if data and (method == "POST" or method == "PUT"):
        print(f"🔧 DEBUG: Datos enviados: {data}")
    
    try:
        if method == "GET":
            response = requests.get(url, headers=headers, timeout=10)
        elif method == "POST":
            response = requests.post(url, json=data, headers=headers, timeout=10)
        elif method == "PUT":
            response = requests.put(url, json=data, headers=headers, timeout=10)
        elif method == "DELETE":
            response = requests.delete(url, headers=headers, timeout=10)
        else:
            print(f"🔧 DEBUG: {method} {endpoint} - Response es None")
        
        return response
        
    except requests.exceptions.RequestException as e:
        print(f"🔧 DEBUG: Connection Error: {e}")
        st.error(f"Error de conexión: {e}")
        return None
    except Exception as e:
        print(f"🔧 DEBUG: Unexpected Error: {e}")
        st.error(f"Error inesperado: {e}")
        return None

def load_events():
    """Cargar eventos del usuario actual con información de debug"""
    if st.session_state.user_id or True:  # Temporal para pruebas
        user_id_to_use = st.session_state.user_id or "user_test"
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

def navigate_calendar():
    """Controles de navegación del calendario"""
    col1, col2, col3, col4, col5 = st.columns([1, 1, 2, 1, 1])
    
    with col1:
        if st.button("◀◀"):
            st.session_state.current_date = st.session_state.current_date.replace(
                year=st.session_state.current_date.year - 1
            )
            st.session_state.selected_date = None
    
    with col2:
        if st.button("◀"):
            if st.session_state.current_date.month == 1:
                st.session_state.current_date = st.session_state.current_date.replace(
                    year=st.session_state.current_date.year - 1, month=12
                )
            else:
                st.session_state.current_date = st.session_state.current_date.replace(
                    month=st.session_state.current_date.month - 1
                )
            st.session_state.selected_date = None
    
    with col3:
        # Selector de mes y año
        current_year = st.session_state.current_date.year
        current_month = st.session_state.current_date.month
        
        selected_year = st.selectbox(
            "Año",
            range(current_year - 10, current_year + 11),
            index=10,
            label_visibility="collapsed"
        )
        
        selected_month = st.selectbox(
            "Mes",
            list(range(1, 13)),
            format_func=lambda x: calendar.month_name[x],
            index=current_month - 1,
            label_visibility="collapsed"
        )
        
        if selected_year != current_year or selected_month != current_month:
            st.session_state.current_date = st.session_state.current_date.replace(
                year=selected_year, month=selected_month
            )
            st.session_state.selected_date = None
    
    with col4:
        if st.button("▶"):
            if st.session_state.current_date.month == 12:
                st.session_state.current_date = st.session_state.current_date.replace(
                    year=st.session_state.current_date.year + 1, month=1
                )
            else:
                st.session_state.current_date = st.session_state.current_date.replace(
                    month=st.session_state.current_date.month + 1
                )
            st.session_state.selected_date = None
    
    with col5:
        if st.button("▶▶"):
            st.session_state.current_date = st.session_state.current_date.replace(
                year=st.session_state.current_date.year + 1
            )
            st.session_state.selected_date = None

def render_calendar():
    """Renderizar calendario mensual navegable con eventos"""
    st.subheader(f"📅 {calendar.month_name[st.session_state.current_date.month]} {st.session_state.current_date.year}")
    
    # Controles de navegación
    navigate_calendar()
    
    # Obtener días del mes
    cal = calendar.monthcalendar(
        st.session_state.current_date.year, 
        st.session_state.current_date.month
    )
    
    # Nombres de los días
    days = ['Lun', 'Mar', 'Mié', 'Jue', 'Vie', 'Sáb', 'Dom']
    
    # Header de días
    cols = st.columns(7)
    for i, day in enumerate(days):
        with cols[i]:
            st.write(f"**{day}**")
    
    # Crear el calendario con claves únicas
    for week_num, week in enumerate(cal):
        cols = st.columns(7)
        for i, day in enumerate(week):
            with cols[i]:
                if day != 0:
                    current_day = datetime(
                        st.session_state.current_date.year,
                        st.session_state.current_date.month,
                        day
                    )
                    
                    # Verificar si hay eventos en este día
                    day_events = get_events_for_day(current_day)
                    
                    # Determinar si es hoy
                    is_today = current_day.date() == datetime.now().date()
                    
                    # Determinar si está seleccionado
                    is_selected = (
                        st.session_state.selected_date and 
                        st.session_state.selected_date.date() == current_day.date()
                    )
                    
                    # Texto del botón
                    button_text = f"{day}"
                    if day_events:
                        button_text += f"\n🔵 {len(day_events)}"
                    
                    # Color del botón
                    button_type = "primary" if is_selected else "secondary"
                    
                    # 🔑 CLAVE ÚNICA para cada botón
                    unique_key = f"day_{st.session_state.current_date.year}_{st.session_state.current_date.month}_{day}"
                    
                    # Botón clickeable para el día
                    if st.button(
                        button_text,
                        key=unique_key,  # Clave única basada en fecha
                        use_container_width=True,
                        type=button_type
                    ):
                        st.session_state.selected_date = current_day
                        st.session_state.form_submitted = False  # Reset form state
                        st.rerun()
                        
                else:
                    # 🔑 Días vacíos también necesitan claves únicas
                    empty_key = f"empty_{week_num}_{i}"
                    st.button("", key=empty_key, disabled=True, use_container_width=True)

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
        render_add_to_group_form(st.session_state.adding_to_group)

    # Crear evento de grupo si está seleccionado
    if st.session_state.get('creating_group_event'):
        render_create_group_event_form()

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

def load_groups():
    """Cargar grupos del usuario actual"""
    if not st.session_state.user_id:
        st.warning("🔒 Debes iniciar sesión para ver tus grupos")
        st.session_state.groups = []
        return

    try:
        # Usar el endpoint correcto para obtener los grupos del usuario
        response = make_api_request(f"/api/v1/groups/users/{st.session_state.user_id}/groups", "GET")
        
        if response is None:
            st.error("❌ No se pudo conectar con el servidor")
            st.session_state.groups = []
            return
            
        if response.status_code == 200:
            data = response.json()
            # Asegurarse de que siempre sea una lista
            st.session_state.groups = data.get('groups', []) or []
            print(f"🔧 DEBUG: Se cargaron {len(st.session_state.groups)} grupos para usuario {st.session_state.user_id}")
            return
            
        # Si el endpoint específico falla, intentar el endpoint genérico
        print(f"🔧 DEBUG: Endpoint específico falló ({response.status_code}), intentando con endpoint genérico")
        response = make_api_request("/api/v1/groups", "GET")
        
        if response and response.status_code == 200:
            data = response.json()
            # Filtrar manualmente los grupos donde el usuario es miembro
            all_groups = data.get('groups', []) or []
            st.session_state.groups = [
                group for group in all_groups 
                if any(member.get('user_id') == st.session_state.user_id 
                      for member in group.get('members', []))
            ]
            print(f"🔧 DEBUG: Se filtraron {len(st.session_state.groups)} grupos para usuario {st.session_state.user_id}")
        else:
            error_msg = response.text if response else "Sin respuesta del servidor"
            print(f"🔧 DEBUG: Error al cargar grupos: {error_msg}")
            st.session_state.groups = []
            
    except Exception as e:
        print(f"🔧 DEBUG: Excepción en load_groups: {str(e)}")
        st.session_state.groups = []
        
    # No forzar actualización automática para evitar bucles
    # La interfaz se actualizará con el siguiente renderizado

def load_group_invitations():
    """Cargar invitaciones de grupo del usuario actual"""
    if not st.session_state.user_id:
        st.session_state.group_invitations = []
        return

    try:
        # Usar el endpoint específico del usuario si está disponible
        response = make_api_request(f"/api/v1/users/{st.session_state.user_id}/group-invitations", "GET")
        
        if response is None:
            st.error("❌ No se pudo conectar con el servidor")
            st.session_state.group_invitations = []
            return
            
        if response.status_code == 200:
            st.session_state.group_invitations = response.json().get('invitations', [])
            print(f"🔧 DEBUG: Se cargaron {len(st.session_state.group_invitations)} invitaciones para el usuario {st.session_state.user_id}")
        elif response.status_code == 404:
            # Si el endpoint específico no existe, intentar con el endpoint genérico
            response = make_api_request("/api/v1/groups/invitations", "GET")
            if response and response.status_code == 200:
                all_invitations = response.json().get('invitations', [])
                # Filtrar manualmente las invitaciones del usuario actual
                st.session_state.group_invitations = [
                    inv for inv in all_invitations 
                    if inv.get('user_id') == st.session_state.user_id
                ]
                print(f"🔧 DEBUG: Se filtraron {len(st.session_state.group_invitations)} invitaciones para el usuario {st.session_state.user_id}")
            else:
                st.session_state.group_invitations = []
        else:
            st.error(f"❌ Error al cargar invitaciones: {response.text}")
            st.session_state.group_invitations = []
            
    except Exception as e:
        st.error(f"❌ Error al cargar invitaciones: {str(e)}")
        st.session_state.group_invitations = []
        print(f"🔧 DEBUG: Excepción en load_group_invitations: {str(e)}")
        
    # No forzar actualización automática para evitar bucles
    # La interfaz se actualizará con el siguiente renderizado

def render_sidebar():
    """Renderizar sidebar con gestión de grupos - ACTUALIZADA"""
    st.sidebar.markdown("---")

    # Estado de autenticación
    if st.session_state.user_id:
        st.sidebar.write(f"**Usuario:** {st.session_state.user_username or st.session_state.user_email}")

        # Toggle para mostrar/ocultar gestión de grupos
        if st.sidebar.button(
            "### 👥 Gestión de Grupos" if not st.session_state.show_group_management else "📅 Ver Agenda",
            use_container_width=True
        ):
            if st.session_state.show_group_management:
                st.session_state.show_group_management = False
                load_events()
                st.sidebar.success("Eventos actualizados")
            else:
                st.session_state.show_group_management = True
                load_groups()
                load_group_invitations()
                st.sidebar.success("Grupos cargados")

        # Mostrar gestión de grupos si está activada
        if st.session_state.show_group_management:
            render_group_management_sidebar()

    else:
        st.sidebar.info("🔐 No has iniciado sesión")

def render_group_management_sidebar():
    """Renderizar sección de gestión de grupos en el sidebar"""
    # Mis Grupos
    if st.session_state.groups:
        st.sidebar.markdown("#### Mis Grupos")
        for group in st.session_state.groups:
            group_name = group.get('name', 'Sin nombre')
            member_count = group.get('member_count', 0)

            if st.sidebar.button(
                f"📋 {group_name} ({member_count} miembros)",
                key=f"group_{group['id']}",
                use_container_width=True
            ):
                st.session_state.selected_group = group
                st.session_state.show_group_management = False
                st.rerun()

    # Invitaciones pendientes
    if st.session_state.group_invitations:
        st.sidebar.markdown("#### 📨 Invitaciones")
        for invitation in st.session_state.group_invitations:
            if invitation.get('status') == 'pending':
                group_name = invitation.get('group_name', 'Grupo desconocido')
                invited_by = invitation.get('invited_by', 'Desconocido')

                col1, col2 = st.sidebar.columns([3, 1])
                with col1:
                    st.sidebar.write(f"📋 {group_name}")
                    st.sidebar.caption(f"Invitado por: {invited_by}")
                with col2:
                    if st.sidebar.button("✅", key=f"accept_{invitation['id']}", help="Aceptar"):
                        respond_to_invitation(invitation['id'], 'accept')
                    if st.sidebar.button("❌", key=f"reject_{invitation['id']}", help="Rechazar"):
                        respond_to_invitation(invitation['id'], 'reject')

    # Crear nuevo grupo
    st.sidebar.markdown("#### ➕ Nuevo Grupo")
    if st.sidebar.button("Crear Grupo", use_container_width=True):
        st.session_state.show_create_group = True

def respond_to_invitation(invitation_id: str, action: str):
    """Responder a una invitación de grupo"""
    data = {"action": action}
    response = make_api_request(f"/api/v1/groups/invitations/{invitation_id}/respond", "POST", data)

    if response and response.status_code == 200:
        st.sidebar.success(f"Invitación {action}ada correctamente")
        load_group_invitations()  # Recargar invitaciones
    else:
        st.sidebar.error("Error al procesar la invitación")
   
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
    user_id = st.session_state.user_id or "user_test"
    
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

def main():
    """Función principal"""
    init_session_state()
    render_sidebar()
    
    # Título principal
    st.title("📅 Mi Agenda")
    
    # Si el usuario está autenticado, mostrar calendario o gestión de grupos
    if st.session_state.user_id:  # Temporal: siempre mostrar para pruebas
        # Mostrar gestión de grupos si está activada
        if st.session_state.show_group_management:
            render_group_management()
        elif st.session_state.selected_group:
            render_group_detail()
        else:
            # Recargar eventos si es necesario
            if not st.session_state.events:
                load_events()

            # Layout principal
            col1, col2 = st.columns([2, 1])

            with col1:
                # Calendario principal
                render_calendar()

            with col2:
                # Eventos del día seleccionado y formulario de creación
                render_selected_day_events()

    else:
        # Pantalla de bienvenida para usuarios no autenticados
        st.markdown("""
        ## Bienvenido a tu Agenda

        **Organiza tus eventos y mantente sincronizado**

        👈 **Usa el sidebar para ir a la página de autenticación**
        """)

def render_group_management():
    """Renderizar interfaz de gestión de grupos"""
    st.title("👥 Gestión de Grupos")

    # Botón para volver a la agenda
    if st.button("⬅️ Volver a Agenda", key="back_to_calendar_btn", use_container_width=True):
        st.session_state.show_group_management = False
        st.session_state.selected_group = None
        st.rerun()

    # Pestañas para organización
    tabs = ["📋 Mis Grupos", "➕ Crear Grupo", "📨 Invitaciones"]
    if 'active_tab' not in st.session_state:
        st.session_state.active_tab = tabs[0]
        
    # Crear pestañas
    tab1, tab2, tab3 = st.tabs(tabs)
    
    # Actualizar pestaña activa
    for i, tab in enumerate(tabs):
        if st.session_state.active_tab == tab:
            st.session_state.active_tab_index = i
            break
    
    with tab1:
        st.subheader("Mis Grupos")
        
        # Cargar grupos del usuario
        load_groups()
        
        # Mostrar mensaje si no hay grupos
        if not st.session_state.groups:
            st.info("No perteneces a ningún grupo. Crea uno nuevo o acepta una invitación.")
        else:
            # Mostrar tarjetas de grupos
            for group in st.session_state.groups:
                with st.container():
                    st.markdown("---")
                    col1, col2 = st.columns([4, 1])
                    
                    with col1:
                        st.markdown(f"### {group.get('name', 'Sin nombre')}")
                        st.caption(f"ID: {group.get('id')}")
                        
                        if group.get('description'):
                            st.markdown(f"*{group.get('description')}*")
                        
                        # Mostrar información del grupo
                        members = group.get('members', [])
                        st.markdown(f"**Miembros:** {len(members)}")
                        
                        # Mostrar eventos del grupo si los hay
                        events = group.get('events', [])
                        if events:
                            st.markdown(f"**Eventos programados:** {len(events)}")
                    
                    with col2:
                        # Botón para ver detalles del grupo
                        if st.button("Ver detalles", key=f"view_{group['id']}", use_container_width=True):
                            st.session_state.selected_group = group
                            st.session_state.show_group_management = False
                            st.rerun()
                        
                        # Botón para invitar miembros
                        if st.button("Invitar", key=f"invite_{group['id']}", use_container_width=True):
                            st.session_state.show_invite_form = group['id']
                            st.rerun()
            
            # Botón para actualizar la lista de grupos
            if st.button("🔄 Actualizar lista de grupos"):
                load_groups()
                st.rerun()
    
    with tab2:
        st.subheader("Crear Nuevo Grupo")
        render_create_group_form()
    
    with tab3:
        st.subheader("Invitaciones Pendientes")
        load_group_invitations()
        
        if not st.session_state.group_invitations:
            st.info("No tienes invitaciones pendientes.")
        else:
            for invite in st.session_state.group_invitations:
                with st.container():
                    st.markdown("---")
                    col1, col2 = st.columns([3, 1])
                    
                    with col1:
                        st.markdown(f"### {invite.get('group_name', 'Grupo Desconocido')}")
                        st.markdown(f"*Invitado por: {invite.get('invited_by', 'Usuario desconocido')}*")
                        
                        if invite.get('message'):
                            st.markdown(f"*Mensaje: {invite.get('message')}*")
                    
                    with col2:
                        # Botones de acción
                        if st.button("✅ Aceptar", 
                                   key=f"accept_{invite['id']}",
                                   use_container_width=True):
                            respond_to_invitation(invite['id'], "accept")
                            st.rerun()
                            
                        if st.button("❌ Rechazar", 
                                   key=f"reject_{invite['id']}",
                                   use_container_width=True):
                            respond_to_invitation(invite['id'], "reject")
                            st.rerun()
            
            # Botón para actualizar invitaciones
            if st.button("🔄 Actualizar invitaciones"):
                load_group_invitations()
                st.rerun()

def render_create_group_form():
    """Renderizar formulario para crear grupo"""
    st.markdown("### ➕ Crear Nuevo Grupo")

    # Usar una clave única para el formulario
    if 'form_key' not in st.session_state:
        st.session_state.form_key = f"create_group_form_{time.time()}"
    
    # Inicializar el estado del formulario
    if 'form_submitted' not in st.session_state:
        st.session_state.form_submitted = False

    with st.form(key=st.session_state.form_key):
        group_name = st.text_input(
            "Nombre del Grupo*", 
            placeholder="Ej: Equipo de Desarrollo", 
            key=f"group_name_{st.session_state.form_key}"
        )
        group_description = st.text_area(
            "Descripción", 
            placeholder="Describe el propósito del grupo...", 
            key=f"group_desc_{st.session_state.form_key}"
        )

        col1, col2 = st.columns(2)
        with col1:
            create_btn = st.form_submit_button("🎯 Crear Grupo", use_container_width=True, type="primary")
        with col2:
            cancel_btn = st.form_submit_button("❌ Cancelar", use_container_width=True, type="secondary")

        # Si el formulario ya fue enviado, no hacer nada
        if st.session_state.form_submitted:
            st.session_state.form_submitted = False
            st.rerun()
            return False

        if create_btn and group_name:
            try:
                # Marcar el formulario como enviado
                st.session_state.form_submitted = True
                
                # Crear grupo
                group_data = {
                    "name": group_name,
                    "description": group_description
                }

                print(f"🔧 DEBUG: Enviando solicitud para crear grupo: {group_data}")
                response = make_api_request("/api/v1/groups", "POST", group_data)
                print(f"🔧 DEBUG: Respuesta del servidor: {response.status_code if response else 'No response'}")
                
                if response and response.status_code == 201:
                    st.success("✅ Grupo creado exitosamente!")
                    # Limpiar el estado del formulario
                    st.session_state.show_create_group = False
                    # Recargar la lista de grupos
                    load_groups()
                    # Generar una nueva clave para el formulario
                    st.session_state.form_key = f"create_group_form_{time.time()}"
                    # Forzar recarga
                    st.rerun()
                else:
                    error_msg = response.text if response else "No se pudo conectar al servidor"
                    st.error(f"❌ Error al crear el grupo: {error_msg}")
                    
            except Exception as e:
                st.error(f"❌ Error inesperado: {str(e)}")
                print(f"🔧 DEBUG: Error al crear grupo: {str(e)}")
            
            return False

        if cancel_btn:
            st.session_state.show_create_group = False
            st.rerun()
            
    return False

def render_group_detail():
    """Renderizar detalles de un grupo específico"""
    if not st.session_state.selected_group:
        return

    group = st.session_state.selected_group
    st.title(f"📋 {group['name']}")

    # Botón para volver
    if st.button("⬅️ Volver a Grupos", use_container_width=True):
        st.session_state.selected_group = None
        st.session_state.show_group_management = True
        load_groups()  # Recargar grupos para asegurar que tenemos la información más reciente
        st.rerun()

    # Información del grupo
    col1, col2 = st.columns([2, 1])

    with col1:
        st.markdown(f"**Descripción:** {group.get('description', 'Sin descripción')}")
        st.markdown(f"**Creado por:** {group.get('created_by', 'Desconocido')}")
        st.markdown(f"**Fecha de creación:** {group['created_at'][:10]}")

    with col2:
        st.markdown(f"**Miembros:** {group.get('member_count', 0)}")

        # Invitar miembros
        if st.button("➕ Invitar Miembro", use_container_width=True):
            st.session_state.inviting_to_group = group['id']
            st.rerun()

    # Gestión de invitaciones
    if st.session_state.get('inviting_to_group') == group['id']:
        render_invite_member_form(group)

    # Lista de miembros
    st.markdown("### 👥 Miembros del Grupo")
    render_group_members(group['id'])

    # Eventos del grupo
    st.markdown("### 📅 Eventos del Grupo")
    render_group_events(group['id'])

def render_invite_member_form(group):
    """Renderizar formulario para invitar miembros"""
    st.markdown("### 📨 Invitar Nuevo Miembro")

    with st.form(f"invite_member_form_{group['id']}"):
        user_email = st.text_input("Email del usuario a invitar*", placeholder="usuario@email.com")

        col1, col2 = st.columns(2)
        with col1:
            invite_btn = st.form_submit_button("📨 Enviar Invitación", use_container_width=True)
        with col2:
            cancel_btn = st.form_submit_button("❌ Cancelar", use_container_width=True)

        if invite_btn and user_email:
            # Crear invitación
            invite_data = {
                "group_id": group['id'],
                "user_id": user_email  # Asumiendo que usamos email como user_id por ahora
            }

            response = make_api_request("/api/v1/groups/invitations", "POST", invite_data)

            if response and response.status_code == 201:
                st.success("✅ Invitación enviada exitosamente!")
                st.session_state.inviting_to_group = None
                st.rerun()
            else:
                st.error("❌ Error al enviar la invitación")

        if cancel_btn:
            st.session_state.inviting_to_group = None
            st.rerun()

def render_group_members(group_id):
    """Renderizar lista de miembros del grupo"""
    response = make_api_request(f"/api/v1/groups/{group_id}/members", "GET")

    if response and response.status_code == 200:
        members_data = response.json()
        members = members_data.get('members', [])

        if not members:
            st.info("Este grupo no tiene miembros aún")
        else:
            for member in members:
                col1, col2, col3 = st.columns([2, 1, 1])

                with col1:
                    st.write(f"**{member['user_id']}**")
                    st.caption(f"Rol: {member['role']}")

                with col2:
                    st.caption(f"Unido: {member['joined_at'][:10]}")

                with col3:
                    if member['role'] != 'admin':  # No permitir remover admins
                        if st.button("❌", key=f"remove_{member['id']}", help="Remover miembro"):
                            remove_member(group_id, member['user_id'])
    else:
        st.error("Error al cargar miembros del grupo")

def render_group_events(group_id):
    """Renderizar eventos del grupo"""
    response = make_api_request(f"/api/v1/groups/{group_id}/events", "GET")

    if response and response.status_code == 200:
        events = response.json()

        if not events:
            st.info("Este grupo no tiene eventos programados")
        else:
            for event in events:
                with st.expander(f"📅 Evento: {event.get('event_id', 'Desconocido')}", expanded=False):
                    st.write(f"**Agregado por:** {event.get('added_by', 'Desconocido')}")
                    st.write(f"**Fecha de agregado:** {event.get('added_at', 'Desconocida')[:10]}")

                    if st.button("🗑️ Remover del Grupo", key=f"remove_event_{event['event_id']}", use_container_width=True):
                        remove_event_from_group(group_id, event['event_id'])
    else:
        st.error("Error al cargar eventos del grupo")

def remove_member(group_id: str, member_id: str):
    """Remover un miembro del grupo"""
    response = make_api_request(f"/api/v1/groups/{group_id}/members/{member_id}", "DELETE")

    if response and response.status_code == 204:
        st.success("✅ Miembro removido exitosamente")
        # Recargar detalles del grupo
        load_groups()
    else:
        st.error("❌ Error al remover miembro")

def remove_event_from_group(group_id: str, event_id: str):
    """Remover un evento del grupo"""
    response = make_api_request(f"/api/v1/groups/{group_id}/events/{event_id}", "DELETE")

    if response and response.status_code == 204:
        st.success("✅ Evento removido del grupo exitosamente")
        st.rerun()
    else:
        st.error("❌ Error al remover evento del grupo")

def render_add_to_group_form(event):
    """Renderizar formulario para agregar evento a grupo"""
    st.markdown("---")
    st.subheader(f"👥 Agregar Evento '{event['title']}' a Grupo")

    # Obtener grupos del usuario
    if not st.session_state.groups:
        load_groups()

    if not st.session_state.groups:
        st.warning("No tienes grupos disponibles. Crea un grupo primero.")
        if st.button("Crear Grupo", use_container_width=True):
            st.session_state.show_create_group = True
            st.session_state.adding_to_group = None
            st.rerun()
        return

    with st.form(f"add_to_group_form_{event['id']}"):
        # Selector de grupo
        group_options = {group['id']: group['name'] for group in st.session_state.groups}
        selected_group_id = st.selectbox(
            "Seleccionar Grupo",
            options=list(group_options.keys()),
            format_func=lambda x: group_options[x],
            help="Elige el grupo al que quieres agregar este evento"
        )

        col1, col2 = st.columns(2)
        with col1:
            add_btn = st.form_submit_button("➕ Agregar a Grupo", use_container_width=True)
        with col2:
            cancel_btn = st.form_submit_button("❌ Cancelar", use_container_width=True)

        if add_btn and selected_group_id:
            # Agregar evento al grupo
            group_event_data = {"event_id": event['id']}

            response = make_api_request(f"/api/v1/groups/{selected_group_id}/events", "POST", group_event_data)

            if response and response.status_code == 201:
                st.success(f"✅ Evento agregado al grupo '{group_options[selected_group_id]}' exitosamente!")
                st.session_state.adding_to_group = None
                st.rerun()
            else:
                st.error("❌ Error al agregar evento al grupo")

        if cancel_btn:
            st.session_state.adding_to_group = None
            st.rerun()

    # Opción para crear evento de grupo
    st.markdown("---")
    st.markdown("### 🎯 O Crear Evento de Grupo")
    st.info("Los eventos de grupo se crean automáticamente para todos los miembros del grupo, respetando sus horarios individuales.")

    if st.button("Crear Evento de Grupo", use_container_width=True, type="primary"):
        st.session_state.creating_group_event = True
        st.rerun()

def render_create_group_event_form():
    """Renderizar formulario para crear evento de grupo"""
    st.markdown("---")
    st.subheader("🎯 Crear Evento de Grupo")

    # Obtener grupos del usuario
    if not st.session_state.groups:
        load_groups()

    if not st.session_state.groups:
        st.warning("No tienes grupos disponibles. Crea un grupo primero.")
        return

    with st.form("create_group_event_form"):
        # Información básica del evento
        event_title = st.text_input("Título del Evento*", placeholder="Reunión de equipo, Evento grupal...")
        event_description = st.text_area("Descripción", placeholder="Detalles del evento grupal...")

        # Selector de grupo
        group_options = {group['id']: f"{group['name']} ({group.get('member_count', 0)} miembros)"
                        for group in st.session_state.groups}
        selected_group_id = st.selectbox(
            "Grupo*",
            options=list(group_options.keys()),
            format_func=lambda x: group_options[x],
            help="El evento se creará para todos los miembros de este grupo"
        )

        # Horarios
        col1, col2 = st.columns(2)
        with col1:
            event_start_time = st.time_input(
                "Hora de inicio*",
                value=datetime.strptime("09:00", "%H:%M").time(),
                step=60,
                help="Hora de inicio del evento"
            )
        with col2:
            event_end_time = st.time_input(
                "Hora de fin*",
                value=datetime.strptime("10:00", "%H:%M").time(),
                step=60,
                help="Hora de fin del evento"
            )

        # Selector de fecha
        event_date = st.date_input(
            "Fecha del Evento*",
            value=datetime.now().date(),
            help="Fecha en que ocurrirá el evento"
        )

        # Botones
        col1, col2 = st.columns(2)
        with col1:
            create_btn = st.form_submit_button("🎯 Crear Evento de Grupo", use_container_width=True, type="primary")
        with col2:
            cancel_btn = st.form_submit_button("❌ Cancelar", use_container_width=True)

        if create_btn:
            if not event_title or not selected_group_id:
                st.error("❌ Título del evento y grupo son obligatorios")
            elif event_end_time <= event_start_time:
                st.error("❌ La hora de fin debe ser después de la hora de inicio")
            else:
                # Crear evento de grupo
                event_datetime = datetime.combine(event_date, event_start_time)
                end_datetime = datetime.combine(event_date, event_end_time)

                group_event_data = {
                    "group_id": selected_group_id,
                    "title": event_title,
                    "description": event_description,
                    "start_time": event_datetime.isoformat(),
                    "end_time": end_datetime.isoformat(),
                    "user_id": st.session_state.user_id or "user_test"
                }

                with st.spinner("🔄 Creando evento de grupo..."):
                    response = make_api_request("/api/v1/group-events", "POST", group_event_data)

                if response and response.status_code == 200:
                    response_data = response.json()
                    status = response_data.get("status")

                    if status == "success":
                        st.success("✅ Evento de grupo creado exitosamente!")
                        st.info(f"📊 Evento creado para {len(response_data.get('created_events', []))} miembros del grupo")
                        load_events()  # Recargar eventos para mostrar los nuevos
                        st.session_state.creating_group_event = False
                        st.rerun()

                    elif status == "partial_success":
                        created_count = len(response_data.get('created_events', []))
                        failed_count = len(response_data.get('failed_members', []))
                        st.warning(f"⚠️ Evento creado parcialmente: {created_count} exitosos, {failed_count} fallidos")

                        if response_data.get('failed_members'):
                            st.error("Miembros con conflictos:")
                            for failed in response_data['failed_members']:
                                st.write(f"• {failed['member_id']}: {failed['error']}")

                        load_events()
                        st.session_state.creating_group_event = False

                    else:
                        error_msg = response_data.get("message", "Error desconocido")
                        st.error(f"❌ Error al crear evento de grupo: {error_msg}")

                else:
                    st.error("❌ Error al crear evento de grupo")

        if cancel_btn:
            st.session_state.creating_group_event = False
            st.rerun()

if __name__ == "__main__":
    main()

