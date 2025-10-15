import json
import uuid
from datetime import datetime
from typing import Dict, Any, Optional
from schemas import EventSchema
import redis.asyncio as redis
from redis.asyncio import Redis
from settings import settings
import time
import asyncio


class RedisService:
    def __init__(self):
        self.client: Optional[Redis] = None
        self.pubsub: Optional[redis.client.PubSub] = None
        asyncio.create_task(self._connect())
    
    async def _connect(self):
        """Establece conexión con Redis"""
        try:
            self.client = redis.Redis(
                host=settings.redis_host,
                port=settings.redis_port,
                decode_responses=settings.redis_decode_responses,
                socket_connect_timeout=5,
                health_check_interval=30,
                retry_on_timeout=True
            )
            await self.client.ping()  # Test connection
            print("✅ Conectado a Redis exitosamente")
            self.pubsub = self.client.pubsub()
        except redis.ConnectionError as e:
            print(f"❌ Error conectando a Redis: {e}")
            self.client = None
            self.pubsub = None
    
    async def is_connected(self) -> bool:
        """Verifica si Redis está conectado"""
        if not self.client:
            return False
        try:
            return await self.client.ping()
        except:
            return False
    
    async def publish_event(self, channel: str, message: str) -> bool:
        """Publica un mensaje en un canal de Redis"""
        if not await self.is_connected():
            return False
        
        try:
            await self.client.publish(channel, message)
            return True
        except redis.RedisError as e:
            print(f"❌ Error publicando en Redis: {e}")
            return False
    
    def subscribe_to_channel(self, channel: str, callback):
        """Suscribirse a un canal de Redis y ejecutar callback para cada mensaje"""
        if not self.is_connected():
            return False
        
        pubsub = self.client.pubsub()
        pubsub.subscribe(channel)
        
        for message in pubsub.listen():
            if message['type'] == 'message':
                try:
                    data = json.loads(message['data'])
                    callback(data)
                except json.JSONDecodeError:
                    continue
        return True

# Instancia global del servicio Redis
redis_service = RedisService()

class EventService:
    def __init__(self):
        self.redis = redis_service
    
    def create_event(self, event_type: str, payload: Dict[str, Any]) -> EventSchema:
        """Crea un evento con estructura estandarizada"""
        return EventSchema(
            event_id=str(uuid.uuid4()),
            type=event_type,
            timestamp=datetime.now(),
            version="1.0",
            payload=payload
        )
    
    def publish_event(self, channel: str, event_type: str, payload: Dict[str, Any]) -> Dict[str, Any]:
        """
        Publica un evento en Redis (síncrono)

        Returns:
            Dict con información del evento publicado o error
        """
        if not self.redis.is_connected():
            return {"error": "Redis not connected", "published": False}
        
        # Crear evento
        event = self.create_event(event_type, payload)
        
        try:
            # Publicar en Redis
            message = event.model_dump_json()
            published = self.redis.publish_event(channel, message)
            
            if published:
                return {
                    "published": True,
                    "event_id": event.event_id,
                    "channel": channel,
                    "timestamp": event.timestamp.isoformat()
                }
            else:
                return {"error": "Failed to publish event", "published": False}
                
        except Exception as e:
            print(f"❌ Error al publicar evento: {str(e)}")
            return {"error": str(e), "published": False}
            
    async def publish_event_async(self, channel: str, event_type: str, payload: Dict[str, Any]) -> Dict[str, Any]:
        """
        Publica un evento en Redis de forma asíncrona

        Returns:
            Dict con información del evento publicado o error
        """
        # Ejecutar la publicación en un hilo separado para no bloquear el bucle de eventos
        loop = asyncio.get_event_loop()
        try:
            result = await loop.run_in_executor(
                None,  # Usar el ejecutor por defecto
                lambda: self.publish_event(channel, event_type, payload)
            )
            return result
        except Exception as e:
            print(f"❌ Error en publish_event_async: {str(e)}")
            return {"error": str(e), "published": False}

    def wait_for_response(self, response_channel: str, correlation_id: str, timeout: int = 10):
        """Espera una respuesta en un canal específico con un ID de correlación"""
        if not self.redis.is_connected():
            return None
            
        response = None
        event = asyncio.Event()
        
        def callback(message):
            nonlocal response
            if message.get('correlation_id') == correlation_id:
                response = message
                event.set()
        
        # Iniciar la suscripción en un hilo separado
        import threading
        thread = threading.Thread(
            target=self.redis.subscribe_to_channel,
            args=(response_channel, callback)
        )
        thread.daemon = True
        thread.start()
        
        # Esperar la respuesta con timeout
        try:
            loop = asyncio.get_event_loop()
            loop.run_until_complete(asyncio.wait_for(event.wait(), timeout=timeout))
        except asyncio.TimeoutError:
            pass
        
        return response

event_service = EventService()