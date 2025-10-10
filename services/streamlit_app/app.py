import streamlit as st
import requests
import json
from datetime import datetime, timedelta
import pandas as pd

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
    if 'access_token' not in st.session_state:
        st.session_state.access_token = None
    if 'user_data' not in st.session_state:
        st.session_state.user_data = None

def make_api_request(endpoint, method="GET", data=None):
    """Realizar peticiones al API Gateway"""
    url = f"{API_GATEWAY_URL}{endpoint}"
    headers = {"Content-Type": "application/json"}
    
    if st.session_state.access_token:
        headers["Authorization"] = f"Bearer {st.session_state.access_token}"
    
    try:
        if method == "GET":
            response = requests.get(url, headers=headers, timeout=10)
        elif method == "POST":
            response = requests.post(url, json=data, headers=headers, timeout=10)
        elif method == "PUT":
            response = requests.put(url, json=data, headers=headers, timeout=10)
        elif method == "DELETE":
            response = requests.delete(url, headers=headers, timeout=10)
        
        return response
    except requests.exceptions.RequestException as e:
        st.error(f"Error de conexión: {e}")
        return None

def login_section():
    """Sección de login/registro"""
    st.sidebar.title("🔐 Autenticación")
    
    tab1, tab2 = st.sidebar.tabs(["Login", "Registro"])
    
    with tab1:
        with st.form("login_form"):
            email = st.text_input("Email")
            password = st.text_input("Contraseña", type="password")
            login_btn = st.form_submit_button("Iniciar Sesión")
            
            if login_btn:
                response = make_api_request(
                    "/api/v1/users/login", 
                    "POST", 
                    {"email": email, "password": password}
                )
                
                if response and response.status_code == 202:
                    st.success("✅ Login en proceso... Revisa tu servicio de usuarios")
                    # Aquí procesarías la respuesta cuando esté lista
                else:
                    st.error("❌ Error en el login")
    
    with tab2:
        with st.form("register_form"):
            email = st.text_input("Email de registro")
            username = st.text_input("Username")
            password = st.text_input("Contraseña de registro", type="password")
            register_btn = st.form_submit_button("Registrarse")
            
            if register_btn:
                response = make_api_request(
                    "/api/v1/users/register",
                    "POST",
                    {"email": email, "password": password, "username": username}
                )
                
                if response and response.status_code == 202:
                    st.success("✅ Registro en proceso... Revisa tu servicio de usuarios")
                else:
                    st.error("❌ Error en el registro")

def events_section():
    """Sección de gestión de eventos"""
    st.header("📅 Gestión de Eventos")
    
    col1, col2 = st.columns([1, 2])
    
    with col1:
        st.subheader("Crear Nuevo Evento")
        with st.form("create_event_form"):
            title = st.text_input("Título del evento")
            description = st.text_area("Descripción")
            start_time = st.datetime_input("Fecha y hora de inicio", datetime.now())
            end_time = st.datetime_input("Fecha y hora de fin", datetime.now() + timedelta(hours=1))
            user_id = st.text_input("User ID", "user123")  # Temporal hasta tener auth real
            
            create_btn = st.form_submit_button("Crear Evento")
            
            if create_btn:
                event_data = {
                    "title": title,
                    "description": description,
                    "start_time": start_time.isoformat(),
                    "end_time": end_time.isoformat(),
                    "user_id": user_id
                }
                
                response = make_api_request("/api/v1/events", "POST", event_data)
                
                if response and response.status_code == 202:
                    st.success("✅ Evento creado y en proceso de validación")
                    st.json(response.json())
                else:
                    st.error("❌ Error creando el evento")
    
    with col2:
        st.subheader("Eventos Recientes")
        # Aquí iría la lista de eventos cuando implementes los endpoints GET
        st.info("La lista de eventos se cargará cuando implementes los endpoints GET")

def dashboard_section():
    """Dashboard principal"""
    st.title("🏠 Dashboard - Agenda Distribuida")
    
    # Métricas rápidas
    col1, col2, col3 = st.columns(3)
    
    with col1:
        st.metric("Eventos Hoy", "0", "0")  # Placeholder
    
    with col2:
        st.metric("Grupos Activos", "0", "0")  # Placeholder
    
    with col3:
        st.metric("Notificaciones", "0", "0")  # Placeholder
    
    # Gráfico de eventos por día (placeholder)
    st.subheader("Eventos por Día")
    events_data = pd.DataFrame({
        'Día': ['Lun', 'Mar', 'Mié', 'Jue', 'Vie'],
        'Eventos': [2, 3, 1, 4, 2]
    })
    st.bar_chart(events_data.set_index('Día'))

def main():
    """Función principal de la app"""
    init_session_state()
    
    # Sidebar con navegación
    st.sidebar.title("📅 Agenda Distribuida")
    
    # Menú de navegación
    if st.session_state.access_token:
        page = st.sidebar.radio(
            "Navegación",
            ["Dashboard", "Eventos", "Grupos", "Configuración"]
        )
    else:
        page = "Dashboard"
    
    # Mostrar sección según la página
    if page == "Dashboard":
        dashboard_section()
    elif page == "Eventos":
        events_section()
    elif page == "Grupos":
        st.header("👥 Gestión de Grupos")
        st.info("Funcionalidad de grupos en desarrollo...")
    elif page == "Configuración":
        st.header("⚙️ Configuración")
        st.info("Configuración de la aplicación...")

if __name__ == "__main__":
    main()