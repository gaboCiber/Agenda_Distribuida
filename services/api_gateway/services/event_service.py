import json
import uuid
from datetime import datetime
from typing import Dict, Any
from schemas import EventSchema
import redis
from settings import settings

class RedisService:
    def __init__(self):
        self.client = None
        self._connect()
    
    def _connect(self):
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
            self.client.ping()  # Test connection
            print("✅ Conectado a Redis exitosamente")
        except redis.ConnectionError as e:
            print(f"❌ Error conectando a Redis: {e}")
            self.client = None
    
    def is_connected(self) -> bool:
        """Verifica si Redis está conectado"""
        if not self.client:
            return False
        try:
            return self.client.ping()
        except:
            return False
    
    def publish_event(self, channel: str, message: str) -> bool:
        """Publica un mensaje en un canal de Redis"""
        if not self.is_connected():
            return False
        
        try:
            self.client.publish(channel, message)
            return True
        except redis.RedisError as e:
            print(f"❌ Error publicando en Redis: {e}")
            return False

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
        Publica un evento en Redis
        
        Returns:
            Dict con información del evento publicado o error
        """
        if not self.redis.is_connected():
            return {"error": "Redis not connected", "published": False}
        
        # Crear evento
        event = self.create_event(event_type, payload)
        
        # Convertir a JSON
        try:
            event_json = event.json()
        except Exception as e:
            return {"error": f"Event serialization failed: {e}", "published": False}
        
        # Publicar en Redis
        if self.redis.publish_event(channel, event_json):
            return {
                "published": True,
                "event_id": event.event_id,
                "channel": channel,
                "event_type": event_type,
                "timestamp": event.timestamp.isoformat()
            }
        else:
            return {"error": "Failed to publish event", "published": False}


event_service = EventService()