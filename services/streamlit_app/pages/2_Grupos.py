import streamlit as st
import requests
from datetime import datetime, timedelta
import time

st.set_page_config(
    page_title="Gestión de Grupos",
    page_icon="👥",
    layout="wide"
)

API_GATEWAY_URL = "http://agenda-api-gateway:8000"

def make_api_request(endpoint, method="GET", data=None):
    """Realizar peticiones al API Gateway con mejor manejo de errores"""
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

def init_session_state():
    """Inicializar el estado de la sesión para grupos"""
    if 'groups' not in st.session_state:
        st.session_state.groups = []
    if 'selected_group' not in st.session_state:
        st.session_state.selected_group = None
    if 'group_invitations' not in st.session_state:
        st.session_state.group_invitations = []
    if 'show_create_group' not in st.session_state:
        st.session_state.show_create_group = False
    if 'show_invite_form' not in st.session_state:
        st.session_state.show_invite_form = None
    if 'inviting_to_group' not in st.session_state:
        st.session_state.inviting_to_group = None
    if 'invite_form_state' not in st.session_state:
        st.session_state.invite_form_state = {
            'submitted': False,
            'error': None,
            'email': ''
        }

def load_groups():
    """Cargar grupos del usuario actual"""
    if not st.session_state.user_id:
        st.warning("🔒 Debes iniciar sesión para ver tus grupos")
        st.session_state.groups = []
        return

    try:
        # Usar el endpoint correcto para obtener los grupos del usuario
        response = make_api_request(f"/api/v1/groups/users/{st.session_state.user_id}/groups", "GET")

        if response is None:
            st.error("❌ No se pudo conectar con el servidor")
            st.session_state.groups = []
            return

        if response.status_code == 200:
            try:
                data = response.json()
                # Asegurarse de que siempre sea una lista
                st.session_state.groups = data.get('groups', []) or []
                print(f"🔧 DEBUG: Se cargaron {len(st.session_state.groups)} grupos para usuario {st.session_state.user_id}")
                return
            except ValueError as e:
                print(f"🔧 DEBUG: Error parsing JSON response: {e}")
                st.error("❌ Error al procesar la respuesta del servidor")
                st.session_state.groups = []
                return

        # Si el endpoint específico falla, intentar el endpoint genérico
        print(f"🔧 DEBUG: Endpoint específico falló ({response.status_code}), intentando con endpoint genérico")
        response = make_api_request("/api/v1/groups", "GET")

        if response and response.status_code == 200:
            try:
                data = response.json()
                # Filtrar manualmente los grupos donde el usuario es miembro
                all_groups = data.get('groups', []) or []
                st.session_state.groups = [
                    group for group in all_groups
                    if any(member.get('user_id') == st.session_state.user_id
                          for member in group.get('members', []))
                ]
                print(f"🔧 DEBUG: Se filtraron {len(st.session_state.groups)} grupos para usuario {st.session_state.user_id}")
            except (ValueError, AttributeError) as e:
                print(f"🔧 DEBUG: Error parsing or filtering groups: {e}")
                st.error("❌ Error al procesar la lista de grupos")
                st.session_state.groups = []
        else:
            error_msg = response.text if response else "Sin respuesta del servidor"
            print(f"🔧 DEBUG: Error al cargar grupos: {error_msg}")
            st.session_state.groups = []

    except Exception as e:
        print(f"🔧 DEBUG: Excepción en load_groups: {str(e)}")
        st.error(f"❌ Error inesperado al cargar grupos: {str(e)}")
        st.session_state.groups = []

