# User Service

Servicio de usuarios para el sistema de Agenda Distribuida. Este servicio maneja la lógica de negocio relacionada con los usuarios, comunicándose con el `db_service` a través de HTTP y recibiendo eventos a través de Redis.

## 🚀 Características

- Procesamiento de eventos de usuario a través de Redis
- Comunicación con el `db_service` mediante HTTP
- Manejo de operaciones CRUD para usuarios
- Sistema de logging estructurado
- Configuración mediante variables de entorno

## 🏗️ Arquitectura

El servicio sigue una arquitectura basada en eventos donde:

1. Recibe eventos a través de Redis
2. Procesa la lógica de negocio
3. Se comunica con el `db_service` para operaciones de persistencia
4. Publica respuestas a través de Redis cuando es necesario

## 📦 Requisitos

- Go 1.21 o superior
- Redis
- Servicio `db_service` en ejecución

## ⚙️ Configuración

Copia el archivo `.env.example` a `.env` y configura las variables según tu entorno:

```bash
cp .env.example .env
```

### Variables de entorno

| Variable | Descripción | Valor por defecto |
|----------|-------------|-------------------|
| `REDIS_URL` | URL de conexión a Redis | `redis://localhost:6379` |
| `REDIS_CHANNEL` | Canal de Redis para escuchar eventos | `users_events` |
| `DB_SERVICE_URL` | URL del servicio de base de datos | `http://db-service:8080` |
| `LOG_LEVEL` | Nivel de logging | `info` |
| `SERVICE_NAME` | Nombre del servicio | `user-service` |

## 🚀 Ejecución

### Localmente

```bash
go run cmd/user-service/main.go
```

### Con Docker

```bash
docker build -t user-service .
docker run --rm -p 8081:8081 --env-file .env user-service
```

## 📡 Eventos Soportados

### Crear Usuario

**Tipo de evento:** `user.create`

**Datos requeridos:**
```json
{
  "email": "usuario@ejemplo.com",
  "password": "contraseña-segura",
  "username": "usuario"
}
```

**Respuesta exitosa:**
```json
{
  "event_id": "event-123",
  "type": "user.create",
  "success": true,
  "data": {
    "id": 1,
    "email": "usuario@ejemplo.com",
    "username": "usuario",
    "is_active": true
  }
}
```

### Eliminar Usuario

**Tipo de evento:** `user.delete`

**Datos requeridos:**
```json
{
  "email": "usuario@ejemplo.com"
}
```

**Respuesta exitosa:**
```json
{
  "event_id": "event-456",
  "type": "user.delete",
  "success": true,
  "data": {
    "message": "Usuario eliminado correctamente",
    "email": "usuario@ejemplo.com"
  }
}
```

## 📝 Notas

- El servicio está diseñado para ser altamente disponible y puede ser escalado horizontalmente.
- Los logs están estructurados en formato JSON para facilitar su procesamiento.
- Se recomienda utilizar un balanceador de carga si se despliegan múltiples instancias.
