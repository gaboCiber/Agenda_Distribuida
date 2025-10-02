from datetime import datetime, timedelta
from typing import Optional, Dict, Any
from jose import JWTError, jwt
from passlib.context import CryptContext
import os
import logging

# Configuración de logging
logger = logging.getLogger(__name__)

# Configuración
SECRET_KEY = os.getenv("SECRET_KEY", "your-secret-key-here-please-change-in-production")
ALGORITHM = "HS256"
ACCESS_TOKEN_EXPIRE_MINUTES = 30

# Configuración de bcrypt para hashing de contraseñas
pwd_context = CryptContext(schemes=["bcrypt"], deprecated="auto")

# Las constantes se han movido arriba para mejor organización

__all__ = [
    'create_access_token', 
    'verify_password', 
    'get_password_hash',
    'get_current_user',
    'oauth2_scheme',
    'ACCESS_TOKEN_EXPIRE_MINUTES'
]

def verify_password(plain_password: str, hashed_password: str) -> bool:
    """Verifica si una contraseña coincide con su hash.
    
    Args:
        plain_password: Contraseña en texto plano
        hashed_password: Hash de la contraseña almacenado
        
    Returns:
        bool: True si la contraseña coincide, False en caso contrario
    """
    try:
        return pwd_context.verify(plain_password, hashed_password)
    except Exception as e:
        logger.error(f"Error verificando contraseña: {e}")
        return False

def get_password_hash(password: str) -> str:
    """Genera un hash seguro de una contraseña.
    
    Args:
        password: Contraseña en texto plano a hashear
        
    Returns:
        str: Hash de la contraseña
        
    Raises:
        ValueError: Si la contraseña excede el límite de longitud
    """
    if len(password.encode('utf-8')) > 72:
        raise ValueError("La contraseña no puede exceder los 72 bytes")
    return pwd_context.hash(password)

def create_access_token(data: Dict[str, Any], expires_delta: Optional[timedelta] = None) -> str:
    """Crea un token JWT de acceso.
    
    Args:
        data: Datos a incluir en el token (ej: {"sub": "user@example.com"})
        expires_delta: Tiempo de expiración del token
        
    Returns:
        str: Token JWT codificado
    """
    to_encode = data.copy()
    expire = datetime.utcnow() + (expires_delta if expires_delta else timedelta(minutes=15))
    to_encode.update({"exp": expire, "iat": datetime.utcnow()})
    
    try:
        return jwt.encode(to_encode, SECRET_KEY, algorithm=ALGORITHM)
    except Exception as e:
        logger.error(f"Error generando token JWT: {e}")
        raise

def decode_token(token: str) -> Dict[str, Any]:
    """Decodifica un token JWT y devuelve su payload.
    
    Args:
        token: Token JWT a decodificar
        
    Returns:
        dict: Payload del token decodificado
        
    Raises:
        JWTError: Si el token es inválido o ha expirado
    """
    try:
        payload = jwt.decode(token, SECRET_KEY, algorithms=[ALGORITHM])
        return payload
    except JWTError as e:
        logger.warning(f"Error decodificando token: {e}")
        raise

def get_email_from_token(token: str) -> Optional[str]:
    """Obtiene el email del usuario desde un token JWT.
    
    Args:
        token: Token JWT
        
    Returns:
        str: Email del usuario o None si el token es inválido
    """
    try:
        payload = decode_token(token)
        return payload.get("sub")
    except JWTError:
        return None
