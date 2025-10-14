import streamlit as st

# Configuración de la página
st.set_page_config(
    page_title="Agenda Distribuida",
    page_icon="🗓",
    layout="wide",
    initial_sidebar_state="expanded")

# Configuración
API_GATEWAY_URL = "http://agenda-api-gateway:8000"

# Importar utilidades y componentes
from utils.session import init_session_state
from utils.events import load_events
from components.sidebar import render_sidebar
from components.calendar import render_calendar
from components.event_form import render_selected_day_events

def main():
    """Función principal - Página de Agenda"""
    init_session_state()
    render_sidebar()

    # Título principal
    st.title("🗓 Mi Agenda")

    # Si el usuario está autenticado, mostrar calendario
    if st.session_state.user_id:
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
        ## Bienvenido a tu Agenda

        **Organiza tus eventos y mantente sincronizado**

        👈 **Usa el sidebar para ir a la página de autenticación**
        """)

if __name__ == "__main__":
    main()

