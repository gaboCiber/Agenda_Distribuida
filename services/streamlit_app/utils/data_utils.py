# Utilidades para manejo de datos
import streamlit as st
from utils.api_utils import make_api_request

def load_groups():
    """Cargar grupos del usuario actual"""
    if not st.session_state.user_id:
        st.session_state.groups = []
        return

    try:
        # Usar el endpoint correcto del API Gateway
        response = make_api_request("/api/v1/groups")

        if response and response.status_code == 200:
            data = response.json()
            st.session_state.groups = data.get('groups', [])
        else:
            st.session_state.groups = []
    except Exception as e:
        st.error(f"Error cargando grupos: {e}")
        st.session_state.groups = []

def load_group_invitations():
    """Cargar invitaciones del usuario"""
    if not st.session_state.user_id:
        st.session_state.group_invitations = []
        return

    try:
        response = make_api_request("/api/v1/groups/invitations")

        if response and response.status_code == 200:
            st.session_state.group_invitations = response.json()
        else:
            st.session_state.group_invitations = []
    except Exception as e:
        st.error(f"Error cargando invitaciones: {e}")
        st.session_state.group_invitations = []

def load_group_details(group_id):
    """Cargar detalles completos de un grupo"""
    try:
        # Cargar información del grupo
        group_response = make_api_request(f"/api/v1/groups/{group_id}")
        if group_response and group_response.status_code == 200:
            group_data = group_response.json()

            # Cargar miembros
            members_response = make_api_request(f"/api/v1/groups/{group_id}/members")
            if members_response and members_response.status_code == 200:
                group_data['members'] = members_response.json().get('members', [])

            # Cargar eventos
            events_response = make_api_request(f"/api/v1/groups/{group_id}/events")
            if events_response and events_response.status_code == 200:
                group_data['events'] = events_response.json()

            return group_data
    except Exception as e:
        st.error(f"Error cargando detalles del grupo: {e}")
    return None