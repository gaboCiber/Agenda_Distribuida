import logging
from typing import List, Optional, Dict, Any
from sqlalchemy.orm import Session
from sqlalchemy.exc import SQLAlchemyError

from models import User
from security import get_password_hash, verify_password

# Configuración de logging
logger = logging.getLogger(__name__)

class CRUDUser:
    """Clase que maneja las operaciones CRUD para el modelo User."""
    
    @staticmethod
    def get_user(db: Session, user_id: int) -> Optional[User]:
        """Obtiene un usuario por su ID.
        
        Args:
            db: Sesión de base de datos
            user_id: ID del usuario a buscar
            
        Returns:
            User: El usuario encontrado o None
        """
        try:
            return db.query(User).filter(User.id == user_id).first()
        except SQLAlchemyError as e:
            logger.error(f"Error al obtener usuario por ID {user_id}: {e}")
            return None
    
    @staticmethod
    def get_user_by_email(db: Session, email: str) -> Optional[User]:
        """Obtiene un usuario por su email.
        
        Args:
            db: Sesión de base de datos
            email: Email del usuario a buscar
            
        Returns:
            User: El usuario encontrado o None
        """
        try:
            return db.query(User).filter(User.email == email).first()
        except SQLAlchemyError as e:
            logger.error(f"Error al obtener usuario por email {email}: {e}")
            return None
    
    @staticmethod
    def get_users(db: Session, skip: int = 0, limit: int = 100) -> List[User]:
        """Obtiene una lista de usuarios con paginación.
        
        Args:
            db: Sesión de base de datos
            skip: Número de registros a omitir
            limit: Número máximo de registros a devolver
            
        Returns:
            List[User]: Lista de usuarios
        """
        try:
            return db.query(User).offset(skip).limit(limit).all()
        except SQLAlchemyError as e:
            logger.error(f"Error al obtener lista de usuarios: {e}")
            return []
    
    @staticmethod
    def create_user(db: Session, user_data: Dict[str, Any]) -> Optional[User]:
        """Crea un nuevo usuario.
        
        Args:
            db: Sesión de base de datos
            user_data: Diccionario con los datos del usuario
                Debe contener: email, password, username (opcional)
                
        Returns:
            User: El usuario creado o None en caso de error
        """
        try:
            hashed_password = get_password_hash(user_data["password"])
            username = user_data.get("username", user_data["email"].split("@")[0])
            
            db_user = User(
                email=user_data["email"],
                username=username,
                hashed_password=hashed_password,
                is_active=True
            )
            
            db.add(db_user)
            db.commit()
            db.refresh(db_user)
            return db_user
            
        except SQLAlchemyError as e:
            db.rollback()
            logger.error(f"Error al crear usuario {user_data.get('email')}: {e}")
            return None
    
    @staticmethod
    def authenticate_user(db: Session, email: str, password: str) -> Optional[User]:
        """Autentica un usuario por email y contraseña.
        
        Args:
            db: Sesión de base de datos
            email: Email del usuario
            password: Contraseña en texto plano
            
        Returns:
            User: El usuario autenticado o None si falla
        """
        user = CRUDUser.get_user_by_email(db, email)
        if not user:
            logger.warning(f"Intento de inicio de sesión fallido: usuario {email} no encontrado")
            return None
            
        if not verify_password(password, user.hashed_password):
            logger.warning(f"Intento de inicio de sesión fallido: contraseña incorrecta para {email}")
            return None
            
        if not user.is_active:
            logger.warning(f"Intento de inicio de sesión fallido: usuario {email} inactivo")
            return None
            
        return user
    
    @staticmethod
    def delete_user(db: Session, user_id: int) -> bool:
        """Elimina un usuario por su ID.
        
        Args:
            db: Sesión de base de datos
            user_id: ID del usuario a eliminar
            
        Returns:
            bool: True si se eliminó correctamente, False en caso contrario
        """
        try:
            user = CRUDUser.get_user(db, user_id)
            if not user:
                logger.warning(f"Intento de eliminar usuario ID {user_id} fallido: no encontrado")
                return False
                
            db.delete(user)
            db.commit()
            logger.info(f"Usuario ID {user_id} eliminado correctamente")
            return True
            
        except SQLAlchemyError as e:
            db.rollback()
            logger.error(f"Error al eliminar usuario ID {user_id}: {e}")
            return False

# Instancia global para facilitar el uso
user_crud = CRUDUser()
