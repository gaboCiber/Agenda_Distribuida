import logging
import json
import uuid
from datetime import datetime, timedelta
from typing import Dict, Any, Optional
from contextlib import contextmanager

from sqlalchemy.orm import Session

from database import get_db, SessionLocal
from security import (
    get_password_hash, 
    verify_password, 
    create_access_token, 
    ACCESS_TOKEN_EXPIRE_MINUTES,
    get_email_from_token
)
from services.event_service import redis_service
from crud import user_crud
from models import User

logger = logging.getLogger(__name__)

class UserEventHandler:
    """Manejador de eventos para operaciones de usuario.
    
    Esta clase se encarga de manejar los eventos relacionados con usuarios,
    como registro, inicio de sesión, actualización de perfil, etc.
    """
    
    def __init__(self):
        """Inicializa el manejador de eventos y registra los manejadores."""
        self.setup_event_handlers()
        logger.info("UserEventHandler inicializado")
    
    def setup_event_handlers(self):
        """Configura los manejadores de eventos para las operaciones de usuario."""
        # Registrar manejadores para eventos de usuario
        redis_service.register_handler("user_register_requested", self.handle_registration)
        redis_service.register_handler("user_login_requested", self.handle_login)
        redis_service.register_handler("user_delete_requested", self.handle_delete)
        
        logger.info("Manejadores de eventos de usuario configurados")
    
    async def handle_registration(self, event: Dict[str, Any]):
        """Maneja el evento de registro de usuario.
        
        Args:
            event: Diccionario con los datos del evento
        """
        logger.info(f"Manejando evento de registro: {event.get('event_id')}")
        
        with self._get_db_session() as db:
            try:
                payload = event.get('payload', {})
                email = payload.get('email', '').strip().lower()
                password = payload.get('password', '').strip()
                username = payload.get('username', '').strip() or email.split('@')[0]
                
                # Validar datos de entrada
                if not email or not password:
                    return await self._publish_registration_response(
                        event=event,
                        success=False,
                        error="Email y contraseña son requeridos"
                    )
                
                # Verificar si el usuario ya existe
                existing_user = user_crud.get_user_by_email(db, email=email)
                if existing_user:
                    return await self._publish_registration_response(
                        event=event,
                        success=False,
                        error="El correo electrónico ya está registrado"
                    )
                
                # Crear nuevo usuario
                user_data = {
                    "email": email,
                    "password": password,
                    "username": username
                }
                
                new_user = user_crud.create_user(db, user_data=user_data)
                if not new_user:
                    return await self._publish_registration_response(
                        event=event,
                        success=False,
                        error="No se pudo crear el usuario"
                    )
                
                # Publicar respuesta de éxito
                await self._publish_registration_response(
                    event=event,
                    success=True,
                    user_id=str(new_user.id),
                    email=new_user.email,
                    username=new_user.username,
                    is_active=new_user.is_active
                )
                
                logger.info(f"Usuario registrado exitosamente: {email}")
                
            except Exception as e:
                logger.error(f"Error en el registro de usuario: {e}", exc_info=True)
                await self._publish_registration_response(
                    event=event,
                    success=False,
                    error="Error interno del servidor"
                )
    
    async def handle_login(self, event: Dict[str, Any]):
        """Maneja el evento de inicio de sesión de usuario.
        
        Args:
            event: Diccionario con los datos del evento
        """
        logger.info(f"Manejando evento de inicio de sesión: {event.get('event_id')}")
        
        with self._get_db_session() as db:
            try:
                payload = event.get('payload', {})
                email = payload.get('email', '').strip().lower()
                password = payload.get('password', '').strip()
                
                # Validar credenciales
                if not email or not password:
                    return await self._publish_login_response(
                        event=event,
                        success=False,
                        error="Email y contraseña son requeridos"
                    )
                
                # Autenticar usuario
                user = user_crud.authenticate_user(db, email=email, password=password)
                if not user:
                    return await self._publish_login_response(
                        event=event,
                        success=False,
                        error="Credenciales inválidas"
                    )
                
                # Generar token de acceso
                access_token = create_access_token(
                    data={"sub": user.email},
                    expires_delta=timedelta(minutes=ACCESS_TOKEN_EXPIRE_MINUTES)
                )
                
                # Publicar respuesta de éxito
                await self._publish_login_response(
                    event=event,
                    success=True,
                    access_token=access_token,
                    token_type="bearer",
                    user_id=str(user.id),
                    email=user.email,
                    username=user.username,
                    is_active=user.is_active
                )
                
                logger.info(f"Inicio de sesión exitoso: {email}")
                
            except Exception as e:
                logger.error(f"Error en el inicio de sesión: {e}", exc_info=True)
                await self._publish_login_response(
                    event=event,
                    success=False,
                    error="Error interno del servidor"
                )
    
    async def handle_delete(self, event: Dict[str, Any]):
        """Maneja el evento de eliminación de usuario.
        
        Args:
            event: Diccionario con los datos del evento
        """
        logger.info(f"Manejando evento de eliminación de usuario: {event.get('event_id')}")
        
        with self._get_db_session() as db:
            try:
                payload = event.get('payload', {})
                token = payload.get('token')
                
                if not token:
                    return await self._publish_delete_response(
                        event=event,
                        success=False,
                        error="Token de autenticación requerido"
                    )
                
                # Obtener email del token
                email = get_email_from_token(token)
                if not email:
                    return await self._publish_delete_response(
                        event=event,
                        success=False,
                        error="Token inválido o expirado"
                    )
                
                # Obtener usuario por email
                user = user_crud.get_user_by_email(db, email=email)
                if not user:
                    return await self._publish_delete_response(
                        event=event,
                        success=False,
                        error="Usuario no encontrado"
                    )
                
                # Eliminar usuario
                success = user_crud.delete_user(db, user_id=user.id)
                if not success:
                    return await self._publish_delete_response(
                        event=event,
                        success=False,
                        error="No se pudo eliminar el usuario"
                    )
                
                # Publicar respuesta de éxito
                await self._publish_delete_response(
                    event=event,
                    success=True,
                    message="Usuario eliminado correctamente"
                )
                
                logger.info(f"Usuario eliminado exitosamente: {email}")
                
            except Exception as e:
                logger.error(f"Error al eliminar usuario: {e}", exc_info=True)
                await self._publish_delete_response(
                    event=event,
                    success=False,
                    error="Error interno del servidor"
                )
    
    @contextmanager
    def _get_db_session(self):
        """Proporciona una sesión de base de datos con manejo de contexto."""
        db = SessionLocal()
        try:
            yield db
        except Exception as e:
            db.rollback()
            logger.error(f"Error en la sesión de base de datos: {e}", exc_info=True)
            raise
        finally:
            db.close()
    
    async def _publish_registration_response(self, event: Dict[str, Any], **response_data):
        """Publica la respuesta de registro de usuario.
        
        Args:
            event: Evento original que generó la respuesta
            **response_data: Datos adicionales para la respuesta
        """
        await self._publish_response(
            original_event=event,
            response_type="user_register_response",
            **response_data
        )
    
    async def _publish_login_response(self, event: Dict[str, Any], **response_data):
        """Publica la respuesta de inicio de sesión.
        
        Args:
            event: Evento original que generó la respuesta
            **response_data: Datos adicionales para la respuesta
        """
        await self._publish_response(
            original_event=event,
            response_type="user_login_response",
            **response_data
        )
    
    async def _publish_delete_response(self, event: Dict[str, Any], **response_data):
        """Publica la respuesta de eliminación de usuario.
        
        Args:
            event: Evento original que generó la respuesta
            **response_data: Datos adicionales para la respuesta
        """
        await self._publish_response(
            original_event=event,
            response_type="user_delete_response",
            **response_data
        )
    
    async def _publish_response(
        self, 
        original_event: Dict[str, Any], 
        response_type: str, 
        **response_data
    ):
        """Publica una respuesta genérica a un evento.
        
        Args:
            original_event: Evento original que generó la respuesta
            response_type: Tipo de respuesta
            **response_data: Datos adicionales para la respuesta
        """
        try:
            response_event = {
                "event_id": str(uuid.uuid4()),
                "type": response_type,
                "timestamp": datetime.utcnow().isoformat(),
                "correlation_id": original_event.get('event_id'),
                "payload": response_data
            }
            
            # Publicar en el canal de respuestas
            await redis_service.publish_event(
                channel="users_events_response",
                event_type=response_type,
                payload=response_event
            )
            
            logger.debug(f"Respuesta publicada: {response_type} (ID: {response_event['event_id']})")
            
        except Exception as e:
            logger.error(f"Error al publicar respuesta {response_type}: {e}", exc_info=True)
            raise

# Instancia global del manejador de eventos
user_event_handler = UserEventHandler()
