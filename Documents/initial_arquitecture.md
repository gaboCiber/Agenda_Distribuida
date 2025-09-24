# 📑 Informe de Arquitectura — Agenda Distribuida (Basada en Pub/Sub)

## 1. Introducción

El proyecto busca implementar una **agenda electrónica distribuida** con soporte para eventos personales y grupales, jerarquías, conflictos de horarios y notificaciones en tiempo real.
Para garantizar una evolución fluida hacia un sistema distribuido, la solución se basará desde el inicio en una **arquitectura Publisher–Subscriber (Pub/Sub)**.

---

## 2. Objetivos de la Entrega Centralizada

* Implementar un sistema **centralizado pero ya estructurado como Pub/Sub**.
* Los servicios no se comunicarán directamente, sino mediante un **bus de mensajes**.
* Mantener las funcionalidades principales:

  * Autenticación e identificación de usuarios.
  * Gestión de citas y grupos jerárquicos/no jerárquicos.
  * Detección de conflictos de horario.
  * Actualización automática de agendas.
  * Notificaciones en tiempo real.
* Preparar la infraestructura para **Docker Swarm**, de forma que el sistema sea escalable a múltiples nodos.

---

## 3. Arquitectura General (Centralizada con Pub/Sub)

### 3.1 Vista global

```plaintext
           ┌───────────────┐
           │   Cliente UI   │
           └───────┬───────┘
                   │ REST
           ┌───────▼────────┐
           │  API Gateway    │
           └───────┬────────┘
                   │ publica eventos
                   │
        ┌──────────▼──────────┐
        │   Bus de Mensajes   │
        │ (ej: Redis, Rabbit) │
        └───┬─────────┬──────┘
            │         │
 ┌──────────▼─┐ ┌─────▼────────┐
 │ Usuarios    │ │ Agenda       │
 │ Service     │ │ Service      │
 └─────────────┘ └─────┬────────┘
                       │
              ┌────────▼─────────┐
              │ Notificaciones   │
              │ Service          │
              └──────────────────┘
```

---

### 3.2 Componentes

#### 🔹 API Gateway (FastAPI, Python)

* Entrada única para clientes (web/móvil/CLI).
* Autenticación JWT.
* Convierte acciones de usuario en **eventos publicados** al bus.
* Ofrece endpoints REST y posiblemente WebSockets para consultas directas.

#### 🔹 Bus de Mensajes (Redis o RabbitMQ)

* Canal central para la comunicación entre servicios.
* Gestiona los eventos:

  * `usuario_creado`
  * `evento_creado`
  * `evento_modificado`
  * `grupo_actualizado`
* Provee **desacoplamiento**: servicios no se llaman entre sí.

#### 🔹 Servicio de Usuarios (Python)

* Maneja registro, login y jerarquías.
* Publica eventos `usuario_creado`, `usuario_actualizado`.
* Se suscribe a eventos relacionados con autenticación o permisos.

#### 🔹 Servicio de Agenda (Go o Python)

* CRUD de eventos, detección de conflictos.
* Publica eventos `evento_creado`, `evento_conflicto`.
* Se suscribe a eventos de grupo y usuarios.

#### 🔹 Servicio de Grupos (Go o Python)

* Gestiona grupos jerárquicos o horizontales.
* Publica eventos `grupo_creado`, `grupo_modificado`.
* Se suscribe a `evento_creado` para decidir cómo propagarlo.

#### 🔹 Servicio de Notificaciones (Python)

* Escucha eventos `evento_creado`, `evento_modificado`.
* Envía notificaciones (WebSockets, correos, etc.).

---

## 4. Flujo de Operaciones (Ejemplo)

### Crear un evento grupal

1. Cliente → `POST /eventos` al **API Gateway**.
2. Gateway valida JWT y **publica evento** `evento_creado` en el bus.
3. **Servicio de Agenda** consume `evento_creado`, guarda en DB local (SQLite), verifica conflictos.
4. **Servicio de Grupos** consume `evento_creado`, revisa jerarquía:

   * Si creador es superior → publica `evento_confirmado`.
   * Si es horizontal → publica `invitaciones_pendientes`.
5. **Servicio de Notificaciones** escucha ambos (`evento_confirmado` o `invitaciones_pendientes`) y envía alertas a los usuarios.
6. Cliente puede consultar su agenda vía REST al Gateway.

---

## 5. Tecnologías Utilizadas

* **Lenguajes**:

  * Python (FastAPI) → API Gateway, Usuarios, Notificaciones.
  * Go → Agenda y Grupos (opcional, para eficiencia).
* **Bus de Mensajes**:

  * Redis (pub/sub simple, rápido).
  * O RabbitMQ (si se requiere mayor control de colas y persistencia).
* **Base de datos**:

  * SQLite local para usuarios y eventos (versión centralizada).
  * Evolucionará a event sourcing + replicación en la entrega distribuida.
* **Docker Swarm**:

  * Cada servicio en un contenedor independiente.
  * Red overlay para comunicación entre nodos.

---

## 6. Ventajas de Pub/Sub en este Proyecto

