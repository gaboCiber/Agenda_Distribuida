"""Utilidades para el manejo del estado de sesión de Streamlit"""

import streamlit as st
from datetime import datetime

def init_session_state():
    """Inicializar el estado de la sesión - ACTUALIZADA"""
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
    if 'adding_to_group' not in st.session_state:
        st.session_state.adding_to_group = None
    if 'creating_group_event' not in st.session_state:
        st.session_state.creating_group_event = None