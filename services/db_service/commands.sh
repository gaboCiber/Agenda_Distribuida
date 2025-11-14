#####################################################################
#                             USERS                                 #
#####################################################################

curl -X GET http://localhost:8000/health 

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

# 5. Login
curl -X POST http://localhost:8000/api/v1/users/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "securepassword123"
  }'


#####################################################################
#                            EVENTS                                 #
#####################################################################

### 1. Crear un nuevo evento

curl -X POST http://localhost:8000/api/v1/events \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Reunión de equipo",
    "description": "Reunión semanal de seguimiento",
    "start_time": "2025-11-13T10:00:00Z",
    "end_time": "2025-11-13T11:00:00Z",
    "user_id": "8318f0ff-02c3-4461-af07-c19e3d144068"
  }'

### 2. Obtener un evento por ID

# Reemplaza EVENT_ID con el ID del evento que acabas de crear
curl -X GET http://localhost:8000/api/v1/events/EVENT_ID \
  -H "Content-Type: application/json"

### 3. Actualizar un evento

curl -X PUT http://localhost:8000/api/v1/events/EVENT_ID \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Reunión de equipo actualizada",
    "description": "Reunión semanal de seguimiento - Actualizada",
    "start_time": "2025-11-13T14:00:00Z",
    "end_time": "2025-11-13T15:00:00Z",
    "user_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
  }'

### 4. Eliminar un evento

curl -X DELETE http://localhost:8000/api/v1/events/EVENT_ID \
  -H "Content-Type: application/json"

### 5. Verificar conflicto de horarios

curl -X POST http://localhost:8000/api/v1/events \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Evento 1",
    "description": "Primer evento",
    "start_time": "2025-11-13T09:00:00Z",
    "end_time": "2025-11-13T10:00:00Z",
    "user_id": "8318f0ff-02c3-4461-af07-c19e3d144068"
  }'

# Segundo evento que se solapa (debería fallar)
curl -X POST http://localhost:8000/api/v1/events \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Evento 2",
    "description": "Segundo evento que se solapa",
    "start_time": "2025-11-13T09:30:00Z",
    "end_time": "2025-11-13T10:30:00Z",
    "user_id": "8318f0ff-02c3-4461-af07-c19e3d144068"
  }'

#####################################################################
#                            Groups                                 #
#####################################################################

# 1. Create a Group

curl -X POST http://localhost:8000/api/v1/groups \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer 8318f0ff-02c3-4461-af07-c19e3d144068" \
  -d '{
    "name": "Test Group",
    "description": "A test group",
    "is_hierarchical": true
  }'

# 2. Add a Member to the Group

curl -X POST http://localhost:8000/api/v1/groups/{groupID}/members \
  -H "Content-Type: application/json" \
  -d '{
    "userId": "550e8400-e29b-41d4-a716-446655440001",
    "role": "admin"
  }'

# 3. List Group Members

# List all members of the group
# Replace {groupId} with the actual group ID
curl http://localhost:8000/api/v1/groups/{groupId}/members

# 4. Create a Subgroup (Child Group)

# Create a subgroup (child group)
# Replace {parentGroupId} with the parent group ID
curl -X POST http://localhost:8000/api/v1/groups \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Subgroup",
    "description": "A subgroup",
    "is_hierarchical": true,
    "parent_group_id": "{parentGroupId}",
    "creator_id": "8318f0ff-02c3-4461-af07-c19e3d144068"
  }'

# 5. Verify Member Inheritance

# Check if the member was inherited to the subgroup
# Replace {subgroupId} with the child group ID
curl http://localhost:8000/api/v1/groups/{subgroupId}/members

# 6. Remove a Member

# Remove a member from the group
# Replace {groupId} and {userId} with actual values
curl -X DELETE http://localhost:8000/api/v1/groups/{groupId}/members/{userId}

# 7. List User's Groups

# List all groups a user is member of
# Replace {userId} with the user ID
curl http://localhost:8000/api/v1/groups/users/{userId}