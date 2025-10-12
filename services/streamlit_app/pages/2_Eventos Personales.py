import streamlit as st
import requests
from datetime import datetime, timedelta

# Configuración de página
st.set_page_config(
    page_title="Eventos",
    page_icon="📅",
    layout="wide"
)

# Configuración
API_GATEWAY_URL = "http://agenda-api-gateway:8000"

def make_api_request(endpoint, method="GET", data=None):
    """Realizar peticiones al API Gateway"""
    url = f"{API_GATEWAY_URL}{endpoint}"
    headers = {"Content-Type": "application/json"}
    
    try:
        if method == "GET":
            response = requests.get(url, headers=headers, timeout=10)
        elif method == "POST":
            response = requests.post(url, json=data, headers=headers, timeout=10)
        return response
    except requests.exceptions.RequestException as e:
        st.error(f"Error de conexión: {e}")
        return None

# 🎯 CONTENIDO PRINCIPAL DE LA PÁGINA
st.title("📅 Gestión de Eventos")

col1, col2 = st.columns([1, 2])

with col1:
    st.subheader("Crear Nuevo Evento")
    with st.form("create_event_form"):
        title = st.text_input("Título del evento*")
        description = st.text_area("Descripción")
        start_time = st.datetime_input("Fecha y hora de inicio", datetime.now())
        end_time = st.datetime_input("Fecha y hora de fin", datetime.now() + timedelta(hours=1))
        user_id = st.text_input("User ID*", "user123")
        
        create_btn = st.form_submit_button("🎯 Crear Evento")
        
        if create_btn:
            if title and user_id:
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
            else:
                st.warning("⚠️ Completa los campos obligatorios (*)")

with col2:
    st.subheader("Tus Eventos")
    st.info("📋 La lista de eventos se cargará cuando implementes el endpoint GET /api/v1/events")
    
    # Placeholder para futura implementación
    if st.button("🔄 Cargar Eventos", disabled=True):
        st.write("Esta funcionalidad estará disponible pronto")