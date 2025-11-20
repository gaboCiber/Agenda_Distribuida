
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
        "user_id": "'$USER2_UUID'",
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

# 14. Create another invitation for testing rejection
docker exec -it agenda-redis-service redis-cli PUBLISH groups_events '{
    "id": "880e8400-e29b-41d4-a716-446655440104",
    "type": "group.invite.create",
    "data": {
        "group_id": "'$GROUP_UUID'",
        "user_id": "'$USER2_UUID'",
        "invited_by": "'$USER1_UUID'"
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
