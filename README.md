# Agenda Distribuida

Sistema de agenda distribuida desarrollado como parte del curso de Sistemas Distribuidos. Este proyecto implementa una arquitectura de microservicios para gestionar usuarios, grupos y eventos de manera distribuida.

## 🚀 Características

- Gestión de usuarios y autenticación
- Creación y gestión de eventos personales y grupales
- Manejo de grupos de usuarios con estructura jerárquica
- Sistema de notificaciones en tiempo real
- Comunicación entre servicios mediante Redis
- Almacenamiento persistente de datos

## 🏗️ Arquitectura

El sistema está compuesto por los siguientes microservicios:

1. **User Service** (Go)
   - Gestión de usuarios y autenticación
   - Manejo de perfiles y credenciales
   - Control de acceso y autorización

2. **Group Service** (Go)
   - Administración de grupos de usuarios
   - Control de miembros y permisos
   - Jerarquía de grupos

3. **DB Service** (Go)
   - Servicio centralizado de base de datos
   - Gestión de transacciones
   - Almacenamiento persistente

4. **Redis**
   - Comunicación entre servicios
   - Sistema de mensajería asíncrona
   - Caché distribuido

## 📦 Requisitos Previos

- Docker 20.10+
- Git
- Go 1.19+ (para desarrollo)

## 🛠️ Instalación

1. Clonar el repositorio:
   ```bash
   git clone https://github.com/gaboCiber/Agenda_Distribuida.git
   cd Agenda_Distribuida
   ```

2. Construir las imágenes de Docker:
   ```bash
   ./scripts/build-images.sh
   ```

3. Iniciar los servicios:
   ```bash
   ./scripts/start.sh
   ```

## 🚀 Uso

### Puertos de los Servicios

- **User Service**: http://localhost:8001
- **Group Service**: http://localhost:8003
- **DB Service**: http://localhost:8005
- **Redis**: redis://localhost:6379

### Comandos Útiles

- Ver logs de los servicios:
  ```bash
  docker-compose logs -f
  ```

- Detener todos los servicios:
  ```bash
  docker-compose down
  ```

## 🛠️ Desarrollo

La estructura del proyecto es la siguiente:

```
Agenda_Distribuida/
├── services/
│   ├── user_service/    # Servicio de usuarios
│   ├── group_service/   # Servicio de grupos
│   └── db_service/      # Servicio de base de datos
├── scripts/             # Scripts de utilidad
└── README.md            # Este archivo
```

## 📄 Licencia

Este proyecto está bajo la licencia MIT. Ver el archivo [LICENSE](LICENSE) para más detalles.
