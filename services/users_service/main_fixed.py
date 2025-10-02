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

# Configuración de Redis
REDIS_HOST = os.getenv('REDIS_HOST', 'agenda-bus-redis')
REDIS_PORT = int(os.getenv('REDIS_PORT', '6379'))
REDIS_DB = int(os.getenv('REDIS_DB', '0'))

# Variable para almacenar la tarea de fondo
background_task = None

# Instancia global del servicio de Redis (inicialización diferida)
redis_service = None
_redis_initialized = False

def get_redis_service():
    """Obtener instancia de Redis con inicialización diferida"""
    global redis_service, _redis_initialized

    if not _redis_initialized:
        try:
            redis_service = RedisService(host=REDIS_HOST, port=REDIS_PORT, db=REDIS_DB)
            logger.info("✅ Servicio Redis inicializado correctamente")
            _redis_initialized = True
        except Exception as e:
            logger.error(f"❌ No se pudo inicializar el servicio Redis: {str(e)}")
            redis_service = None

    return redis_service

async def setup_redis_subscription():
    """Configurar suscripción a Redis en segundo plano"""
    global background_task

    try:
        redis_svc = get_redis_service()
        if redis_svc and redis_svc.is_connected():
            logger.info("🔄 Configurando suscripción a eventos de Redis...")
            background_task = asyncio.create_task(
                redis_svc.subscribe_to_channel("users_events")
            )
            logger.info("✅ Suscripción a canal 'users_events' configurada correctamente")
        else:
            logger.warning("⚠️ Redis no disponible - no se pudo configurar suscripción")
    except Exception as e:
        logger.error(f"Error configurando suscripción Redis: {e}")

# Eventos de inicio y parada removidos para evitar conflictos

@app.post("/setup-redis")
async def setup_redis_endpoint():
    """Endpoint para configurar manualmente la suscripción a Redis"""
    try:
        await setup_redis_subscription()
        return {"status": "success", "message": "Suscripción a Redis configurada correctamente"}
    except Exception as e:
        logger.error(f"Error configurando Redis: {e}")
        return {"status": "error", "message": str(e)}, 500

@app.get("/health")
async def health_check():
    """Endpoint de verificación de salud"""
    try:
        # Inicializar Redis si no está inicializado
        redis_svc = get_redis_service()

        # Verificar conexión a Redis sin bloquear
        redis_connected = False
        if redis_svc and hasattr(redis_svc, 'client') and redis_svc.client:
            try:
                redis_connected = redis_svc.client.ping()
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
