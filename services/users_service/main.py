import asyncio
import logging
import os
import sys
import json
from datetime import datetime
from fastapi import FastAPI, HTTPException, BackgroundTasks
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
    from services.event_service import redis_service
    from services.user_event_handler import user_event_handler  # ✅ Importar manejador de eventos

    # Crear tablas en la base de datos
    models.Base.metadata.create_all(bind=engine)
    logger.info("✅ Base de datos inicializada correctamente")

except Exception as e:
    logger.critical(f"❌ Error crítico al inicializar la base de datos: {str(e)}", exc_info=True)
    sys.exit(1)

# Inicializar FastAPI
app = FastAPI(title="Users Service (Final)")

# Configuración de Redis
REDIS_HOST = os.getenv('REDIS_HOST', 'agenda-bus-redis')
REDIS_PORT = int(os.getenv('REDIS_PORT', '6379'))
REDIS_DB = int(os.getenv('REDIS_DB', '0'))

# Estado global del servicio
service_state = {
    "redis_connected": False,
    "events_listener_running": False,
    "events_listener_task": None
}

# Instancia global del servicio de Redis
# redis_service = None

def get_redis_service():
    """Obtener instancia de Redis"""
    global redis_service

    if redis_service is None:
        try:
            redis_service = RedisService(host=REDIS_HOST, port=REDIS_PORT, db=REDIS_DB)
            logger.info("✅ Servicio Redis creado correctamente")
        except Exception as e:
            redis_service = None
    else:
        logger.info("✅ Servicio Redis inicializado correctamente")


    return redis_service

async def events_listener():
    """Listener de eventos que procesa eventos usando user_event_handler"""
    logger.info("Iniciando listener de eventos con user_event_handler...")

    redis_svc = get_redis_service()
    if not redis_svc:
        logger.error("No se pudo obtener servicio Redis para el listener")
        return

    try:
        # Este bucle se ejecuta completamente independiente del servidor HTTP
        while True:
            try:
                if not redis_svc.is_connected():
                    logger.warning("Redis desconectado, intentando reconectar...")
                    await asyncio.sleep(5)
                    continue

                # Usar get_message con timeout corto para no bloquear
                message = redis_svc.pubsub.get_message(timeout=1.0, ignore_subscribe_messages=True)

                if message and message.get('type') == 'message':
                    try:
                        # Procesar el evento usando el user_event_handler existente
                        await redis_svc._process_message(message)
                        logger.info("Evento procesado correctamente por user_event_handler")
                    except Exception as e:
                        logger.error(f"Error procesando evento: {e}")

                await asyncio.sleep(0.1)  # Pequeña pausa para no consumir CPU

            except Exception as e:
                logger.error(f"Error en listener de eventos: {e}")
                await asyncio.sleep(5)

    except asyncio.CancelledError:
        logger.info("🛑 Listener de eventos detenido correctamente")
    except Exception as e:
        logger.error(f"Error fatal en listener de eventos: {e}")

@app.get("/health")
async def health_check():
    """Endpoint de verificación de salud"""
    try:
        # Verificar conexión a Redis
        redis_svc = get_redis_service()
        redis_connected = False

        if redis_svc and hasattr(redis_svc, 'client') and redis_svc.client:
            try:
                redis_connected = redis_svc.client.ping()
            except:
                redis_connected = False

        # Actualizar estado global
        service_state["redis_connected"] = redis_connected

        return {
            "status": "ok",
            "service": "users_service_final",
            "redis_connected": redis_connected,
            "events_listener_running": service_state["events_listener_running"],
            "timestamp": datetime.utcnow().isoformat()
        }
    except Exception as e:
        logger.error(f"Error en health check: {str(e)}", exc_info=True)
        return {
            "status": "error",
            "service": "users_service_final",
            "error": str(e),
            "timestamp": datetime.utcnow().isoformat()
        }, 503

@app.get("/")
async def read_root():
    return {
        "message": "Users Service (Final) is running",
        "status": "active",
        "redis_connected": service_state["redis_connected"],
        "events_listener_running": service_state["events_listener_running"]
    }

@app.post("/start-events")
async def start_events_endpoint(background_tasks: BackgroundTasks):
    """Iniciar listener de eventos en segundo plano"""
    try:
        if service_state["events_listener_running"]:
            return {"status": "info", "message": "El listener de eventos ya está corriendo"}

        redis_svc = get_redis_service()
        if not redis_svc or not redis_svc.is_connected():
            return {"status": "error", "message": "Redis no disponible"}, 503

        # Iniciar listener en segundo plano completamente independiente
        task = asyncio.create_task(events_listener())
        service_state["events_listener_task"] = task
        service_state["events_listener_running"] = True

        # Suscribirse al canal de Redis
        redis_svc.pubsub.subscribe("users_events")
        logger.info("✅ Suscrito al canal users_events")

        return {"status": "success", "message": "Listener de eventos iniciado correctamente"}

    except Exception as e:
        logger.error(f"Error iniciando listener de eventos: {e}")
        return {"status": "error", "message": str(e)}, 500

@app.post("/stop-events")
async def stop_events_endpoint():
    """Detener listener de eventos"""
    try:
        if service_state["events_listener_task"]:
            service_state["events_listener_task"].cancel()
            service_state["events_listener_running"] = False
            service_state["events_listener_task"] = None

        return {"status": "success", "message": "Listener de eventos detenido correctamente"}

    except Exception as e:
        logger.error(f"Error deteniendo listener de eventos: {e}")
        return {"status": "error", "message": str(e)}, 500

@app.on_event("shutdown")
async def shutdown_event():
    """Evento de parada del servicio"""
    logger.info("🛑 Deteniendo servicio de usuarios...")

    # Detener listener de eventos si está corriendo
    if service_state["events_listener_task"]:
        service_state["events_listener_task"].cancel()
        try:
            await service_state["events_listener_task"]
        except asyncio.CancelledError:
            pass

    logger.info("✅ Servicio detenido correctamente")

# Ejecutar la aplicación con uvicorn cuando se ejecute este archivo directamente
if __name__ == "__main__":
    import uvicorn
    uvicorn.run("main:app", host="0.0.0.0", port=8001, reload=True)
