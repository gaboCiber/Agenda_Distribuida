import streamlit as st
import requests
import json
import time
from datetime import datetime
import uuid

# Importar módulos separados
from utils.api_utils import make_api_request, init_session_state
from utils.data_utils import load_groups, load_group_invitations, load_group_details
from utils.action_utils import (
    create_group, update_group, delete_group, invite_member,
    remove_member, respond_to_invitation
)
from components.group_components import (
    render_group_card, render_create_group_form, render_invite_form,
    render_group_detail, render_members_management, render_events_management,
    render_group_settings, render_invitations
)

st.set_page_config(
    page_title="Gestión de Grupos",
    page_icon="👥",
    layout="wide"
)

API_GATEWAY_URL = "http://agenda-api-gateway:8000"

def main():
    """Función principal de la página de grupos"""
    init_session_state()

    # Verificar autenticación
    if not st.session_state.user_id:
        st.warning("🔒 Debes iniciar sesión para acceder a la gestión de grupos")
        st.info("👈 Ve a la página de autenticación desde el sidebar")
        return

    st.title("👥 Gestión de Grupos")

    # Pestañas principales
    tabs = st.tabs(["📋 Mis Grupos", "➕ Crear Grupo", "📨 Invitaciones"])

    with tabs[0]:
        st.markdown("### 📋 Mis Grupos")

        # Botón para recargar grupos
        col1, col2 = st.columns([4, 1])
        with col1:
            pass
        with col2:
            if st.button("🔄 Actualizar", use_container_width=True):
                load_groups()

        # Cargar y mostrar grupos
        load_groups()
        groups = st.session_state.groups

        if not groups:
            st.info("No perteneces a ningún grupo. ¡Crea uno nuevo!")
        else:
            for group in groups:
                render_group_card(group)

    with tabs[1]:
        render_create_group_form()

    with tabs[2]:
        render_invitations()

    # Mostrar formularios modales SOLO en la pestaña "Mis Grupos"
    # tabs[0] es la pestaña "Mis Grupos", tabs[1] es "Crear Grupo", tabs[2] es "Invitaciones"
    if st.session_state.show_invite_form:
        group = next((g for g in st.session_state.groups if g['id'] == st.session_state.show_invite_form), None)
        if group:
            render_invite_form(group)

    # Mostrar vista detallada si hay un grupo seleccionado
    if st.session_state.selected_group:
        render_group_detail()

if __name__ == "__main__":
    main()
