"""Componentes para el sidebar"""

import streamlit as st

def render_sidebar():
    """Renderizar sidebar con navegación a páginas"""
    st.sidebar.markdown("---")

    # Estado de autenticación
    if st.session_state.user_id:
        st.sidebar.write(f"**Usuario:** {st.session_state.user_username or st.session_state.user_email}")

    else:
        st.sidebar.info("🔐 No has iniciado sesión")