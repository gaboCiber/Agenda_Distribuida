import json
import logging
import time
import asyncio
import uuid
from typing import Dict, Any, Callable, Optional, List
from datetime import datetime
from concurrent.futures import ThreadPoolExecutor

import redis
from redis.exceptions import (
    ConnectionError as RedisConnectionError,
    TimeoutError as RedisTimeoutError,
    RedisError
)

# Configuración de logging
import os
log_dir = os.path.dirname(os.path.abspath(__file__))
log_file = os.path.join(log_dir, "event_service.log")

# Asegurar que el directorio de logs exista
os.makedirs(log_dir, exist_ok=True)

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
    handlers=[
        logging.StreamHandler(),
        logging.FileHandler(log_file)
    ]
)
logger = logging.getLogger(__name__)

# Constantes
MAX_RECONNECT_ATTEMPTS = 5
RECONNECT_DELAY = 5  # segundos

class RedisService:
    """Servicio para manejar la comunicación con Redis en un entorno asíncrono.
    
    Este servicio permite publicar y suscribirse a canales de Redis, con manejo
    automático de reconexión y procesamiento de mensajes en segundo plano.
    """
    
    def __init__(self, host: str = 'agenda-bus-redis', port: int = 6379, db: int = 0):
        """Inicializa el servicio de Redis.
        
        Args:
            host: Dirección del servidor Redis
            port: Puerto del servidor Redis
            db: Número de base de datos a utilizar
        """
        self.host = host
        self.port = port
        self.db = db
        self.client = None
        self.pubsub = None
        self.handlers = {}
        self.executor = ThreadPoolExecutor(max_workers=10)
        self.running = False
        self.reconnect_attempts = 0
        self._connect()
    
    def _connect(self) -> bool:
        """Establece conexión con Redis con manejo de reintentos.
        
        Returns:
            bool: True si la conexión fue exitosa, False en caso contrario
        """
        for attempt in range(MAX_RECONNECT_ATTEMPTS):
            try:
                logger.info(f"Intentando conectar a Redis en {self.host}:{self.port}/{self.db}...")
                
                self.client = redis.Redis(
                    host=self.host,
                    port=self.port,
                    db=self.db,
                    decode_responses=True,
                    encoding='utf-8',
                    socket_connect_timeout=5,
                    socket_timeout=5,
                    health_check_interval=30,
                    retry_on_timeout=True,
                    retry_on_error=[RedisConnectionError],
                    socket_keepalive=True
                )
                
                # Verificar conexión
                if not self.client.ping():
                    raise RedisConnectionError("Failed to ping Redis")
                
                self.pubsub = self.client.pubsub(
                    ignore_subscribe_messages=False  # Queremos ver los mensajes de suscripción
                )
                
                self.reconnect_attempts = 0
                logger.info(f"✅ Conectado exitosamente a Redis en {self.host}:{self.port}/{self.db}")
                return True
                
            except (RedisConnectionError, RedisTimeoutError) as e:
                self.reconnect_attempts += 1
                wait_time = min(RECONNECT_DELAY * (2 ** attempt), 30)  # Backoff exponencial con máximo 30s
                logger.error(
                    f"❌ Intento {self.reconnect_attempts}/{MAX_RECONNECT_ATTEMPTS} fallido: {str(e)}. "
                    f"Reintentando en {wait_time} segundos..."
                )
                time.sleep(wait_time)
                
            except Exception as e:
                logger.error(f"Error de Redis: {e}")
                break
                
        logger.error("No se pudo establecer conexión con Redis después de varios intentos")
        return False
    
    def is_connected(self) -> bool:
        """Verifica si Redis está conectado y responde.
        
        Returns:
            bool: True si la conexión está activa, False en caso contrario
        """
        if not self.client or not self.pubsub:
            return False
            
        try:
            return self.client.ping()
        except (RedisConnectionError, RedisTimeoutError, AttributeError):
            # Intentar reconectar una vez
            try:
                return self._connect()
            except Exception:
                return False
    
    def publish_event(self, channel: str, event_type: str, payload: Dict[str, Any]) -> bool:
        """Publica un evento en un canal de Redis.
        
        Args:
            channel: Nombre del canal donde publicar el evento
            event_type: Tipo de evento (para enrutamiento)
            payload: Datos del evento a publicar
            
        Returns:
            bool: True si el evento se publicó correctamente, False en caso contrario
        """
        if not self.is_connected() and not self._connect():
            logger.error("No se puede publicar: No hay conexión con Redis")
            return False
        
        event = {
            'event_id': str(uuid.uuid4()),
            'type': event_type,
            'timestamp': datetime.utcnow().isoformat(),
            'payload': payload
        }
        
        try:
            # Serializar a JSON y codificar como bytes
            message = json.dumps(event, ensure_ascii=False)
            result = self.client.publish(channel, message.encode('utf-8'))
            
            if result > 0:
                logger.debug(f"Evento publicado en {channel}: {event_type} (ID: {event['event_id']})")
                return True
            else:
                logger.warning(f"No hay suscriptores para el canal {channel}")
                return False
                
        except (RedisError, TypeError, ValueError) as e:
            logger.error(f"Error publicando evento en {channel}: {e}")
            # Intentar reconectar para la próxima vez
            self._connect()
            return False
    
    def register_handler(self, event_type: str, handler: Callable):
        """Registra un manejador para un tipo de evento específico.
        
        Args:
            event_type: Tipo de evento a manejar (ej: 'user_registration_requested')
            handler: Función que manejará el evento. Debe aceptar un parámetro 'event'
        """
        if not callable(handler):
            raise ValueError("El manejador debe ser una función invocable")
            
        if event_type not in self.handlers:
            self.handlers[event_type] = []
            
        self.handlers[event_type].append(handler)
        logger.info(f"Manejador registrado para evento: {event_type}")
    
    async def _process_message(self, message: Dict):
        """Procesa un mensaje recibido de Redis.
        
        Args:
            message: Mensaje recibido de la suscripción a Redis
        """
        if not message or 'type' not in message:
            logger.warning("Mensaje inválido recibido de Redis")
            return
            
        # Solo procesar mensajes de tipo 'message'
        if message['type'] != 'message' or 'data' not in message:
            return
            
        try:
            # Procesar el mensaje (manejar tanto bytes como strings)
            try:
                if isinstance(message['data'], bytes):
                    # Si es bytes, decodificar
                    event = json.loads(message['data'].decode('utf-8'))
                else:
                    # Si ya es string, usar directamente
                    event = json.loads(message['data'])
            except (UnicodeDecodeError, json.JSONDecodeError) as e:
                logger.error(f"Error procesando mensaje JSON: {e}. Datos recibidos: {message['data']}")
                return

            logger.info(f"Evento recibido: {event}")  # Logging detallado

            event_type = event.get('type')
            if not event_type:
                logger.warning(f"Mensaje recibido sin tipo de evento. Evento completo: {event}")
                return

            logger.info(f"Procesando evento de tipo: {event_type}")  # Logging detallado

            print(f"🔍 DEBUG: Tipo de evento obtenido: {event_type}")  # Debug directo
            print(f"🔍 DEBUG: Evento completo: {event}")  # Debug directo

            # Obtener manejadores para este tipo de evento
            handlers = self.handlers.get(event_type, [])
            print(f"🔍 DEBUG: Handlers encontrados: {len(handlers)} para evento {event_type}")  # Debug directo

            if not handlers:
                logger.debug(f"No hay manejadores para el evento: {event_type}")
                print(f"⚠️ DEBUG: No hay manejadores para el evento: {event_type}")  # Debug directo
                return
            else:
                logger.info(f"Handlers: {handlers} ")
                print(f"✅ DEBUG: Handlers encontrados: {handlers}")  # Debug directo
                
            # Ejecutar manejadores en el ThreadPool
            loop = asyncio.get_event_loop()
            for handler in handlers:
                try:
                    logger.info(f"🚀🚀🚀 EJECUTANDO MANEJADOR: {handler} 🚀🚀🚀")
                    logger.info(f"📦 EVENTO COMPLETO: {event}")

                    # Verificar si el manejador es una función asíncrona
                    if asyncio.iscoroutinefunction(handler):
                        logger.info("✅ Manejador es función asíncrona")
                        # Si el manejador es una corutina, esperar directamente
                        result = await handler(event)
                        logger.info(f"✅ Manejador asíncrono ejecutado exitosamente: {result}")
                    else:
                        logger.info("✅ Manejador es función síncrona")
                        # Si es síncrono, ejecutar en un hilo separado
                        result = await loop.run_in_executor(
                            self.executor,
                            lambda: handler(event)
                        )
                        logger.info(f"✅ Manejador síncrono ejecutado exitosamente: {result}")
                    logger.info(f"✅✅✅ MANEJADOR EJECUTADO EXITOSAMENTE para evento {event_type}")
                except Exception as e:
                    logger.error(f"❌❌❌ ERROR EJECUTANDO MANEJADOR para {event_type}: {e}", exc_info=True)
                    logger.error(f"Handler que falló: {handler}")
                    logger.error(f"Evento que causó el error: {event}")
                    
        except Exception as e:
            logger.error(f"Error procesando mensaje: {e}", exc_info=True)
    
    async def subscribe_to_channel(self, channel: str):
        """Se suscribe a un canal de Redis y procesa mensajes de forma asíncrona.
        
        Args:
            channel: Nombre del canal al que suscribirse
            
        Esta función se ejecuta en un bucle infinito hasta que se cancele la tarea.
        """
        self.running = True
        
        while self.running:
            try:
                if not self.is_connected():
                    logger.warning("No hay conexión a Redis, intentando reconectar...")
                    if not self._connect():
                        logger.warning(f"Reconexión fallida, reintentando en {RECONNECT_DELAY} segundos...")
                        await asyncio.sleep(RECONNECT_DELAY)
                        continue
                
                # Suscribirse al canal
                logger.info(f"Suscribiéndose al canal: {channel}")
                self.pubsub.subscribe(channel)
                
                # Esperar confirmación de suscripción
                try:
                    # Leer mensaje de confirmación de suscripción
                    message = self.pubsub.get_message(timeout=5.0, ignore_subscribe_messages=False)
                    if message and message['type'] == 'subscribe':
                        logger.info(f"✅ Suscrito exitosamente al canal: {channel}")
                    else:
                        logger.warning("No se recibió confirmación de suscripción")
                        continue
                except Exception as e:
                    logger.error(f"Error esperando confirmación de suscripción: {e}")
                    continue
                
                # Procesar mensajes entrantes
                while self.running:
                    try:
                        # Usar get_message con timeout en lugar de listen() para mejor control
                        message = self.pubsub.get_message(timeout=1.0, ignore_subscribe_messages=True)
                        if message and message.get('type') == 'message':
                            logger.debug(f"Mensaje recibido en canal {message['channel']}: {message}")
                            await self._process_message(message)

                        # Verificar periódicamente la conexión
                        if not self.is_connected():
                            logger.warning("Conexión perdida, intentando reconectar...")
                            break

                    except (RedisConnectionError, RedisTimeoutError) as e:
                        logger.error(f"Error de conexión: {e}")
                        break
                    except Exception as e:
                        logger.error(f"Error procesando mensaje: {e}", exc_info=True)
                        continue
                
                # Si llegamos aquí, hubo un error o perdimos la conexión
                logger.warning("Reconectando en 2 segundos...")
                await asyncio.sleep(2)
                
            except Exception as e:
                logger.error(f"Error en el bucle de suscripción: {e}", exc_info=True)
                await asyncio.sleep(RECONNECT_DELAY)

import os
from dotenv import load_dotenv

# Load environment variables from .env file
load_dotenv()

# Get Redis configuration from environment variables
REDIS_HOST = os.getenv('REDIS_HOST', 'agenda-bus-redis')
REDIS_PORT = int(os.getenv('REDIS_PORT', '6379'))
REDIS_DB = int(os.getenv('REDIS_DB', '0'))

# Log Redis connection details
logger.info(f"Connecting to Redis at {REDIS_HOST}:{REDIS_PORT} (DB: {REDIS_DB})")

# Instancia global del servicio de Redis
redis_service = RedisService(host=REDIS_HOST, port=REDIS_PORT, db=REDIS_DB)
