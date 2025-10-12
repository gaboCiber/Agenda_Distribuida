import requests
import json
import uuid
from datetime import datetime, timedelta
import time

# Configuration
BASE_URL = "http://localhost:8003"
TEST_USER = "test-user-123"
TEST_ADMIN = "test-admin-456"

# Authentication headers
def get_auth_headers(user_id):
    return {"X-User-ID": user_id, "Content-Type": "application/json"}

# Helper function to print responses
def print_response(response, title=""):
    if title:
        print(f"\n🔹 {title}")
        print("-" * 50)
    
    print(f"URL: {response.url}")
    print(f"Status Code: {response.status_code}")
    
    try:
        print("Response:")
        print(json.dumps(response.json(), indent=2))
    except:
        print("Response:", response.text)
    
    print("-" * 50)
    return response

# Test data
def create_test_group(name, description, is_hierarchical=False):
    return {
        "name": name,
        "description": description,
        "is_hierarchical": is_hierarchical,
        "created_by": TEST_USER
    }

def create_test_event(group_id, title, start_time, end_time, created_by=TEST_USER):
    return {
        "group_id": group_id,
        "title": title,
        "description": f"Test event for group {group_id}",
        "start_time": start_time.isoformat() + "Z",
        "end_time": end_time.isoformat() + "Z",
        "created_by": created_by
    }

def create_test_invitation(group_id, user_id, invited_by=TEST_USER):
    return {
        "group_id": group_id,
        "user_id": user_id,
        "invited_by": invited_by
    }

# Test cases
def test_group_operations():
    print("\n🚀 Testing Group Operations")
    print("=" * 50)
    
    # 1. Create a non-hierarchical group
    non_hier_group = create_test_group(
        "Non-Hierarchical Group", 
        "A test non-hierarchical group"
    )
    
    response = requests.post(
        f"{BASE_URL}/groups",
        json=non_hier_group,
        headers=get_auth_headers(TEST_USER)
    )
    non_hier_group_id = response.json()["id"]
    print_response(response, "Created Non-Hierarchical Group")
    
    # 2. Create a hierarchical group
    hier_group = create_test_group(
        "Hierarchical Group", 
        "A test hierarchical group",
        is_hierarchical=True
    )
    
    response = requests.post(
        f"{BASE_URL}/groups",
        json=hier_group, 
        headers=get_auth_headers(TEST_USER)
    )
    hier_group_id = response.json()["id"]
    print_response(response, "Created Hierarchical Group")
    
    return non_hier_group_id, hier_group_id
def test_member_operations(group_ids):
    non_hier_group_id, hier_group_id = group_ids
    print("\n👥 Testing Member Operations")
    print("=" * 50)
    
    # First, add the test admin as a group admin using the group creator's credentials
    admin_member = {"user_id": TEST_ADMIN, "role": "admin"}
    response = requests.post(
        f"{BASE_URL}/groups/{non_hier_group_id}/members",
        json=admin_member,
        headers=get_auth_headers(TEST_USER)  # Use the group creator's credentials
    )
    print_response(response, f"Added admin {TEST_ADMIN} to non-hierarchical group")
    
    # Now add other members using the admin's credentials
    members = [
        {"user_id": "user1", "role": "member"},
        {"user_id": "user2", "role": "moderator"}
    ]
    
    for member in members:
        response = requests.post(
            f"{BASE_URL}/groups/{non_hier_group_id}/members",
            json=member,
            headers=get_auth_headers(TEST_ADMIN)  # Now TEST_ADMIN is an admin of the group
        )
        print_response(response, f"Added member {member['user_id']} to non-hierarchical group")
    
    # Add members to hierarchical group with roles
    members = [
        {"user_id": "user3", "role": "member"},
        {"user_id": "user4", "role": "moderator"},
        {"user_id": TEST_ADMIN, "role": "admin"}
    ]
    
    for member in members:
        response = requests.post(
            f"{BASE_URL}/groups/{hier_group_id}/members",
            json=member,
            headers=get_auth_headers(TEST_USER)
        )
        print_response(response, f"Added member {member['user_id']} to hierarchical group")
    
    # List members
    response = requests.get(
        f"{BASE_URL}/groups/{hier_group_id}/members",
        headers=get_auth_headers(TEST_USER)
    )
    print_response(response, f"List of members in hierarchical group {hier_group_id}")

