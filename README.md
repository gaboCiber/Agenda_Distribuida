# Agenda Distribuida

Sistema de agenda distribuida desarrollado como parte del curso de Sistemas Distribuidos. Este proyecto implementa una arquitectura de microservicios para gestionar eventos, grupos de usuarios y notificaciones.

## 🚀 Características

- Gestión de usuarios y autenticación
- Creación y gestión de eventos
- Manejo de grupos de usuarios
- Sistema de notificaciones en tiempo real
- API Gateway para enrutamiento de peticiones
- Comunicación entre servicios mediante Redis

## 🏗️ Arquitectura

El sistema está compuesto por los siguientes microservicios:

1. **API Gateway** (Python) - Punto de entrada único para todas las peticiones
2. **Users Service** - Manejo de usuarios y autenticación
3. **Events Service** (Go) - Gestión de eventos
4. **Groups Service** (Go) - Administración de grupos de usuarios
5. **Notifications Service** - Sistema de notificaciones
6. **Redis** - Bus de mensajes para comunicación entre servicios

## 📦 Requisitos Previos

- Docker

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

3. Iniciar los contenedores:
   ```bash
   ./scripts/start.sh
   ```

## 🚀 Uso

Una vez iniciados los servicios, puedes acceder a ellos en los siguientes puertos:

- **API Gateway**: http://localhost:8080
- **Users Service**: http://localhost:8001
- **Events Service**: http://localhost:8002
- **Groups Service**: http://localhost:8003
- **Notifications Service**: http://localhost:8004
