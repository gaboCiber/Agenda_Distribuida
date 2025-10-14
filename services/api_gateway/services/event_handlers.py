# Manejadores de eventos para el API Gateway
import json
from datetime import datetime
from services.event_service import event_service

# Almacenamiento temporal de respuestas de eventos
event_responses = {}

def process_event_response(message):
    """Procesar respuestas del Events Service - ACTUALIZADA"""
    try:
        event_data = json.loads(message['data'])
        event_type = event_data.get('type')
        correlation_id = event_data.get('payload', {}).get('correlation_id')
        response_data = event_data.get('payload', {}).get('response', {})

        if correlation_id and response_data:
            event_responses[correlation_id] = {
                'success': response_data.get('success', False),
                'message': response_data.get('message', ''),
                'data': response_data,
                'timestamp': datetime.now(),
                'type': event_type  # 🔥 NUEVO: Guardar tipo de evento
            }
            print(f"✅ Respuesta procesada para {event_type} {correlation_id}: {response_data}")
    except Exception as e:
        print(f"❌ Error procesando respuesta de evento: {e}")

def get_event_response(event_id: str):
    """Obtener respuesta de evento por ID"""
    if event_id in event_responses:
        response = event_responses[event_id]
        return {
            "status": "completed",
            "success": response['success'],
            "message": response['message'],
            "data": response['data'],
            "timestamp": response['timestamp']
        }
    else:
        return {
            "status": "processing",
            "message": "Event still being processed"
        }