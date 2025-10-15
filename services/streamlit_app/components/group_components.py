# Componentes de UI para la gestión de grupos
import streamlit as st
import time
from utils.data_utils import load_groups, load_group_invitations, load_group_details
from utils.action_utils import create_group, update_group, delete_group, invite_member, remove_member, respond_to_invitation

def render_group_card(group):
    """Renderizar una tarjeta de grupo"""
    with st.container():
        st.markdown("---")
        col1, col2, col3 = st.columns([3, 1, 1])

        with col1:
            st.markdown(f"### {group.get('name', 'Sin nombre')}")
            #st.caption(f"ID: {group.get('id', '')}")

            if group.get('description'):
                st.markdown(f"*{group.get('description')}*")

            # Información del grupo
            members_count = group.get('member_count', 0)
            st.markdown(f"**👥 Miembros:** {members_count}")

            created_date = group.get('created_at', '')[:10] if group.get('created_at') else ''
            st.caption(f"Creado: {created_date}")

        with col2:
            if st.button("👁️ Ver", key=f"view_{group['id']}", use_container_width=True):
                st.session_state.selected_group = load_group_details(group['id'])
                st.session_state.active_tab = 'Grupo Detallado'
                # No hacer st.rerun() para evitar bucles infinitos

        with col3:
            if st.button("📧 Invitar", key=f"invite_{group['id']}", use_container_width=True):
                st.session_state.show_invite_form = group['id']
                st.session_state.active_tab = 'Invitar Miembro'
                # No hacer st.rerun() para evitar bucles infinitos

def render_create_group_form():
    """Formulario para crear grupo"""
    st.markdown("### ➕ Crear Nuevo Grupo")

    with st.form("create_group_form_main", clear_on_submit=True):
        name = st.text_input("Nombre del Grupo*", placeholder="Ej: Equipo de Desarrollo")
        description = st.text_area("Descripción", placeholder="Describe el propósito del grupo...")

        col1, col2 = st.columns(2)
        with col1:
            submitted = st.form_submit_button("🎯 Crear Grupo", use_container_width=True, type="primary")
        with col2:
            cancel = st.form_submit_button("❌ Cancelar", use_container_width=True)

        if submitted and name:
            if create_group(name, description):
                st.success("✅ Grupo creado exitosamente!")
                # El form con clear_on_submit=True ya maneja el rerun automáticamente
                # No necesitamos st.rerun() ni cambiar session_state manualmente
                # El usuario puede recargar manualmente

        if cancel:
            st.session_state.show_create_group = False
            # El form maneja el rerun automáticamente

def render_invite_form(group):
    """Formulario para invitar miembros"""
    st.markdown(f"### 📧 Invitar a {group['name']}")

    # Mostrar mensajes de estado de invitación anteriores
    if 'invite_status' in st.session_state and st.session_state.invite_status:
        if st.session_state.invite_status == "success":
            st.success("✅ Invitación enviada exitosamente!")
        elif st.session_state.invite_status.startswith("error:"):
            error_msg = st.session_state.invite_status[6:]  # Remover "error: "
            st.error(f"❌ Error al enviar invitación: {error_msg}")
        # Limpiar el mensaje después de mostrarlo
        st.session_state.invite_status = None

    with st.form(f"invite_form_{group['id']}", clear_on_submit=True):
        email = st.text_input("Email del usuario*", placeholder="usuario@email.com",
                            help="Ingresa el email del usuario que quieres invitar")

        # Información de debug
        with st.expander("🔍 Información de Debug", expanded=False):
            st.write(f"**Grupo ID:** {group['id']}")
            st.write(f"**Grupo Nombre:** {group['name']}")
            st.write(f"**Usuario actual:** {st.session_state.get('user_id', 'No autenticado')}")

            # Información adicional de debug (sin botones dentro del form)
            if st.checkbox("Mostrar logs detallados", key=f"debug_logs_{group['id']}"):
                st.code(f"""
Grupo: {group['name']} ({group['id']})
Usuario: {st.session_state.get('user_id', 'No autenticado')}
Email a invitar: {email if 'email' in locals() else 'No especificado'}
                """)

        col1, col2 = st.columns(2)
        with col1:
            submitted = st.form_submit_button("📨 Enviar Invitación", use_container_width=True, type="primary")
        with col2:
            cancel = st.form_submit_button("❌ Cancelar", use_container_width=True)

        if submitted and email:
            with st.spinner("Enviando invitación..."):
                success = invite_member(group['id'], email)
                if success:
                    st.success("✅ Invitación enviada exitosamente!")
                    # El form con clear_on_submit=True maneja el rerun automáticamente
                else:
                    # El mensaje de error se mostrará arriba en el próximo render
                    pass

        if cancel:
            st.session_state.show_invite_form = None
            # El form maneja el rerun automáticamente

