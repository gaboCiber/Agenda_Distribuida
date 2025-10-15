# Utilidades para acciones de grupos
import streamlit as st
import time
from utils.api_utils import make_api_request
from utils.data_utils import load_groups, load_group_invitations, load_group_details

def create_group(name, description):
    """Crear un nuevo grupo"""
    if not name or not st.session_state.user_id:
        return False

    data = {
        "name": name,
        "description": description or ""
    }

    try:
        response = make_api_request("/api/v1/groups", "POST", data)

        if response and response.status_code == 201:
            st.session_state.create_group_status = "success"
            # No recargar automáticamente para evitar bucles infinitos
            # load_groups() será llamado manualmente por el usuario
            return True
        else:
            error_msg = response.json().get('detail', 'Error desconocido') if response else 'Error de conexión'
            st.session_state.create_group_status = f"error: {error_msg}"
            return False
    except Exception as e:
        st.session_state.create_group_status = f"error: {str(e)}"
        return False

def update_group(group_id, name, description):
    """Actualizar un grupo"""
    data = {
        "name": name,
        "description": description or ""
    }

    try:
        response = make_api_request(f"/api/v1/groups/{group_id}", "PUT", data)

        if response and response.status_code == 200:
            st.session_state.selected_group.update(data)
            st.success("Grupo actualizado exitosamente")
            return True
        else:
            error_msg = response.json().get('detail', 'Error desconocido') if response else 'Error de conexión'
            st.error(f"Error actualizando grupo: {error_msg}")
            return False
    except Exception as e:
        st.error(f"Error actualizando grupo: {str(e)}")
        return False

def delete_group(group_id):
    """Eliminar un grupo"""
    try:
        response = make_api_request(f"/api/v1/groups/{group_id}", "DELETE")

        if response and response.status_code == 204:
            st.success("Grupo eliminado exitosamente")
            st.session_state.selected_group = None
            load_groups()  # Recargar lista
            return True
        else:
            error_msg = response.json().get('detail', 'Error desconocido') if response else 'Error de conexión'
            st.error(f"Error eliminando grupo: {error_msg}")
            return False
    except Exception as e:
        st.error(f"Error eliminando grupo: {str(e)}")
        return False

def invite_member(group_id, user_email):
    """Invitar a un usuario al grupo"""
    try:
        # DEBUG: Mostrar información de debug
        print(f"🔍 DEBUG: Invitando usuario {user_email} al grupo {group_id}")
        print(f"🔍 DEBUG: Usuario actual: {st.session_state.get('user_id', 'No autenticado')}")

        # Validar que el usuario esté autenticado
        if not st.session_state.get('user_id'):
            st.session_state.invite_status = "error: Usuario no autenticado"
            print("❌ DEBUG: Usuario no autenticado")
            return False

        # Validar email
        if not user_email or '@' not in user_email:
            st.session_state.invite_status = "error: Email inválido"
            print("❌ DEBUG: Email inválido")
            return False

        # Buscar usuario por email - CORRECCIÓN: Usar endpoint correcto del API Gateway
        print(f"🔍 DEBUG: Buscando usuario por email: {user_email}")
        user_response = make_api_request(f"/api/v1/users/email/{user_email}")

        print(f"🔍 DEBUG: Respuesta de búsqueda de usuario - Status: {user_response.status_code if user_response else 'None'}")

        if not user_response or user_response.status_code != 200:
            error_detail = ""
            if user_response:
                try:
                    error_data = user_response.json()
                    error_detail = f" - {error_data.get('detail', 'Sin detalle')}"
                except:
                    error_detail = f" - Status: {user_response.status_code}"
            st.session_state.invite_status = f"error: Usuario no encontrado{error_detail}"
            print(f"❌ DEBUG: Usuario no encontrado: {error_detail}")
            return False

        user_data = user_response.json()
        user_id = user_data.get('id')

        print(f"🔍 DEBUG: Usuario encontrado - ID: {user_id}, Email: {user_data.get('email', 'N/A')}")

        if not user_id:
            st.session_state.invite_status = "error: ID de usuario no encontrado en respuesta"
            print("❌ DEBUG: ID de usuario no encontrado en respuesta")
            return False

        # Verificar que no se esté invitando a uno mismo
        if user_id == st.session_state.user_id:
            st.session_state.invite_status = "error: No puedes invitarte a ti mismo"
            print("❌ DEBUG: Intento de auto-invitación")
            return False

        # Crear invitación
        invite_data = {
            "group_id": group_id,
            "user_id": user_id
        }

        print(f"🔍 DEBUG: Creando invitación con datos: {invite_data}")
        invite_response = make_api_request("/api/v1/groups/invitations", "POST", invite_data)

        print(f"🔍 DEBUG: Respuesta de creación de invitación - Status: {invite_response.status_code if invite_response else 'None'}")

        if invite_response and invite_response.status_code == 201:
            st.session_state.invite_status = "success"
            print("✅ DEBUG: Invitación creada exitosamente")
            return True
        else:
            error_msg = "Error desconocido"
            if invite_response:
                try:
                    error_data = invite_response.json()
                    error_msg = error_data.get('detail', f'HTTP {invite_response.status_code}')
                except Exception as e:
                    error_msg = f'Error parseando respuesta: {str(e)}'
            st.session_state.invite_status = f"error: {error_msg}"
            print(f"❌ DEBUG: Error creando invitación: {error_msg}")
            return False

    except Exception as e:
        error_msg = f"error: Excepción inesperada: {str(e)}"
        st.session_state.invite_status = error_msg
        print(f"❌ DEBUG: Excepción en invite_member: {str(e)}")
        import traceback
        print(f"❌ DEBUG: Traceback: {traceback.format_exc()}")
        return False

def remove_member(group_id, member_id):
    """Remover un miembro del grupo"""
    try:
        response = make_api_request(f"/api/v1/groups/{group_id}/members/{member_id}", "DELETE")

        if response and response.status_code == 204:
            st.session_state.member_action_status = "removed"
            # Recargar detalles del grupo
            if st.session_state.selected_group:
                group_details = load_group_details(group_id)
                if group_details:
                    st.session_state.selected_group = group_details
            return True
        else:
            error_msg = response.json().get('detail', 'Error desconocido') if response else 'Error de conexión'
            st.session_state.member_action_status = f"error: {error_msg}"
            return False
    except Exception as e:
        st.session_state.member_action_status = f"error: {str(e)}"
        return False

def respond_to_invitation(invitation_id, action):
    """Responder a una invitación"""
    data = {"action": action}

    try:
        response = make_api_request(f"/api/v1/groups/invitations/{invitation_id}/respond", "POST", data)

        if response and response.status_code == 200:
            # Recargar invitaciones y grupos
            load_group_invitations()
            load_groups()
            return True
        else:
            st.error("Error procesando invitación")
            return False
    except Exception as e:
        st.error(f"Error procesando invitación: {str(e)}")
        return False