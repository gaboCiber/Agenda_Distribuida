"""Utilidades para las llamadas a la API"""

import streamlit as st
import requests

# Configuración
API_GATEWAY_URL = "http://agenda-api-gateway:8000"

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