def render_group_detail():
    """Vista detallada de un grupo"""
    if not st.session_state.selected_group:
        return

    group = st.session_state.selected_group

    # Header con acciones principales
    col1, col2, col3 = st.columns([3, 1, 1])

    with col1:
        st.title(f"📋 {group['name']}")
        st.markdown(f"*{group.get('description', 'Sin descripción')}*")

    with col2:
        if st.button("⬅️ Volver", use_container_width=True):
            st.session_state.selected_group = None
            st.session_state.active_tab = 'Mis Grupos'
            # No hacer st.rerun() para evitar bucles infinitos

    with col3:
        # Menú de acciones
        with st.popover("⚙️ Acciones"):
            if st.button("✏️ Editar Grupo", use_container_width=True):
                st.session_state.editing_group = True
            if st.button("🗑️ Eliminar Grupo", use_container_width=True):
                if st.button("Confirmar eliminación", type="primary"):
                    if delete_group(group['id']):
                        # No hacer st.rerun() para evitar bucles infinitos
                        pass

    # Información del grupo
    col1, col2, col3 = st.columns(3)

    with col1:
        st.metric("👥 Miembros", len(group.get('members', [])))

    with col2:
        st.metric("📅 Eventos", len(group.get('events', [])))

    with col3:
        created_date = group.get('created_at', '')[:10] if group.get('created_at') else 'N/A'
        st.metric("📆 Creado", created_date)

    # Pestañas para el grupo
    tab1, tab2, tab3 = st.tabs(["👥 Miembros", "📅 Eventos", "⚙️ Configuración"])

    with tab1:
        render_members_management(group)

    with tab2:
        render_events_management(group)

    with tab3:
        render_group_settings(group)

def render_members_management(group):
    """Gestión de miembros del grupo"""
    st.markdown("### 👥 Gestión de Miembros")

    # Lista de miembros actuales
    members = group.get('members', [])

    if not members:
        st.info("Este grupo no tiene miembros aún")
    else:
        for member in members:
            with st.container():
                col1, col2, col3, col4 = st.columns([3, 1, 1, 1])

                with col1:
                    st.write(f"**{member.get('user_id', 'Usuario desconocido')}**")
                    role = member.get('role', 'member')
                    role_icon = "👑" if role == "admin" else "👤"
                    st.caption(f"{role_icon} {role.capitalize()}")

                with col2:
                    joined_date = member.get('joined_at', '')[:10] if member.get('joined_at') else ''
                    st.caption(f"Unido: {joined_date}")

                with col3:
                    # Cambiar rol (solo admins pueden hacerlo)
                    if role == "member":
                        if st.button("👑 Hacer Admin", key=f"promote_{member['id']}", use_container_width=True):
                            # Implementar cambio de rol
                            pass
                    else:
                        if st.button("👤 Quitar Admin", key=f"demote_{member['id']}", use_container_width=True):
                            # Implementar cambio de rol
                            pass

                with col4:
                    # Remover miembro (solo admins)
                    if st.button("❌", key=f"remove_{member['id']}", help="Remover miembro"):
                        if remove_member(group['id'], member['user_id']):
                            st.success("Miembro removido")
                            # No hacer st.rerun() para evitar bucles infinitos

    # Botón para invitar nuevos miembros
    if st.button("➕ Invitar Miembro", use_container_width=True, type="primary"):
        st.session_state.show_invite_form = group['id']
        # No hacer st.rerun() para evitar bucles infinitos

