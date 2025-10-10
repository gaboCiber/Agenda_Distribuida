import streamlit as st
import requests

# Configuración de página - CADA ARCHIVO necesita esto
st.set_page_config(
    page_title="Autenticación",
    page_icon="🔐",
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
st.title("🔐 Autenticación")

col1, col2 = st.columns(2)

with col1:
    st.subheader("Registro")
    with st.form("register_form"):
        email = st.text_input("Email")
        username = st.text_input("Username")
        password = st.text_input("Contraseña", type="password")
        register_btn = st.form_submit_button("Registrarse")
        
        if register_btn:
            response = make_api_request(
                "/api/v1/users/register",
                "POST",
                {"email": email, "password": password, "username": username}
            )
            
            if response and response.status_code == 202:
                st.success("✅ Registro en proceso... Revisa tu servicio de usuarios")
                st.json(response.json())
            else:
                st.error("❌ Error en el registro")

with col2:
    st.subheader("Login")
    with st.form("login_form"):
        email = st.text_input("Email de login")
        password = st.text_input("Contraseña de login", type="password")
        login_btn = st.form_submit_button("Iniciar Sesión")
        
        if login_btn:
            response = make_api_request(
                "/api/v1/users/login", 
                "POST", 
                {"email": email, "password": password}
            )
            
            if response and response.status_code == 202:
                st.success("✅ Login en proceso... Revisa tu servicio de usuarios")
                st.json(response.json())
            else:
                st.error("❌ Error en el login")