def load_group_invitations():
    """Cargar invitaciones de grupo del usuario actual"""
    if not st.session_state.user_id:
        st.session_state.group_invitations = []
        return

    try:
        # Usar el endpoint específico del usuario si está disponible
        response = make_api_request(f"/api/v1/users/{st.session_state.user_id}/group-invitations", "GET")

        if response is None:
            st.error("❌ No se pudo conectar con el servidor")
            st.session_state.group_invitations = []
            return

        if response.status_code == 200:
            st.session_state.group_invitations = response.json().get('invitations', [])
            print(f"🔧 DEBUG: Se cargaron {len(st.session_state.group_invitations)} invitaciones para el usuario {st.session_state.user_id}")
        elif response.status_code == 404:
            # Si el endpoint específico no existe, intentar con el endpoint genérico
            response = make_api_request("/api/v1/groups/invitations", "GET")
            if response and response.status_code == 200:
                all_invitations = response.json().get('invitations', [])
                # Filtrar manualmente las invitaciones del usuario actual
                st.session_state.group_invitations = [
                    inv for inv in all_invitations
                    if inv.get('user_id') == st.session_state.user_id
                ]
                print(f"🔧 DEBUG: Se filtraron {len(st.session_state.group_invitations)} invitaciones para el usuario {st.session_state.user_id}")
            else:
                st.session_state.group_invitations = []
        else:
            st.error(f"❌ Error al cargar invitaciones: {response.text}")
            st.session_state.group_invitations = []

    except Exception as e:
        st.error(f"❌ Error al cargar invitaciones: {str(e)}")
        st.session_state.group_invitations = []
        print(f"🔧 DEBUG: Excepción en load_group_invitations: {str(e)}")

def respond_to_invitation(invitation_id: str, action: str):
    """Responder a una invitación de grupo"""
    data = {"action": action}
    response = make_api_request(f"/api/v1/groups/invitations/{invitation_id}/respond", "POST", data)

    if response and response.status_code == 200:
        st.sidebar.success(f"Invitación {action}ada correctamente")
        load_group_invitations()  # Recargar invitaciones
    else:
        st.sidebar.error("Error al procesar la invitación")

def render_create_group_form():
    """Renderizar formulario para crear grupo"""
    st.markdown("### ➕ Crear Nuevo Grupo")

    # Usar una clave única para el formulario
    if 'form_key' not in st.session_state:
        st.session_state.form_key = f"create_group_form_{time.time()}"

    # Inicializar el estado del formulario
    if 'form_submitted' not in st.session_state:
        st.session_state.form_submitted = False

    with st.form(key=st.session_state.form_key):
        group_name = st.text_input(
            "Nombre del Grupo*",
            placeholder="Ej: Equipo de Desarrollo",
            key=f"group_name_{st.session_state.form_key}"
        )
        group_description = st.text_area(
            "Descripción",
            placeholder="Describe el propósito del grupo...",
            key=f"group_desc_{st.session_state.form_key}"
        )

        col1, col2 = st.columns(2)
        with col1:
            create_btn = st.form_submit_button("🎯 Crear Grupo", use_container_width=True, type="primary")
        with col2:
            cancel_btn = st.form_submit_button("❌ Cancelar", use_container_width=True, type="secondary")

        # Si el formulario ya fue enviado, no hacer nada
        if st.session_state.form_submitted:
            st.session_state.form_submitted = False
            st.rerun()
            return False

        if create_btn and group_name:
            try:
                # Marcar el formulario como enviado
                st.session_state.form_submitted = True

                # Crear grupo
                group_data = {
                    "name": group_name,
                    "description": group_description
                }

                print(f"🔧 DEBUG: Enviando solicitud para crear grupo: {group_data}")
                response = make_api_request("/api/v1/groups", "POST", group_data)
                print(f"🔧 DEBUG: Respuesta del servidor: {response.status_code if response else 'No response'}")

                if response and response.status_code == 201:
                    st.success("✅ Grupo creado exitosamente!")
                    # Limpiar el estado del formulario
                    st.session_state.show_create_group = False
                    # Recargar la lista de grupos
                    load_groups()
                    # Generar una nueva clave para el formulario
                    st.session_state.form_key = f"create_group_form_{time.time()}"
                    # Forzar recarga
                    st.rerun()
                else:
                    error_msg = response.text if response else "No se pudo conectar al servidor"
                    st.error(f"❌ Error al crear el grupo: {error_msg}")

            except Exception as e:
                st.error(f"❌ Error inesperado: {str(e)}")
                print(f"🔧 DEBUG: Error al crear grupo: {str(e)}")

            return False

        if cancel_btn:
            st.session_state.show_create_group = False
            st.rerun()

        return False

