
docker exec -it agenda-redis-service redis-cli SUBSCRIBE users_events_response

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