def test_invitation_operations(group_ids):
    non_hier_group_id, hier_group_id = group_ids
    print("\n📨 Testing Invitation Operations")
    print("=" * 50)
    
    # Create invitations
    invitees = ["invitee1", "invitee2", "invitee3"]
    invitation_ids = []
    
    for i, invitee in enumerate(invitees):
        group_id = hier_group_id if i % 2 == 0 else non_hier_group_id
        invitation = create_test_invitation(group_id, invitee)
        
        response = requests.post(
            f"{BASE_URL}/invitations",
            json=invitation,
            headers=get_auth_headers(TEST_USER)
        )
        print_response(response, f"Invited {invitee} to group {group_id}")
        
        if response.status_code == 201:
            invitation_ids.append((response.json()["id"], invitee))
    
    # List invitations for a user
    response = requests.get(
        f"{BASE_URL}/invitations/user/invitee1",
        headers=get_auth_headers("invitee1")
    )
    print_response(response, "List of invitations for invitee1")
    
    # Respond to an invitation
    if invitation_ids:
        invite_id, invitee = invitation_ids[0]
        response = requests.post(
            f"{BASE_URL}/invitations/{invite_id}/respond",
            json={"action": "accept"},  # Cambiado de 'status' a 'action' para coincidir con el manejador
            headers=get_auth_headers(invitee)
        )
        print_response(response, f"{invitee} accepted the invitation")

def test_event_operations(group_ids):
    non_hier_group_id, hier_group_id = group_ids
    print("\n📅 Testing Event Operations")
    print("=" * 50)
    
    # Create events in both groups
    events = [
        {
            "event_id": "event-1",
            "added_by": "test-user-123"
        },
        {
            "event_id": "event-2",
            "added_by": "test-admin-456"
        },
    ]
    
    # Add events to the non-hierarchical group
    for event in events:
        response = requests.post(
            f"{BASE_URL}/groups/{non_hier_group_id}/events",
            json={"event_id": event["event_id"], "added_by": event["added_by"]},
            headers=get_auth_headers(event["added_by"])
        )
        print_response(response, f"Added event {event['event_id']} to non-hierarchical group")
    
    # Add events to the hierarchical group
    for event in events:
        response = requests.post(
            f"{BASE_URL}/groups/{hier_group_id}/events",
            json={"event_id": event["event_id"], "added_by": event["added_by"]},
            headers=get_auth_headers(event["added_by"])
        )
        print_response(response, f"Added event {event['event_id']} to hierarchical group")
    
    # List events for the non-hierarchical group
    response = requests.get(
        f"{BASE_URL}/groups/{non_hier_group_id}/events",
        headers=get_auth_headers("test-user-123")
    )
    print_response(response, f"Events in non-hierarchical group {non_hier_group_id}")
    
    # List events for the hierarchical group
    response = requests.get(
        f"{BASE_URL}/groups/{hier_group_id}/events",
        headers=get_auth_headers("test-user-123")
    )
    print_response(response, f"Events in hierarchical group {hier_group_id}")
    
    # Remove an event from the non-hierarchical group
    if events:
        event_id = events[0]["event_id"]
        response = requests.delete(
            f"{BASE_URL}/groups/{non_hier_group_id}/events/{event_id}",
            headers=get_auth_headers("test-user-123")
        )
        print_response(response, f"Removed event {event_id} from non-hierarchical group {non_hier_group_id}")
        
        # Verify the event was removed by listing events again
        response = requests.get(
            f"{BASE_URL}/groups/{non_hier_group_id}/events",
            headers=get_auth_headers("test-user-123")
        )
        print_response(response, f"Events in non-hierarchical group {non_hier_group_id} after removal")


def run_all_tests():
    print("🔍 Starting Integration Tests")
    print("=" * 50)
    
    # Wait for service to be ready
    print("⏳ Waiting for service to be ready...")
    for _ in range(10):  # Try for 10 seconds
        try:
            response = requests.get(f"{BASE_URL}/health")
            if response.status_code == 200:
                print("✅ Service is ready!")
                break
        except:
            pass
        time.sleep(1)
    else:
        print("❌ Service is not responding. Please start the service first.")
        return
    
    # Run tests
    try:
        group_ids = test_group_operations()
        test_member_operations(group_ids)
        test_invitation_operations(group_ids)
        test_event_operations(group_ids)
        
        print("\n🎉 All tests completed successfully!")
    except Exception as e:
        print(f"\n❌ Test failed: {str(e)}")
        raise

if __name__ == "__main__":
    run_all_tests()
