from datetime import datetime
from sqlalchemy import Column, Integer, String, DateTime, Boolean
from sqlalchemy.sql import func
from database import Base

class User(Base):
    """Modelo de usuario para la base de datos.
    
    Atributos:
        id: Identificador único del usuario
        username: Nombre de usuario único
        email: Correo electrónico único
        hashed_password: Contraseña hasheada
        is_active: Indica si el usuario está activo
        created_at: Fecha de creación del usuario
        updated_at: Fecha de última actualización
    """
    __tablename__ = "users"

    id = Column(Integer, primary_key=True, index=True)
    username = Column(String(50), unique=True, index=True, nullable=False)
    email = Column(String(255), unique=True, index=True, nullable=False)
    hashed_password = Column(String(255), nullable=False)
    is_active = Column(Boolean, default=True)
    created_at = Column(DateTime, default=func.now())
    updated_at = Column(DateTime, default=func.now(), onupdate=func.now())

    def to_dict(self):
        """Convierte el objeto User a un diccionario."""
        return {
            "id": self.id,
            "username": self.username,
            "email": self.email,
            "is_active": self.is_active,
            "created_at": self.created_at.isoformat() if self.created_at else None,
            "updated_at": self.updated_at.isoformat() if self.updated_at else None
        }

    def __repr__(self):
        return f"<User(id={self.id}, email={self.email}, username={self.username})>"
