# 1. Crear un nuevo usuario

curl -X POST http://localhost:8000/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "email": "test@example.com",
    "password": "securepassword123"
  }'

# 2. Obtener un usuario por ID

curl -X GET http://localhost:8000/api/v1/users/USER_ID \
  -H "Content-Type: application/json"

#3. Actualizar un usuario

curl -X PUT http://localhost:8000/api/v1/users/USER_ID \
  -H "Content-Type: application/json" \
  -d '{
    "username": "updateduser",
    "email": "updated@example.com"
  }'

# 4. Eliminar un usuario

curl -X DELETE http://localhost:8000/api/v1/users/USER_ID \
  -H "Content-Type: application/json"