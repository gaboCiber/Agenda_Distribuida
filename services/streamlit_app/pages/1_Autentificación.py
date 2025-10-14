import streamlit as st
import requests
import time

st.set_page_config(
    page_title="Autenticación",
    page_icon="🔐",
    layout="centered"
)

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

def init_session_state():
    """Inicializar el estado de la sesión"""
    if 'user_id' not in st.session_state:
        st.session_state.user_id = None
    if 'user_email' not in st.session_state:
        st.session_state.user_email = None
    if 'user_username' not in st.session_state:
        st.session_state.user_username = None
    if 'show_register' not in st.session_state:
        st.session_state.show_register = False

# Inicializar estado
init_session_state()

st.title("🔐 Autenticación")

# 🔍 VERIFICAR SI EL USUARIO YA ESTÁ AUTENTICADO
if st.session_state.user_id:
    # USUARIO AUTENTICADO - Mostrar información de la cuenta y opción para cerrar sesión
    st.success(f"✅ ¡Hola {st.session_state.user_username or st.session_state.user_email}!")
    
    st.subheader("📋 Información de tu Cuenta")
    
    col1, col2 = st.columns(2)
    
    with col1:
        st.info(f"**Email:** {st.session_state.user_email}")
        if st.session_state.user_username:
            st.info(f"**Usuario:** {st.session_state.user_username}")
    
    with col2:
        st.info("**Estado:** ✅ Sesión activa")
        st.info("**Último acceso:** Ahora mismo")
    
    st.markdown("---")
    
    # Opciones para el usuario autenticado
    st.subheader("⚙️ Opciones de Cuenta")
    

    if st.button("🚪 Cerrar Sesión", use_container_width=True, type="secondary"):
            # Limpiar sesión
            st.session_state.user_id = None
            st.session_state.user_email = None
            st.session_state.user_username = None
            st.session_state.show_register = False
            st.success("✅ Sesión cerrada correctamente")
            st.rerun()
    
    st.markdown("---")
    st.info("💡 Puedes navegar a tu agenda desde el sidebar o usando los botones arriba")

else:
    # USUARIO NO AUTENTICADO - Mostrar formularios de login/registro
    st.info("🔐 Inicia sesión o crea una cuenta para acceder a tu agenda")
    
    # Contenedor principal para formularios
    with st.container():
        # Determinar qué formulario mostrar
        if st.session_state.show_register:
            # FORMULARIO DE REGISTRO
            st.subheader("📝 Crear Cuenta")
            
            with st.form("register_form"):
                email = st.text_input("📧 Email", placeholder="tu@email.com")
                username = st.text_input("👤 Username (opcional)", placeholder="tu_usuario")
                password = st.text_input("🔒 Contraseña", type="password", placeholder="Mínimo 8 caracteres")
                confirm_password = st.text_input("🔒 Confirmar Contraseña", type="password", placeholder="Repite tu contraseña")
                
                register_btn = st.form_submit_button("🎯 Crear Cuenta", use_container_width=True)
                
                if register_btn:
                    if not email or not password:
                        st.error("❌ Email y contraseña son obligatorios")
                    elif password != confirm_password:
                        st.error("❌ Las contraseñas no coinciden")
                    elif len(password) < 8:
                        st.error("❌ La contraseña debe tener al menos 8 caracteres")
                    else:
                        response = make_api_request(
                            "/api/v1/users/register",
                            "POST",
                            {
                                "email": email, 
                                "password": password, 
                                "username": username if username else email.split('@')[0]
                            }
                        )
                        
                        if response and response.status_code == 202:
                            st.success("✅ ¡Cuenta creada exitosamente!")
                            
                            # Establecer sesión
                            st.session_state.user_id = "user_" + email.split('@')[0]
                            st.session_state.user_email = email
                            st.session_state.user_username = username if username else email.split('@')[0]
                            st.session_state.show_register = False
                            
                            st.success("🎉 ¡Ya puedes acceder a tu agenda!")
                            st.info("👈 Usa el sidebar para ir a la página principal")
                            
                        else:
                            st.error("❌ Error creando la cuenta. El email ya podría estar registrado.")
            
            # Enlace para cambiar a Login
            st.markdown("---")
            st.write("¿Ya tienes una cuenta?")
            if st.button("🚪 Iniciar Sesión", use_container_width=True, type="primary"):
                st.session_state.show_register = False
                st.rerun()
        
        else:
            # FORMULARIO DE LOGIN
            st.subheader("🚪 Iniciar Sesión")
            
            with st.form("login_form"):
                email = st.text_input("📧 Email", placeholder="tu@email.com")
                password = st.text_input("🔒 Contraseña", type="password", placeholder="Tu contraseña")
                
                login_btn = st.form_submit_button("🎯 Entrar a Mi Agenda", use_container_width=True)
                
                if login_btn:
                    if not email or not password:
                        st.error("❌ Email y contraseña son obligatorios")
                    else:
                        response = make_api_request(
                            "/api/v1/users/login", 
                            "POST", 
                            {"email": email, "password": password}
                        )
                        
                        if response and response.status_code == 202:
                            st.success("✅ ¡Bienvenido de vuelta!")
                            
                            # Establecer sesión
                            st.session_state.user_id = "user_" + email.split('@')[0]
                            st.session_state.user_email = email
                            st.session_state.user_username = email.split('@')[0]
                            
                            st.success("🎉 ¡Sesión iniciada correctamente!")
                            st.info("👈 Usa el sidebar para ir a tu agenda")
                            
                        else:
                            st.error("❌ Email o contraseña incorrectos")
            
            # Enlace para cambiar a Registro
            st.markdown("---")
            st.write("¿No tienes una cuenta?")
            if st.button("📝 Registrarse", use_container_width=True, type="primary"):
                st.session_state.show_register = True
                st.rerun()

    # Información adicional solo para usuarios no autenticados
    st.markdown("---")
    st.info("""
    **💡 ¿Primera vez aquí?**
    - Crea una cuenta para empezar a gestionar tus eventos
    - Tu información está segura con nuestro sistema de autenticación
    - Podrás acceder a tu agenda desde cualquier dispositivo
    """)