def render_group_detail():
    """Renderizar detalles de un grupo específico"""
    if not st.session_state.selected_group:
        return

    group = st.session_state.selected_group
    st.title(f"📋 {group['name']}")

    # Botón para volver
    if st.button("⬅️ Volver a Grupos", use_container_width=True):
        st.session_state.selected_group = None
        st.session_state.show_group_management = True
        load_groups()  # Recargar grupos para asegurar que tenemos la información más reciente
        st.rerun()

    # Información del grupo
    col1, col2 = st.columns([2, 1])

    with col1:
        st.markdown(f"**Descripción:** {group.get('description', 'Sin descripción')}")
        st.markdown(f"**Creado por:** {group.get('created_by', 'Desconocido')}")
        st.markdown(f"**Fecha de creación:** {group['created_at'][:10]}")

    with col2:
        st.markdown(f"**Miembros:** {group.get('member_count', 0)}")

        # Invitar miembros
        if st.button("➕ Invitar Miembro", use_container_width=True):
            st.session_state.inviting_to_group = group['id']
            st.rerun()

    # Gestión de invitaciones
    if st.session_state.get('inviting_to_group') == group['id']:
        render_invite_member_form(group)

    # Lista de miembros
    st.markdown("### 👥 Miembros del Grupo")
    render_group_members(group['id'])

    # Eventos del grupo
    st.markdown("### 📅 Eventos del Grupo")
    render_group_events(group['id'])

def render_invite_member_form(group):
    """Renderizar formulario para invitar miembros"""
    form_key = f"invite_form_{group['id']}"

    # Inicializar el estado del formulario si no existe
    if 'invite_form_state' not in st.session_state:
        st.session_state.invite_form_state = {
            'submitted': False,
            'error': None,
            'email': ''
        }

    # Mostrar mensaje de error si existe
    if st.session_state.invite_form_state['error']:
        st.error(st.session_state.invite_form_state['error'])

    # Si el formulario ya fue enviado con éxito
    if st.session_state.invite_form_state['submitted']:
        st.success("✅ Invitación enviada exitosamente!")
        if st.button("Cerrar", key=f"close_invite_form_{group['id']}"):
            st.session_state.inviting_to_group = None
            st.session_state.invite_form_state = {'submitted': False, 'error': None, 'email': ''}
            st.rerun()
        return

    # Formulario de invitación
    with st.form(form_key, clear_on_submit=True):
        user_email = st.text_input(
            "Email del usuario a invitar*",
            placeholder="usuario@email.com",
            key=f"invite_email_{group['id']}",
            value=st.session_state.invite_form_state.get('email', '')
        )

        col1, col2 = st.columns(2)
        with col1:
            invite_btn = st.form_submit_button("📨 Enviar Invitación", use_container_width=True)
        with col2:
            cancel_btn = st.form_submit_button("❌ Cancelar", use_container_width=True)

        if cancel_btn:
            st.session_state.inviting_to_group = None
            st.session_state.invite_form_state = {'submitted': False, 'error': None, 'email': ''}
            st.rerun()

        if invite_btn and user_email:
            st.session_state.invite_form_state['email'] = user_email
            try:
                # Buscar el ID del usuario por su email
                user_response = make_api_request(f"/api/v1/users/email/{user_email}", "GET")

                if user_response and user_response.status_code == 200:
                    user_data = user_response.json()
                    user_id = user_data.get('id')

                    if user_id:
                        # Verificar si el usuario ya es miembro del grupo
                        members_response = make_api_request(f"/api/v1/groups/{group['id']}/members", "GET")
                        if members_response and members_response.status_code == 200:
                            members = members_response.json().get('members', [])
                            if any(member['user_id'] == user_id for member in members):
                                st.session_state.invite_form_state['error'] = "❌ Este usuario ya es miembro del grupo"
                                st.rerun()

                        # Crear invitación con el ID del usuario
                        invite_data = {
                            "group_id": group['id'],
                            "user_id": user_id
                        }

                        response = make_api_request("/api/v1/groups/invitations", "POST", invite_data)

                        if response and response.status_code == 201:
                            st.session_state.invite_form_state = {
                                'submitted': True,
                                'error': None,
                                'email': ''
                            }
                            st.rerun()
                        else:
                            error_msg = response.json().get('detail', 'Error desconocido') if response else "No se pudo conectar con el servidor"
                            st.session_state.invite_form_state['error'] = f"❌ Error al enviar la invitación: {error_msg}"
                            st.rerun()
                    else:
                        st.session_state.invite_form_state['error'] = "❌ No se pudo obtener el ID del usuario"
                        st.rerun()
                else:
                    st.session_state.invite_form_state['error'] = "❌ No se encontró ningún usuario con ese correo electrónico"
                    st.rerun()

            except Exception as e:
                st.session_state.invite_form_state['error'] = f"❌ Error inesperado: {str(e)}"
                st.rerun()
                st.rerun()

