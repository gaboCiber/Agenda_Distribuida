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
    initial_sidebar_state="expanded"
)

# Configuración
API_GATEWAY_URL = "http://agenda-api-gateway:8000"

def init_session_state():
    """Inicializar el estado de la sesión"""
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

def make_api_request(endpoint, method="GET", data=None):
    """Realizar peticiones al API Gateway con mejor manejo de errores"""
    url = f"{API_GATEWAY_URL}{endpoint}"
    headers = {"Content-Type": "application/json"}
    
    print(f"🔧 DEBUG: Haciendo {method} request a {url}")
    if data and method == "POST":
        print(f"🔧 DEBUG: Datos COMPLETOS enviados: {data}")
    
    try:
        if method == "GET":
            response = requests.get(url, headers=headers, timeout=10)
        elif method == "POST":
            response = requests.post(url, json=data, headers=headers, timeout=10)
        
        # ✅ DEBUG: Ver respuestas del API
        print(f"🔧 DEBUG: {method} {endpoint} - Status: {response.status_code}")
        if response.status_code != 200 and response.status_code != 202:
            print(f"🔧 DEBUG: Response Text: {response.text}")
        else:
            try:
                response_data = response.json()
                print(f"🔧 DEBUG: Response Data: {response_data}")
            except:
                print(f"🔧 DEBUG: Response Text: {response.text}")
        
        return response
    except requests.exceptions.RequestException as e:
        print(f"🔧 DEBUG: Connection Error: {e}")
        st.error(f"Error de conexión: {e}")
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
    """Mostrar eventos del día seleccionado y opción para crear evento"""
    import time
    
    if not st.session_state.selected_date:
        return
    
    st.markdown("---")
    st.subheader(f"📋 Eventos para el {st.session_state.selected_date.strftime('%d/%m/%Y')}")
    
    day_events = get_events_for_day(st.session_state.selected_date)
    
    if not day_events:
        st.info("📭 No hay eventos programados para este día")
    else:
        for i, event in enumerate(day_events):
            with st.expander(f"📅 {event['title']}", expanded=True):
                start_time = datetime.fromisoformat(event['start_time'].replace('Z', '+00:00'))
                end_time = datetime.fromisoformat(event['end_time'].replace('Z', '+00:00'))
                
                col1, col2 = st.columns([3, 1])
                with col1:
                    st.write(f"**Descripción:** {event['description'] or 'Sin descripción'}")
                    st.write(f"**Horario:** {start_time.strftime('%H:%M')} - {end_time.strftime('%H:%M')}")
                with col2:
                    st.write(f"**Duración:** {(end_time - start_time).total_seconds() / 3600:.1f} horas")
    
    # Opción para crear nuevo evento en este día
    st.markdown("---")
    st.subheader("➕ Crear Nuevo Evento")
    
    with st.form(f"create_event_{st.session_state.selected_date.strftime('%Y%m%d')}", clear_on_submit=True):
        event_title = st.text_input("Título del evento*", placeholder="Reunión, Cita, Recordatorio...")
        event_description = st.text_area("Descripción", placeholder="Detalles del evento...")
        
        col1, col2 = st.columns(2)
        with col1:
            event_start_time = st.time_input("Hora de inicio", value=datetime.strptime("00:00", "%H:%M").time())
        with col2:
            event_end_time = st.time_input("Hora de fin", value=datetime.strptime("00:15", "%H:%M").time())
        
        event_start_datetime = datetime.combine(st.session_state.selected_date.date(), event_start_time)
        event_end_datetime = datetime.combine(st.session_state.selected_date.date(), event_end_time)
        
        create_button = st.form_submit_button("🎯 Crear Evento", use_container_width=True)
        
        if create_button:
            current_time = time.time()
            if (st.session_state.form_submitted or 
                current_time - st.session_state.last_submission_time < 3):
                st.warning("⏳ Procesando evento anterior...")
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
                        st.info("⏳ " + response_data.get("message", "Evento en proceso..."))
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

def render_sidebar():
    """Renderizar sidebar mínimo"""
    st.sidebar.title("📅 Agenda Distribuida")
    st.sidebar.markdown("---")
    
    # Estado de autenticación
    if st.session_state.user_id:
        st.sidebar.write(f"**Usuario:** {st.session_state.user_username or st.session_state.user_email}")
        
        # Botón para recargar eventos
        if st.sidebar.button("🔄 Actualizar Eventos", use_container_width=True):
            load_events()
            st.sidebar.success("Eventos actualizados")
    else:
        st.sidebar.info("🔐 No has iniciado sesión")
        st.sidebar.write("Usando modo de prueba con user_test")

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

def main():
    """Función principal"""
    init_session_state()
    render_sidebar()
    
    # Título principal
    st.title("📅 Mi Agenda")
    
    # Si el usuario está autenticado, mostrar calendario
    if st.session_state.user_id or True:  # Temporal: siempre mostrar para pruebas
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
        ## 🎯 Bienvenido a tu Agenda Distribuida
        
        **Organiza tus eventos y mantente sincronizado**
        
        👈 **Usa el sidebar para ir a la página de autenticación**
        """)

if __name__ == "__main__":
    main()
