
import os
from sqlalchemy import create_engine
from sqlalchemy.ext.declarative import declarative_base
from sqlalchemy.orm import sessionmaker, scoped_session
from contextlib import contextmanager
import logging

# Configuración de logging
logging.basicConfig()
logger = logging.getLogger(__name__)
logger.setLevel(logging.INFO)

# Usar SQLite para la base de datos local
# Obtener la ruta de la base de datos del entorno o usar la predeterminada
db_path = os.getenv("DATABASE_PATH", "/app/data/db/agenda_users.db")

# Asegurarse de que el directorio existe
os.makedirs(os.path.dirname(db_path), exist_ok=True)

# Configurar la URL de la base de datos
SQLALCHEMY_DATABASE_URL = f"sqlite:///{db_path}"

# Configurar el motor de la base de datos
engine = create_engine(
    SQLALCHEMY_DATABASE_URL, 
    connect_args={"check_same_thread": False},
    pool_pre_ping=True,  # Verifica la conexión antes de usarla
    pool_recycle=300,    # Reciclar conexiones después de 5 minutos
)

# Configurar la sesión de SQLAlchemy
SessionLocal = sessionmaker(
    autocommit=False, 
    autoflush=False, 
    bind=engine
)

# Base para los modelos
Base = declarative_base()

# Crear una fábrica de sesiones con alcance de hilo
session_factory = sessionmaker(bind=engine)
Session = scoped_session(session_factory)

@contextmanager
def get_db():
    """Proporciona una sesión de base de datos transaccional.
    
    Uso:
        with get_db() as db:
            # Usar la sesión db aquí
            pass  # La sesión se cierra automáticamente al salir del bloque
    """
    db = Session()
    try:
        yield db
        db.commit()
    except Exception as e:
        db.rollback()
        logger.error(f"Error en la sesión de la base de datos: {e}")
        raise
    finally:
        db.close()

def init_db():
    """Inicializa la base de datos creando todas las tablas."""
    import models  # Importar modelos para que se registren con SQLAlchemy
    Base.metadata.create_all(bind=engine)
    logger.info("Base de datos inicializada correctamente")