def render_group_members(group_id):
    """Renderizar lista de miembros del grupo"""
    response = make_api_request(f"/api/v1/groups/{group_id}/members", "GET")

    if response and response.status_code == 200:
        members_data = response.json()
        members = members_data.get('members', [])

        if not members:
            st.info("Este grupo no tiene miembros aún")
        else:
            for member in members:
                col1, col2, col3 = st.columns([2, 1, 1])

                with col1:
                    st.write(f"**{member['user_id']}**")
                    st.caption(f"Rol: {member['role']}")

                with col2:
                    st.caption(f"Unido: {member['joined_at'][:10]}")

                with col3:
                    if member['role'] != 'admin':  # No permitir remover admins
                        if st.button("❌", key=f"remove_{member['id']}", help="Remover miembro"):
                            remove_member(group_id, member['user_id'])
    else:
        st.error("Error al cargar miembros del grupo")

def render_group_events(group_id):
    """Renderizar eventos del grupo"""
    response = make_api_request(f"/api/v1/groups/{group_id}/events", "GET")

    if response and response.status_code == 200:
        events = response.json()

        if not events:
            st.info("Este grupo no tiene eventos programados")
        else:
            for event in events:
                with st.expander(f"📅 Evento: {event.get('event_id', 'Desconocido')}", expanded=False):
                    st.write(f"**Agregado por:** {event.get('added_by', 'Desconocido')}")
                    st.write(f"**Fecha de agregado:** {event.get('added_at', 'Desconocida')[:10]}")

                    if st.button("🗑️ Remover del Grupo", key=f"remove_event_{event['event_id']}", use_container_width=True):
                        remove_event_from_group(group_id, event['event_id'])
    else:
        st.error("Error al cargar eventos del grupo")

def remove_member(group_id: str, member_id: str):
    """Remover un miembro del grupo"""
    response = make_api_request(f"/api/v1/groups/{group_id}/members/{member_id}", "DELETE")

    if response and response.status_code == 204:
        st.success("✅ Miembro removido exitosamente")
        # Recargar detalles del grupo
        load_groups()
    else:
        st.error("❌ Error al remover miembro")

def remove_event_from_group(group_id: str, event_id: str):
    """Remover un evento del grupo"""
    response = make_api_request(f"/api/v1/groups/{group_id}/events/{event_id}", "DELETE")

    if response and response.status_code == 204:
        st.success("✅ Evento removido del grupo exitosamente")
        st.rerun()
    else:
        st.error("❌ Error al remover evento del grupo")

