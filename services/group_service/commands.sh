
#####################################################################
#                            Groups                                 #
#####################################################################

# 1. **Create a Group**:
docker exec -it agenda-redis-service redis-cli PUBLISH groups_events '{
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "type": "group.create",
    "data": {
        "name": "Developers",
        "description": "Development team",
        "is_hierarchical": true,
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

# 5. **List Group Members**
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

# 6. **Remove Member from Group**
# Removes USER2_UUID from the group (removed by USER1_UUID)
docker exec -it agenda-redis-service redis-cli PUBLISH groups_events '{
    "id": "660e8400-e29b-41d4-a716-446655440103",
    "type": "group.member.remove",
    "data": {
        "group_id": "'$GROUP_UUID'",
        "email": "'$USER2_UUID'"
    },
    "metadata": {
        "reply_to": "group_events_response"
    }
}'

# 7. **Update Member Role**
docker exec -it agenda-redis-service redis-cli PUBLISH groups_events '{
    "id": "660e8400-e29b-41d4-a716-446655440103",
    "type": "group.member.update",
    "data": {
        "group_id": "'$GROUP_UUID'",
        "email": "'$USER2_UUID'",
        "role": "admin",
    },
    "metadata": {
        "reply_to": "group_events_response"
    }
}'

# 9. List all groups for a user
docker exec -it agenda-redis-service redis-cli PUBLISH groups_events '{
    "id": "770e8400-e29b-41d4-a716-446655440100",
    "type": "user.groups.list",
    "data": {
        "user_id": "'$USER1_UUID'"
    },
    "metadata": {
        "reply_to": "group_events_response"
    }
}'

#####################################################################
#                       Group Invitations                           #
#####################################################################

# 10. Create Invitation
# USER1 invites USER2 to the group
docker exec -it agenda-redis-service redis-cli PUBLISH groups_events '{
    "id": "880e8400-e29b-41d4-a716-446655440100",
    "type": "group.invite.create",
    "data": {
        "group_id": "'$GROUP_UUID'",
        "email": "'$USER2_UUID'",
        "invited_by": "'$USER1_UUID'"
    },
    "metadata": {
        "reply_to": "group_events_response"
    }
}'

# 11. Get Invitation
docker exec -it agenda-redis-service redis-cli PUBLISH groups_events '{
    "id": "880e8400-e29b-41d4-a716-446655440101",
    "type": "group.invite.get",
    "data": {
        "invitation_id": "'$INVITATION_ID'"
    },
    "metadata": {
        "reply_to": "group_events_response"
    }
}'

# 12. List User Invitations (for USER2)
docker exec -it agenda-redis-service redis-cli PUBLISH groups_events '{
    "id": "880e8400-e29b-41d4-a716-446655440102",
    "type": "group.invite.list",
    "data": {
        "user_id": "'$USER2_UUID'",
        "status": "pending"
    },
    "metadata": {
        "reply_to": "group_events_response"
    }
}'

# 13. Accept Invitation (as USER2)
docker exec -it agenda-redis-service redis-cli PUBLISH groups_events '{
    "id": "880e8400-e29b-41d4-a716-446655440103",
    "type": "group.invite.accept",
    "data": {
        "invitation_id": "'$INVITATION_ID'",
        "user_id": "'$USER2_UUID'"
    },
    "metadata": {
        "reply_to": "group_events_response"
    }
}'

# 15. Reject Invitation (as USER2)
docker exec -it agenda-redis-service redis-cli PUBLISH groups_events '{
    "id": "880e8400-e29b-41d4-a716-446655440105",
    "type": "group.invite.reject",
    "data": {
        "invitation_id": "'$INVITATION_ID_REJECT'",
        "user_id": "'$USER2_UUID'"
    },
    "metadata": {
        "reply_to": "group_events_response"
    }
}'

# 17. Cancel Invitation (as USER1 who created it)
docker exec -it agenda-redis-service redis-cli PUBLISH groups_events '{
    "id": "880e8400-e29b-41d4-a716-446655440107",
    "type": "group.invite.cancel",
    "data": {
        "invitation_id": "'$INVITATION_ID_CANCEL'",
        "user_id": "'$USER1_UUID'"
    },
    "metadata": {
        "reply_to": "group_events_response"
    }
}'

# 18. List all invitations for USER2 (all statuses)
docker exec -it agenda-redis-service redis-cli PUBLISH groups_events '{
    "id": "880e8400-e29b-41d4-a716-446655440108",
    "type": "group.invite.list",
    "data": {
        "user_id": "'$USER2_UUID'"
    },
    "metadata": {
        "reply_to": "group_events_response"
    }
}'

