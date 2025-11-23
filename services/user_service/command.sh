
docker exec -it agenda-redis-service redis-cli SUBSCRIBE users_events_response


#####################################################################
#                            USERS                                  #
#####################################################################

# 1. **Crear un usuario**:

docker exec -it agenda-redis-service redis-cli PUBLISH users_events '{
  "id": "123e4567-e89b-12d3-a456-426614174000",
  "type": "user.create",
  "data": {
    "email": "usuario@ejemplo.com",
    "password": "micontraseña",
    "username": "usuario1"
  },
  "metadata": {
    "reply_to": "users_events_response"
  }
}'


# 2. **Obtener un usuario por ID** (reemplaza `USER_ID` con un ID válido):

docker exec -it agenda-redis-service redis-cli PUBLISH users_events '{
  "id": "123e4567-e89b-12d3-a456-426614174001",
  "type": "user.get",
  "data": {
    "user_id": "USER_ID"
  },
  "metadata": {
    "reply_to": "users_events_response"
  }
}'


# 3. **Iniciar sesión**:

docker exec -it agenda-redis-service redis-cli PUBLISH users_events '{
  "id": "123e4567-e89b-12d3-a456-426614174002",
  "type": "user.login",
  "data": {
    "email": "usuario@ejemplo.com",
    "password": "micontraseña"
  },
  "metadata": {
    "reply_to": "users_events_response"
  }
}'


# 4. **Eliminar un usuario** (reemplaza `USER_ID` con un ID válido):

docker exec -it agenda-redis-service redis-cli PUBLISH users_events '{
  "id": "123e4567-e89b-12d3-a456-426614174003",
  "type": "user.delete",
  "data": {
    "user_id": "USER_ID"
  },
  "metadata": {
    "reply_to": "users_events_response"
  }
}'

# 5. **Actualizar un usuario**

docker exec -it agenda-redis-service redis-cli PUBLISH users_events '{
  "id": "123e4567-e89b-12d3-a456-426614174004",
  "type": "user.update",
  "data": {
    "user_id": "ID_DEL_USUARIO",
    "username": "nuevo_usuario",
    "email": "nuevo@email.com"
  },
  "metadata": {
    "reply_to": "users_events_response"
  }
}'

#####################################################################
#                            EVENTS                                 #
#####################################################################

# 1. **Create an Event**:

docker exec -it agenda-redis-service redis-cli PUBLISH users_events '{
  "id": "123e4567-e89b-12d3-a456-426614174010",
  "type": "agenda.event.create",
  "data": {
    "title": "Reunión de equipo",
    "description": "Revisión del sprint actual",
    "start_time": "2025-11-17T10:00:00Z",
    "end_time": "2025-11-17T11:00:00Z",
    "location": "Sala de conferencias",
    "user_id": "user-123"
  },
  "metadata": {
    "reply_to": "users_events_response"
  }
}'


# 2. **Get an Event**:

docker exec -it agenda-redis-service redis-cli PUBLISH users_events '{
  "id": "123e4567-e89b-12d3-a456-426614174011",
  "type": "agenda.event.get",
  "data": {
    "event_id": "EVENT_ID_HERE"
  },
  "metadata": {
    "reply_to": "users_events_response"
  }
}'


# 3. **Update an Event**:

docker exec -it agenda-redis-service redis-cli PUBLISH users_events '{
  "id": "123e4567-e89b-12d3-a456-426614174012",
  "type": "agenda.event.update",
  "data": {
    "user_id": "USER_ID_HERE",
    "event_id": "EVENT_ID_HERE",
    "title": "Reunión de equipo (actualizada)",
    "description": "Revisión del sprint actual - actualizada"
  },
  "metadata": {
    "reply_to": "users_events_response"
  }
}'


# 4. **Delete an Event**:

docker exec -it agenda-redis-service redis-cli PUBLISH users_events '{
  "id": "123e4567-e89b-12d3-a456-426614174013",
  "type": "agenda.event.delete",
  "data": {
    "event_id": "EVENT_ID_HERE"
  },
  "metadata": {
    "reply_to": "users_events_response"
  }
}'


# 5. **List Events by User** (sin paginación, usa valores por defecto):

docker exec -it agenda-redis-service redis-cli PUBLISH users_events '{
  "id": "123e4567-e89b-12d3-a456-426614174014",
  "type": "agenda.event.list",
  "data": {
    "user_id": "USER_ID_HERE"
  },
  "metadata": {
    "reply_to": "users_events_response"
  }
}'


# 6. **List Events by User** (con paginación):

docker exec -it agenda-redis-service redis-cli PUBLISH users_events '{
  "id": "123e4567-e89b-12d3-a456-426614174015",
  "type": "agenda.event.list",
  "data": {
    "user_id": "USER_ID_HERE",
    "offset": 0,
    "limit": 50
  },
  "metadata": {
    "reply_to": "users_events_response"
  }
}'
