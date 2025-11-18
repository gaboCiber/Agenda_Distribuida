
# 1. **Create a Group**:
docker exec -it agenda-redis-service redis-cli PUBLISH groups_events '{
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "type": "group.create",
    "data": {
        "name": "Developers",
        "description": "Development team",
        "is_hierarchical": false,
        "creator_id": "<USER_ID>"
    },
    "metadata": {
        "reply_to": "group_events_response"
    }
}'

# 2. **Get a Group**:
docker exec -it agenda-redis-service redis-cli PUBLISH groups_events '{
    "id": "550e8400-e29b-41d4-a716-446655440001",
    "type": "group.get",
    "data": {
        "id": "<GROUP_ID>"
    },
    "metadata": {
        "reply_to": "group_events_response"
    }
}'

# 3. **Update a Group**:
docker exec -it agenda-redis-service redis-cli PUBLISH groups_events '{
    "id": "550e8400-e29b-41d4-a716-446655440002",
    "type": "group.update",
    "data": {
        "id": "<GROUP_ID>",
        "data": {
            "name": "Developers Team",
            "description": "Updated description"
        }
    },
    "metadata": {
        "reply_to": "group_events_response"
    }
}'

# 4. **Delete a Group**:
docker exec -it agenda-redis-service redis-cli PUBLISH groups_events '{
    "id": "550e8400-e29b-41d4-a716-446655440003",
    "type": "group.delete",
    "data": {
        "id": "<GROUP_ID>"
    },
    "metadata": {
        "reply_to": "group_events_response"
    }
}'