#####################################################################
#                         Group Events                              #
#####################################################################

# 1. **Create a Group Event**:
# For hierarchical groups (admin creates, auto-accepted by all)
docker exec -it agenda-redis-service redis-cli PUBLISH groups_events '{
    "id": "123e4567-e89b-12d3-a456-426614174011",
    "type": "group.event.create",
    "data": {
        "group_id": "<GROUP_ID>",
        "event_id": "<EVENT_ID_1>",
        "user_id": "<USER1_ID>",
        "is_hierarchical": true
    },
    "metadata": {
        "reply_to": "group_events_response"
    }
}'

# For non-hierarchical groups (any member creates, pending acceptance)
docker exec -it agenda-redis-service redis-cli PUBLISH groups_events '{
    "id": "123e4567-e89b-12d3-a456-426614174012",
    "type": "group.event.create",
    "data": {
        "group_id": "<GROUP_ID>",
        "event_id": "<EVENT_ID_2>",
        "added_by": "<USER2_ID>",
        "is_hierarchical": false
    },
    "metadata": {
        "reply_to": "group_events_response"
    }
}'

# 2. **Get a Group Event**:
docker exec -it agenda-redis-service redis-cli PUBLISH groups_events '{
    "id": "123e4567-e89b-12d3-a456-426614174013",
    "type": "group.event.get",
    "data": {
        "group_id": "<GROUP_ID>",
        "event_id": "<EVENT_ID_1>",
        "user_id": "<USER1_ID>"
    },
    "metadata": {
        "reply_to": "group_events_response"
    }
}'

# 3. **List All Events in a Group**:
docker exec -it agenda-redis-service redis-cli PUBLISH groups_events '{
    "id": "123e4567-e89b-12d3-a456-426614174014",
    "type": "group.event.list",
    "data": {
        "group_id": "<GROUP_ID>",
        "user_id": "<USER1_ID>"
    },
    "metadata": {
        "reply_to": "group_events_response"
    }
}'

# 4. **Update Event Status** (Accept/Decline):
# Accept an event
docker exec -it agenda-redis-service redis-cli PUBLISH groups_events '{
    "id": "123e4567-e89b-12d3-a456-426614174015",
    "type": "group.event.status.update",
    "data": {
        "group_id": "<GROUP_ID>",
        "event_id": "<EVENT_ID_2>",
        "user_id": "<USER1_ID>",
        "status": "accepted"
    },
    "metadata": {
        "reply_to": "group_events_response"
    }
}'

# Decline an event
docker exec -it agenda-redis-service redis-cli PUBLISH groups_events '{
    "id": "123e4567-e89b-12d3-a456-426614174016",
    "type": "group.event.status.update",
    "data": {
        "group_id": "<GROUP_ID>",
        "event_id": "<EVENT_ID_2>",
        "user_id": "<USER2_ID>",
        "status": "declined"
    },
    "metadata": {
        "reply_to": "group_events_response"
    }
}'

# 5. **Get Event Status**:
docker exec -it agenda-redis-service redis-cli PUBLISH groups_events '{
    "id": "123e4567-e89b-12d3-a456-426614174017",
    "type": "group.event.status.get",
    "data": {
        "group_id": "<GROUP_ID>",
        "event_id": "<EVENT_ID_2>",
        "user_id": "<USER1_ID>"
    },
    "metadata": {
        "reply_to": "group_events_response"
    }
}'

# 6. **Delete a Group Event** (Admin only for hierarchical groups):
docker exec -it agenda-redis-service redis-cli PUBLISH groups_events '{
    "id": "123e4567-e89b-12d3-a456-426614174018",
    "type": "group.event.delete",
    "data": {
        "group_id": "<GROUP_ID>",
        "event_id": "<EVENT_ID_1>",
        "user_id": "<USER1_ID>"
    },
    "metadata": {
        "reply_to": "group_events_response"
    }
}'

# 7. **List Event Statuses** (for group admins or event creator):
docker exec -it agenda-redis-service redis-cli PUBLISH groups_events '{
    "id": "123e4567-e89b-12d3-a456-426614174019",
    "type": "group.event.status.list",
    "data": {
        "group_id": "<GROUP_ID>",
        "event_id": "<EVENT_ID_2>",
        "user_id": "<USER1_ID>"
    },
    "metadata": {
        "reply_to": "group_events_response"
    }
}'