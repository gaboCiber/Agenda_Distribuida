from fastapi import FastAPI
from settings import settings
from routers import users_router, health_router

# Crear aplicación FastAPI
app = FastAPI(
    title=settings.app_title,
    version=settings.app_version
)

# Incluir routers
app.include_router(users_router.router)
app.include_router(health_router.router)

# Info adicional
@app.on_event("startup")
async def startup_event():
    print(f"🚀 {settings.app_title} v{settings.app_version} iniciando...")

@app.on_event("shutdown") 
async def shutdown_event():
    print("👋 API Gateway apagándose...")