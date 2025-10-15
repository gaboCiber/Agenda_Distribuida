# Utilidades para llamadas a la API
import streamlit as st
import requests

API_GATEWAY_URL = "http://agenda-api-gateway:8000"

def make_api_request(endpoint, method="GET", data=None, headers=None):
    """Realizar peticiones al API Gateway con mejor manejo de errores y debug"""
    url = f"{API_GATEWAY_URL}{endpoint}"

    default_headers = {"Content-Type": "application/json"}
    if headers:
        default_headers.update(headers)

    # Agregar el ID de usuario a los headers si está disponible
    if 'user_id' in st.session_state and st.session_state.user_id:
        default_headers["X-User-ID"] = st.session_state.user_id

    # DEBUG: Mostrar información de la petición
    print(f"🔍 DEBUG API: {method} {url}")
    print(f"🔍 DEBUG API: Headers: {default_headers}")
    if data:
        print(f"🔍 DEBUG API: Data: {data}")

    try:
        if method == "GET":
            response = requests.get(url, headers=default_headers, timeout=10)
        elif method == "POST":
            response = requests.post(url, json=data, headers=default_headers, timeout=10)
        elif method == "PUT":
            response = requests.put(url, json=data, headers=default_headers, timeout=10)
        elif method == "DELETE":
            response = requests.delete(url, headers=default_headers, timeout=10)

        # DEBUG: Mostrar respuesta
        print(f"🔍 DEBUG API: Response Status: {response.status_code}")
        print(f"🔍 DEBUG API: Response Headers: {dict(response.headers)}")

        try:
            response_data = response.json()
            print(f"🔍 DEBUG API: Response Data: {response_data}")
        except:
            print(f"🔍 DEBUG API: Response Text: {response.text[:200]}...")

        return response
    except Exception as e:
        print(f"❌ DEBUG API: Exception: {e}")
        import traceback
        print(f"❌ DEBUG API: Traceback: {traceback.format_exc()}")
        st.error(f"Error de conexión: {e}")
        return None

def init_session_state():
    """Inicializar estado de la sesión para grupos"""
    defaults = {
        'groups': [],
        'selected_group': None,
        'group_invitations': [],
        'show_create_group': False,
        'show_invite_form': None,
        'inviting_to_group': None,
        'create_group_status': None,
        'invite_status': None,
        'member_action_status': None,
        'group_events': [],
        'active_tab': 'Mis Grupos',
        'editing_group': False
    }

    for key, default_value in defaults.items():
        if key not in st.session_state:
            st.session_state[key] = default_value