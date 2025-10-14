from pydantic import BaseModel, EmailStr
from typing import Optional
from datetime import datetime

class UserRegistration(BaseModel):
    email: EmailStr
    password: str
    username: Optional[str] = None

class UserLogin(BaseModel):
    email: EmailStr
    password: str

class EventSchema(BaseModel):
    event_id: str
    type: str
    timestamp: datetime
    version: str = "1.0"
    payload: dict

class UserResponse(BaseModel):
    id: str
    email: str
    username: str
    created_at: datetime
    updated_at: datetime
    is_active: bool

    class Config:
        from_attributes = True

class HealthResponse(BaseModel):
    service: str
    status: str
    timestamp: datetime
    dependencies: dict