def render_events_management(group):
    """Gestión de eventos del grupo"""
    st.markdown("### 📅 Eventos del Grupo")

    events = group.get('events', [])

    if not events:
        st.info("Este grupo no tiene eventos programados")
        st.markdown("**💡 Tip:** Puedes agregar eventos existentes del calendario a este grupo")
    else:
        for event in events:
            with st.expander(f"📅 {event.get('title', 'Evento sin título')}", expanded=False):
                st.write(f"**Agregado por:** {event.get('added_by', 'Desconocido')}")
                st.write(f"**Fecha de agregado:** {event.get('added_at', 'Desconocida')[:10]}")

                if st.button("🗑️ Remover del Grupo",
                           key=f"remove_event_{event.get('event_id', '')}",
                           use_container_width=True):
                    # Implementar remoción de evento
                    st.info("Funcionalidad en desarrollo")

def render_group_settings(group):
    """Configuración del grupo"""
    st.markdown("### ⚙️ Configuración del Grupo")

    # Formulario de edición
    if st.session_state.get('editing_group', False):
        st.markdown("#### ✏️ Editar Grupo")

        with st.form(f"edit_group_form_{group['id']}"):
            new_name = st.text_input("Nombre", value=group.get('name', ''))
            new_description = st.text_area("Descripción", value=group.get('description', ''))

            col1, col2 = st.columns(2)
            with col1:
                save = st.form_submit_button("💾 Guardar Cambios", use_container_width=True, type="primary")
            with col2:
                cancel = st.form_submit_button("❌ Cancelar", use_container_width=True)

            if save and new_name:
                if update_group(group['id'], new_name, new_description):
                    st.session_state.editing_group = False
                    # El form maneja el rerun automáticamente

            if cancel:
                st.session_state.editing_group = False
                # El form maneja el rerun automáticamente

    else:
        # Información del grupo
        st.markdown("#### 📋 Información del Grupo")
        st.write(f"**Nombre:** {group.get('name', 'N/A')}")
        st.write(f"**Descripción:** {group.get('description', 'Sin descripción')}")
        st.write(f"**Creado por:** {group.get('created_by', 'Desconocido')}")
        st.write(f"**Fecha de creación:** {group.get('created_at', 'N/A')[:10]}")

        if st.button("✏️ Editar Grupo", use_container_width=True):
            st.session_state.editing_group = True

def render_invitations():
    """Vista de invitaciones"""
    st.markdown("### 📨 Invitaciones")

    load_group_invitations()
    invitations = st.session_state.group_invitations

    if not invitations:
        st.info("No tienes invitaciones pendientes")
    else:
        for invite in invitations:
            with st.container():
                st.markdown("---")
                col1, col2 = st.columns([3, 1])

                with col1:
                    st.markdown(f"### {invite.get('group_name', 'Grupo desconocido')}")
                    st.markdown(f"*Invitado por: {invite.get('invited_by', 'Usuario desconocido')}*")

                    if invite.get('message'):
                        st.markdown(f"*Mensaje: {invite.get('message')}*")

                with col2:
                    if st.button("✅ Aceptar", key=f"accept_{invite['id']}", use_container_width=True):
                        if respond_to_invitation(invite['id'], "accept"):
                            st.success("Invitación aceptada!")
                            # No hacer st.rerun() para evitar bucles infinitos

                    if st.button("❌ Rechazar", key=f"reject_{invite['id']}", use_container_width=True):
                        if respond_to_invitation(invite['id'], "reject"):
                            st.success("Invitación rechazada")
                            # No hacer st.rerun() para evitar bucles infinitos