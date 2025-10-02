import asyncio
import logging
import os
import sys
from datetime import datetime
from fastapi import FastAPI, HTTPException
from sqlalchemy.orm import Session

# Cargar variables de entorno primero
from dotenv import load_dotenv
load_dotenv()

# Configuración inicial de logging para capturar errores de arranque
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
    handlers=[
        logging.StreamHandler(sys.stdout)
    ]
)
logger = logging.getLogger(__name__)

try:
    # Importar dependencias después de configurar el logging
    from database import engine, SessionLocal
    import models
    from services.event_service import RedisService
    from services.user_event_handler import user_event_handler
    
    # Crear tablas en la base de datos
    models.Base.metadata.create_all(bind=engine)
    logger.info("✅ Base de datos inicializada correctamente")
    
except Exception as e:
    logger.critical(f"❌ Error crítico al inicializar la base de datos: {str(e)}", exc_info=True)
    sys.exit(1)

# Inicializar FastAPI
app = FastAPI(title="Users Service (Event-Driven)")

# Variable para almacenar la tarea de fondo
background_task = None

# Get Redis configuration from environment variables
REDIS_HOST = os.getenv('REDIS_HOST', 'agenda-bus-redis')
REDIS_PORT = int(os.getenv('REDIS_PORT', '6379'))
REDIS_DB = int(os.getenv('REDIS_DB', '0'))

# Log Redis connection details
logger.info(f"🔌 Intentando conectar a Redis en {REDIS_HOST}:{REDIS_PORT} (DB: {REDIS_DB})")

# Instancia global del servicio de Redis
try:
    redis_service = RedisService(host=REDIS_HOST, port=REDIS_PORT, db=REDIS_DB)
    logger.info("✅ Servicio Redis inicializado correctamente")
except Exception as e:
    logger.critical(f"❌ No se pudo inicializar el servicio Redis: {str(e)}", exc_info=True)
    redis_service = None


@app.on_event("startup")
async def startup_event():
    """Iniciar el servicio de suscripción a eventos al arrancar"""
    global background_task, redis_service
    
    try:
        # Configurar directorio de logs
        log_dir = os.path.join(os.getcwd(), "logs")
        os.makedirs(log_dir, exist_ok=True)
        log_file = os.path.join(log_dir, "users_service.log")
        
        # Configurar logging con un FileHandler adicional
        file_handler = logging.FileHandler(log_file)
        file_handler.setLevel(logging.INFO)
        file_handler.setFormatter(logging.Formatter('%(asctime)s - %(name)s - %(levelname)s - %(message)s'))
        
        # Añadir el manejador de archivo al logger raíz
        root_logger = logging.getLogger()
        root_logger.addHandler(file_handler)
        
        logger.info("=" * 50)
        logger.info("🚀 Iniciando servicio de usuarios (Event-Driven)")
        
        # Verificar que el servicio Redis está disponible
        if redis_service is None:
            raise RuntimeError("El servicio Redis no se pudo inicializar")
        
        # Verificar conexión a Redis
        max_retries = 3
        retry_delay = 5
        redis_connected = False
        
        for attempt in range(1, max_retries + 1):
            try:
                if redis_service.is_connected():
                    logger.info("✅ Conexión a Redis verificada")
                    redis_connected = True
                    break
                    
                logger.warning(f"Intento {attempt}/{max_retries} - No se pudo conectar a Redis")
                if attempt < max_retries:
                    logger.info(f"Reintentando en {retry_delay} segundos...")
                    await asyncio.sleep(retry_delay)
            except Exception as e:
                logger.error(f"Error verificando conexión a Redis: {e}")
                if attempt >= max_retries:
                    raise
        
        if not redis_connected:
            raise RuntimeError("❌ No se pudo conectar a Redis después de varios intentos")
        
        # Iniciar la suscripción a Redis en segundo plano
        try:
            background_task = asyncio.create_task(
                redis_service.subscribe_to_channel("users_events")
            )
            logger.info("✅ Servicio de usuarios listo para recibir eventos")
        except Exception as e:
            logger.error(f"Error al iniciar la suscripción a eventos: {e}", exc_info=True)
            raise
            
    except Exception as e:
        logger.critical(f"❌ Error crítico durante el inicio: {str(e)}", exc_info=True)
        # Forzar la salida del proceso si hay un error crítico
        import sys
        sys.exit(1)

@app.on_event("shutdown")
async def shutdown_event():
    """Manejar la parada del servicio"""
    logger.info("Deteniendo servicio de usuarios...")
    if background_task:
        background_task.cancel()
        try:
            await background_task
        except asyncio.CancelledError:
            pass
    logger.info("Servicio de usuarios detenido")

@app.get("/health")
async def health_check():
    """Endpoint de verificación de salud"""
    try:
        # Verificar conexión a Redis sin bloquear
        redis_connected = False
        if hasattr(redis_service, 'client') and redis_service.client:
            try:
                redis_connected = redis_service.client.ping()
            except:
                redis_connected = False
                
        return {
            "status": "ok",
            "service": "users_service",
            "redis_connected": redis_connected,
            "timestamp": datetime.utcnow().isoformat()
        }
    except Exception as e:
        logger.error(f"Error en health check: {str(e)}", exc_info=True)
        return {
            "status": "error",
            "service": "users_service",
            "error": str(e),
            "timestamp": datetime.utcnow().isoformat()
        }, 503

@app.get("/")
async def read_root():
    return {
        "message": "Users Service (Event-Driven) is running",
        "status": "active",
        "events": ["user_registration_requested", "user_login_requested"]
    }

# Ejecutar la aplicación con uvicorn cuando se ejecute este archivo directamente
if __name__ == "__main__":
    import uvicorn
    uvicorn.run("main:app", host="0.0.0.0", port=8001, reload=True)
