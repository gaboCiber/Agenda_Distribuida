# Modelos de datos para el API Gateway
from pydantic import BaseModel
from typing import Optional, List
from datetime import datetime

# Modelos de eventos individuales
class EventCreateRequest(BaseModel):
    title: str
    description: str
    start_time: datetime
    end_time: datetime
    user_id: str

class EventResponse(BaseModel):
    id: str
    title: str
    description: str
    start_time: datetime
    end_time: datetime
    user_id: str
    created_at: datetime

class EventListResponse(BaseModel):
    events: List[EventResponse]
    total: int

class EventDeleteRequest(BaseModel):
    event_id: str
    user_id: str

# Modelos de eventos de grupo
class GroupEventCreateRequest(BaseModel):
    group_id: str
    title: str
    description: str
    start_time: datetime
    end_time: datetime
    user_id: str

class GroupEventResponse(BaseModel):
    event_id: str
    group_id: str
    title: str
    description: str
    start_time: datetime
    end_time: datetime
    created_by: str
    created_at: datetime

# Modelos de usuario
class UserProfileResponse(BaseModel):
    id: str
    email: str
    username: str
    is_active: bool

# Modelos de salud
class HealthResponse(BaseModel):
    status: str
    service: str
    timestamp: datetime
    version: str
    dependencies: dict