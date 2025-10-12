#!/bin/bash

# Test script for Group Service events via Redis
# Make this script executable: chmod +x test_redis_events.sh

# Configuration
REDIS_CONTAINER="agenda-bus-redis"  # Update this if your Redis container has a different name
REDIS_CMD="docker exec -i $REDIS_CONTAINER redis-cli"
CHANNEL="groups"  # Channel that group service is listening to
RESPONSE_CHANNEL="group_responses"  # Channel for service responses

# Colors for better output
GREEN='[SUCCESS]'
YELLOW='[WAITING]'
BLUE='[PUBLISH]'
RED='[ERROR]  '
NC='' # No Color

# Function to publish an event to Redis and wait for response
publish_event() {
    local event_type=$1
    local payload=$2
    local expect_response=${3:-false}
    
    # Create event with timestamp and unique ID
    local event_id=$(uuidgen 2>/dev/null || echo "test-$(date +%s)")
    local timestamp=$(date -u +"%Y-%m-%dT%H:%M:%S.000Z")
    
    # Add response channel to payload if it's a JSON object
    if [[ $payload == {* ]]; then
        # Add response channel and ensure it's valid JSON
        payload=$(echo "$payload" | jq -c ". + {response_channel: \"$RESPONSE_CHANNEL\"}" 2>/dev/null)
        if [ $? -ne 0 ]; then
            echo "$RED Failed to add response channel to payload"
            return 1
        fi
    fi
    
    # Construct the event JSON
    local event_json="{\"event_id\":\"$event_id\",\"type\":\"$event_type\",\"timestamp\":\"$timestamp\",\"payload\":$payload}"
    
    echo "$BLUE Publishing event: $event_type"
    echo "  $event_json"
    
    # Start listening for response in background if needed
    local response_file=""
    if [ "$expect_response" = true ]; then
        response_file=$(mktemp)
        (
            # Listen for response with a timeout
            $REDIS_CMD --raw BLPOP "$RESPONSE_CHANNEL" 10 > "$response_file"
        ) & 
        local listener_pid=$!
    fi
    
    # Publish the event and capture the response
    local response
    response=$(echo "PUBLISH $CHANNEL '$event_json'" | $REDIS_CMD 2>&1)
    
    # If we're expecting a response, write the response to the response file
    if [ "$expect_response" = true ] && [ -n "$response_file" ] && [ -n "$response" ]; then
        echo "$response" > "$response_file"
    fi
    
    # Wait for response if needed
    if [ "$expect_response" = true ]; then
        echo "$YELLOW Waiting for response to: $event_type (timeout: 10s)"
        wait $listener_pid 2>/dev/null
        
        if [ -s "$response_file" ]; then
            # Extract the JSON part from the response (removing the channel name)
            local response=$(tail -n +2 "$response_file" | jq -c . 2>/dev/null || cat "$response_file")
            
            # Try to parse the response as JSON
            local parsed_response=$(echo "$response" | jq -r . 2>/dev/null)
            if [ $? -eq 0 ] && [ "$parsed_response" != "null" ]; then
                response="$parsed_response"
            fi
            
            echo "$GREEN Received response:"
            echo "  $response"
            
            # Extract and return the group ID if this was a group creation
            if [ "$event_type" = "group_created" ] || [ "$event_type" = "group_created_response" ]; then
                local group_id=$(echo "$response" | jq -r '.payload.group_id // empty' 2>/dev/null)
                if [ -n "$group_id" ]; then
                    echo "$GREEN Extracted group_id: $group_id"
                    echo "$group_id"
                    rm -f "$response_file"
                    return 0
                fi
            fi
            
            # Check if the response indicates success
            local status=$(echo "$response" | jq -r '.status // empty' 2>/dev/null)
            if [ "$status" = "success" ]; then
                rm -f "$response_file"
                return 0
            elif [ "$status" = "error" ]; then
                local error_msg=$(echo "$response" | jq -r '.message // .error // "Unknown error"' 2>/dev/null)
                echo "$RED Error: $error_msg"
                rm -f "$response_file"
                return 1
            fi
        else
            echo "$RED Timeout waiting for response to: $event_type"
            rm -f "$response_file"
            return 1
        fi
        
        rm -f "$response_file"
    fi
    
    return 0
}

# Function to wait for a response
wait_for_response() {
    local event_type=$1
    local timeout=${2:-5}  # Default timeout: 5 seconds
    local start_time=$(date +%s)
    local container_id=$(docker ps -q --filter name=agenda-group-service 2>/dev/null)
    
    echo "$YELLOW Waiting for response: $event_type (timeout: ${timeout}s)"
    
    while [ $(($(date +%s) - start_time)) -lt $timeout ]; do
        # Check if the event was processed by looking for a response
        if [ -n "$container_id" ]; then
            if docker logs $container_id 2>&1 | tail -n 20 | grep -q "$event_type"; then
                echo "$GREEN Event processed: $event_type"
                return 0
            fi
        fi
        sleep 0.5
    done
    
    echo "$RED Timeout waiting for: $event_type"
    return 1
}

# Check if Redis container is running
if ! docker ps | grep -q $REDIS_CONTAINER; then
    echo "$YELLOW Redis container '$REDIS_CONTAINER' is not running. Starting it..."
    docker start $REDIS_CONTAINER >/dev/null 2>&1
    sleep 2
    
    if ! docker ps | grep -q $REDIS_CONTAINER; then
        echo "$RED Failed to start Redis container. Please make sure Docker is running and the container exists."
        exit 1
    fi
fi

TEST_USER_ID="test-user-$(date +%s)"
TEST_GROUP_ID="test-group-$(date +%s)"
TEST_EVENT_ID="test-event-$(date +%s)"
TEST_ADMIN_ID="test-admin-$(date +%s)"

# Function to create a test group
create_test_group() {
    local name="$1"
    local description="$2"
    local is_hierarchical="${3:-false}"
    local created_by="${4:-$TEST_USER_ID}"
    
    echo "$GREEN === Creating Group: $name ==="
    # Generate a group ID if not provided
    local group_id=$(uuidgen 2>/dev/null || echo "group-$(date +%s)")
    
    local group_payload=$(jq -n \
        --arg name "$name" \
        --arg description "$description" \
        --argjson is_hierarchical "$is_hierarchical" \
        --arg created_by "$created_by" \
        --arg group_id "$group_id" \
        '{
            group_id: $group_id,
            name: $name,
            description: $description,
            is_hierarchical: $is_hierarchical,
            created_by: $created_by,
            response_channel: "group_responses"
        }')
    
    # Publish the group creation event and capture the response
    local response_file=$(mktemp)
    local response
    
    # Publish the event and capture the response
    if ! publish_event "group_created" "$group_payload" true > "$response_file"; then
        echo "$RED Failed to publish group_created event"
        rm -f "$response_file"
        return 1
    fi
    
    # Read the response file
    response=$(cat "$response_file")
    rm -f "$response_file"
    
    # Extract the group ID from the response
    local group_id=$(echo "$response" | jq -r '.payload.group_id // empty' 2>/dev/null)
    
    if [ -z "$group_id" ]; then
        echo "$RED Failed to extract group ID from response"
        echo "Response: $response"
        return 1
    fi
    
    echo "$GREEN Successfully created group: $name (ID: $group_id)"
    echo "$group_id"
    return 0
}

# Function to add a member to a group
add_group_member() {
    local group_id="$1"
    local user_id="$2"
    local role="${3:-member}"
    local added_by="${4:-$TEST_USER_ID}"
    
    echo "$BLUE Adding member: {user_id: $user_id, role: $role, group_id: $group_id}"
    
    local member_payload=$(jq -n \
        --arg group_id "$group_id" \
        --arg user_id "$user_id" \
        --arg role "$role" \
        --arg added_by "$added_by" \
        '{
            group_id: $group_id,
            user_id: $user_id,
            role: $role,
            added_by: $added_by
        }')
    
    publish_event "member_added" "$member_payload" true
    return $?
}

# Start test workflow
echo "$GREEN === Starting Test Workflow ==="
echo "Test User ID: $TEST_USER_ID"
echo "Test Event ID: $TEST_EVENT_ID"
echo "Test Admin ID: $TEST_ADMIN_ID"
echo ""

# 1. Create a non-hierarchical group
echo "$GREEN === 1. Creating Non-Hierarchical Group ==="

# Create a temporary file to store the response
RESPONSE_FILE=$(mktemp)

# Create the group payload
GROUP_ID=$(uuidgen 2>/dev/null || echo "group-$(date +%s)")
PAYLOAD=$(jq -n \
  --arg name "Non-Hierarchical Group" \
  --arg description "A test non-hierarchical group" \
  --argjson is_hierarchical false \
  --arg created_by "$TEST_USER_ID" \
  --arg group_id "$GROUP_ID" \
  '{
    group_id: $group_id,
    name: $name,
    description: $description,
    is_hierarchical: $is_hierarchical,
    created_by: $created_by,
    response_channel: "group_responses"
  }')

# Print the payload for debugging
echo "$GREEN Sending payload:"
echo "$PAYLOAD" | jq .

# Publish the event and capture the response
echo "$YELLOW Publishing group_created event..."
if ! publish_event "group_created" "$PAYLOAD" true > "$RESPONSE_FILE"; then
  echo "$RED Failed to publish group_created event"
  rm -f "$RESPONSE_FILE"
  exit 1
fi
# fi

# # 3. Add regular members (using admin's credentials)
# echo "$GREEN === 3. Adding Regular Members ==="
# MEMBERS=(
#     '{"user_id":"user1","role":"member"}'
#     '{"user_id":"user2","role":"moderator"}'
# )

# for member in "${MEMBERS[@]}"; do
#     user_id=$(echo "$member" | jq -r '.user_id')
#     role=$(echo "$member" | jq -r '.role')
    
#     add_group_member "$TEST_GROUP_ID" "$user_id" "$role" "$TEST_ADMIN_ID"
    
#     if [ $? -ne 0 ]; then
#         echo "$RED Failed to add member: $user_id"
#     fi
# done

# # 4. List group members
# echo "$GREEN === 4. Listing Group Members ==="
# publish_event "list_members" "{\"group_id\":\"$TEST_GROUP_ID\",\"requested_by\":\"$TEST_ADMIN_ID\"}" true
# wait_for_response "members_listed" 5

# echo "$GREEN Test completed successfully!"

# # 5. Create an event
# printf "\n%s\n" "$GREEN === 5. Creating Event ==="
# publish_event "event_created" "{\"event_id\":\"$TEST_EVENT_ID\",\"title\":\"Test Event\",\"start_time\":\"2025-10-11T10:00:00Z\",\"end_time\":\"2025-10-11T11:00:00Z\",\"created_by\":\"$TEST_USER_ID\"}" true
# wait_for_response "event_created" 3

# # 6. Add event to group
# printf "\n%s\n" "$GREEN === 6. Adding Event to Group ==="
# publish_event "group_event_added" "{\"group_id\":\"$TEST_GROUP_ID\",\"event_id\":\"$TEST_EVENT_ID\",\"added_by\":\"$TEST_ADMIN_ID\"}" true
# wait_for_response "group_event_added" 3

# # 7. List group members (if supported by events)
# printf "\n%s\n" "$GREEN === 7. Listing Group Members ==="
# publish_event "list_group_members" "{\"group_id\":\"$TEST_GROUP_ID\",\"requested_by\":\"$TEST_ADMIN_ID\"}" true
# wait_for_response "group_members_listed" 3

# # 8. List group events (if supported by events)
# printf "\n%s\n" "$GREEN === 8. Listing Group Events ==="
# wait_for_response "group_events_listed" 3

# # 9. Remove event from group
# printf "\n%s\n" "$GREEN === 9. Removing Event from Group ==="
# publish_event "group_event_removed" "{\"group_id\":\"$TEST_GROUP_ID\",\"event_id\":\"$TEST_EVENT_ID\",\"removed_by\":\"$TEST_ADMIN_ID\"}" true
# wait_for_response "group_event_removed" 3

# # 11. Delete the group
# printf "\n%s\n" "$GREEN === 11. Deleting Group ==="
# publish_event "group_deleted" "{\"group_id\":\"$TEST_GROUP_ID\",\"deleted_by\":\"$TEST_ADMIN_ID\"}" true
# wait_for_response "group_deleted" 3
# # 12. Delete the event
# printf "\n%s\n" "$GREEN === 12. Deleting Event ==="
# publish_event "event_deleted" "{\"event_id\":\"$TEST_EVENT_ID\",\"deleted_by\":\"$TEST_USER_ID\"}" true
# wait_for_response "event_deleted" 3

# # 13. Delete the test user
# printf "\n%s\n" "$GREEN === 13. Deleting Test User ==="
# publish_event "user_deleted" "{\"user_id\":\"$TEST_USER_ID\"}" true
# wait_for_response "user_deleted" 3

# # 14. Delete the admin user
# publish_event "user_deleted" "{\"user_id\":\"$TEST_ADMIN_ID\"}" true
# wait_for_response "user_deleted" 3

# printf "\n%s\n" "$GREEN Test workflow completed!"
# printf "%s\n\n" "Check your group service logs for detailed information."
