
#####################################################################
#                            Groups                                 #
#####################################################################

# Replace these with actual IDs from your system
USER1_UUID="11111111-1111-1111-1111-111111111111"
USER2_UUID="22222222-2222-2222-2222-222222222222"
GROUP_UUID="00000000-0000-0000-0000-000000000000"


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

#####################################################################
#                       Group Members                               #
#####################################################################

# 5. **Add Member to Group**
# Adds USER2_UUID as a member to the group (added by USER1_UUID)
docker exec -it agenda-redis-service redis-cli PUBLISH groups_events '{
    "id": "660e8400-e29b-41d4-a716-446655440100",
    "type": "group.member.add",
    "data": {
        "group_id": "'$GROUP_UUID'",
        "user_id": "'$USER2_UUID'",
        "role": "member",
        "added_by": "'$USER1_UUID'"
    },
    "metadata": {
        "reply_to": "group_events_response"
    }
}'

# 6. **Add Admin to Group**
# Adds USER2_UUID as an admin to the group (added by USER1_UUID)
docker exec -it agenda-redis-service redis-cli PUBLISH groups_events '{
    "id": "660e8400-e29b-41d4-a716-446655440101",
    "type": "group.member.add",
    "data": {
        "group_id": "'$GROUP_UUID'",
        "user_id": "'$USER2_UUID'",
        "role": "admin",
        "added_by": "'$USER1_UUID'"
    },
    "metadata": {
        "reply_to": "group_events_response"
    }
}'

# 7. **List Group Members**
# Lists all members of the group
docker exec -it agenda-redis-service redis-cli PUBLISH groups_events '{
    "id": "660e8400-e29b-41d4-a716-446655440102",
    "type": "group.member.list",
    "data": {
        "group_id": "'$GROUP_UUID'"
    },
    "metadata": {
        "reply_to": "group_events_response"
    }
}'

# 8. **Remove Member from Group**
# Removes USER2_UUID from the group (removed by USER1_UUID)
docker exec -it agenda-redis-service redis-cli PUBLISH groups_events '{
    "id": "660e8400-e29b-41d4-a716-446655440103",
    "type": "group.member.remove",
    "data": {
        "group_id": "'$GROUP_UUID'",
        "user_id": "'$USER2_UUID'",
        "removed_by": "'$USER1_UUID'"
    },
    "metadata": {
        "reply_to": "group_events_response"
    }
}'