* **Desacoplamiento**: los servicios no dependen directamente unos de otros.
* **Escalabilidad**: fácil añadir nuevos servicios (ej. analytics, reportes).
* **Preparado para distribución**: el bus de mensajes permite que múltiples réplicas de un servicio trabajen en paralelo.
* **Tolerancia a fallos**: si un servicio cae, los mensajes pueden esperar hasta que vuelva.
* **Transparencia para el usuario**: el sistema se comporta igual centralizado o distribuido.

---

## 7. Preparación para la Entrega 2 (Distribuida)

* Reemplazar el bus de mensajes centralizado por una versión replicada (ej. cluster de Redis o RabbitMQ).
* Implementar **replicación de eventos** entre nodos de Agenda vía logs distribuidos.
* Añadir un **middleware cliente** para dar transparencia completa.
* Escalar servicios con múltiples réplicas en Docker Swarm.

---

¡Perfecto! 🚀 Ahora que ya tenemos la arquitectura basada en **Pub/Sub**, te detallo las **imágenes Docker necesarias por servicio** para la **versión centralizada** (Entrega 1).

---

## 8. Imágenes Docker por Servicio

### 🔹 1. API Gateway

* **Lenguaje:** Python (FastAPI).
* **Imagen base:**

  ```dockerfile
  FROM python:3.12-slim
  ```
* **Dependencias principales:**

  * `fastapi`
  * `uvicorn[standard]`
  * `python-jose[cryptography]`, `passlib[bcrypt]` (JWT y seguridad)
  * `pydantic`
* **Rol:** punto de entrada único, validación de JWT, publicación de eventos al bus.
* **Nombre de imagen:**

  ```
  agenda-api-gateway
  ```

### 🔹 2. Servicio de Usuarios

* **Lenguaje:** Python (FastAPI).
* **Imagen base:**

  ```dockerfile
  FROM python:3.12-slim
  ```
* **Dependencias principales:**

  * `fastapi`, `uvicorn`
  * `sqlalchemy` (manejo SQLite)
  * `bcrypt` (hash contraseñas)
  * `redis` o `pika` (para conexión con bus)
* **Rol:** registro/login, gestión de usuarios y jerarquías, publica/consume eventos.
* **Nombre de imagen:**

  ```
  agenda-users-service
  ```

### 🔹 3. Servicio de Agenda

* **Lenguaje:** Go (ideal por concurrencia).
* **Imagen base:**

  ```dockerfile
  FROM golang:1.22-alpine
  ```
* **Dependencias principales:**

  * Librería Go para SQLite → `modernc.org/sqlite`
  * Cliente Redis (`github.com/go-redis/redis/v9`) o RabbitMQ (`github.com/streadway/amqp`)
* **Rol:** CRUD de eventos, detección de conflictos, publicación de eventos al bus.
* **Nombre de imagen:**

  ```
  agenda-events-service
  ```

*(Si prefieres simplicidad → también se puede en Python con FastAPI, misma imagen base que Usuarios).*


### 🔹 4. Servicio de Grupos

* **Lenguaje:** Go o Python (similar al de Agenda).
* **Imagen base (Go):**

  ```dockerfile
  FROM golang:1.22-alpine
  ```
* **Dependencias:**

  * Cliente Redis/RabbitMQ (igual que Agenda).
  * SQLite para persistencia local.
* **Rol:** gestiona grupos, jerarquías, aceptación/rechazo de citas grupales.
* **Nombre de imagen:**

  ```
  agenda-groups-service
  ```

### 🔹 5. Servicio de Notificaciones

* **Lenguaje:** Python (FastAPI + WebSockets).
* **Imagen base:**

  ```dockerfile
  FROM python:3.12-slim
  ```
* **Dependencias principales:**

  * `fastapi` con `websockets`
  * Cliente Redis/RabbitMQ
  * (Opcional) `smtplib` o `aiosmtplib` para correos
* **Rol:** suscriptor de eventos, envía notificaciones (WS, correo, push).
* **Nombre de imagen:**

  ```
  agenda-notifications-service
  ```

### 🔹 6. Bus de Mensajes (Broker)

Aquí tienes dos opciones:

#### Opción A: Redis (más simple)

* **Imagen oficial:**

  ```
  redis:7-alpine
  ```
* **Rol:** canal pub/sub centralizado para los servicios.
* **Nombre de imagen:**

  ```
  agenda-bus-redis
  ```

#### Opción B: RabbitMQ (más completo)

* **Imagen oficial:**

  ```
  rabbitmq:3-management
  ```
* Incluye dashboard web de gestión.
* Mejor si quieres colas persistentes y más control.
* **Nombre de imagen:**

  ```
  agenda-bus-rabbit
  ```

### Resumen de Imágenes

1. `agenda-api-gateway` → Python + FastAPI.
2. `agenda-users-service` → Python + FastAPI + SQLite.
3. `agenda-events-service` → Go + SQLite + Redis/RabbitMQ.
4. `agenda-groups-service` → Go (o Python) + SQLite + Redis/RabbitMQ.
5. `agenda-notifications-service` → Python + FastAPI (WebSockets).
6. `agenda-bus-redis` o `agenda-bus-rabbit` → broker de mensajes.