def main():
    """Función principal de la página de grupos"""
    init_session_state()

    # Verificar autenticación
    if not st.session_state.user_id:
        st.warning("🔒 Debes iniciar sesión para acceder a la gestión de grupos")
        st.info("👈 Ve a la página de autenticación desde el sidebar")
        return

    st.title("👥 Gestión de Grupos")

    # Pestañas para organización
    tabs = ["📋 Mis Grupos", "➕ Crear Grupo", "📨 Invitaciones"]
    if 'active_tab' not in st.session_state:
        st.session_state.active_tab = tabs[0]

    # Crear pestañas
    tab1, tab2, tab3 = st.tabs(tabs)

    # Actualizar pestaña activa
    for i, tab in enumerate(tabs):
        if st.session_state.active_tab == tab:
            st.session_state.active_tab_index = i
            break

    with tab1:
        st.subheader("Mis Grupos")

        # Cargar grupos del usuario
        load_groups()

        # Mostrar mensaje si no hay grupos
        if not st.session_state.groups:
            st.info("No perteneces a ningún grupo. Crea uno nuevo o acepta una invitación.")
        else:
            # Mostrar tarjetas de grupos
            for group in st.session_state.groups:
                with st.container():
                    st.markdown("---")
                    col1, col2 = st.columns([4, 1])

                    with col1:
                        st.markdown(f"### {group.get('name', 'Sin nombre')}")
                        st.caption(f"ID: {group.get('id')}")
                        st.caption(f"Creado por: {group.get('created_by', 'Desconocido')}")

                        if group.get('description'):
                            st.markdown(f"*{group.get('description')}*")

                        # Mostrar información del grupo
                        members = group.get('members', [])
                        st.markdown(f"**Miembros:** {len(members)}")

                        # Mostrar eventos del grupo si los hay
                        events = group.get('events', [])
                        if events:
                            st.markdown(f"**Eventos programados:** {len(events)}")

                    with col2:
                        # Botón para ver detalles del grupo
                        if st.button("Ver detalles", key=f"view_{group['id']}", use_container_width=True):
                            st.session_state.selected_group = group
                            st.session_state.show_group_management = False
                            st.rerun()

                        # Botón para invitar miembros
                        if st.button("Invitar", key=f"invite_{group['id']}", use_container_width=True):
                            st.session_state.show_invite_form = group['id']
                            st.rerun()

            # Botón para actualizar la lista de grupos
            if st.button("🔄 Actualizar lista de grupos"):
                load_groups()

    with tab2:
        st.subheader("Crear Nuevo Grupo")
        render_create_group_form()

    with tab3:
        st.subheader("Invitaciones Pendientes")
        load_group_invitations()

        if not st.session_state.group_invitations:
            st.info("No tienes invitaciones pendientes.")
        else:
            for invite in st.session_state.group_invitations:
                with st.container():
                    st.markdown("---")
                    col1, col2 = st.columns([3, 1])

                    with col1:
                        st.markdown(f"### {invite.get('group_name', 'Grupo Desconocido')}")
                        st.markdown(f"*Invitado por: {invite.get('invited_by', 'Usuario desconocido')}*")

                        if invite.get('message'):
                            st.markdown(f"*Mensaje: {invite.get('message')}*")

                    with col2:
                        # Botones de acción
                        if st.button("✅ Aceptar",
                                    key=f"accept_{invite['id']}",
                                    use_container_width=True):
                            respond_to_invitation(invite['id'], "accept")
                            st.rerun()

                        if st.button("❌ Rechazar",
                                    key=f"reject_{invite['id']}",
                                    use_container_width=True):
                            respond_to_invitation(invite['id'], "reject")
                            st.rerun()

            # Botón para actualizar invitaciones
            if st.button("🔄 Actualizar invitaciones"):
                load_group_invitations()

    # Mostrar detalles del grupo si está seleccionado
    if st.session_state.selected_group:
        render_group_detail()

if __name__ == "__main__":
    main()