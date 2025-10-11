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
            json={"status": "accepted"},
            headers=get_auth_headers(invitee)
        )
        print_response(response, f"{invitee} accepted the invitation")

def test_event_operations(group_ids):
    non_hier_group_id, hier_group_id = group_ids
    print("\n📅 Testing Event Operations")
    print("=" * 50)
    
    # Create events in both groups
    now = datetime.utcnow()
    events = [
        create_test_event(
            non_hier_group_id,
            "Team Meeting",
            now + timedelta(days=1),
            now + timedelta(days=1, hours=1)
        ),
        create_test_event(
            hier_group_id,
            "Project Review",
            now + timedelta(days=2),
            now + timedelta(days=2, hours=2)
        )
    ]
    
    event_ids = []
    for event in events:
        response = requests.post(
            f"{BASE_URL}/events",
            json=event,
            headers=get_auth_headers(event["created_by"])
        )
        print_response(response, f"Created event: {event['title']}")
        
        if response.status_code == 201:
            event_ids.append(response.json()["id"])
    
    # List events for a group
    response = requests.get(
        f"{BASE_URL}/groups/{non_hier_group_id}/events",
        headers=get_auth_headers(TEST_USER)
    )
    print_response(response, f"Events in non-hierarchical group {non_hier_group_id}")
    
    # Get event details
    if event_ids:
        response = requests.get(
            f"{BASE_URL}/events/{event_ids[0]}",
            headers=get_auth_headers(TEST_USER)
        )
        print_response(response, f"Details for event {event_ids[0]}")

def test_group_hierarchy():
    print("\n🌳 Testing Group Hierarchy")
    print("=" * 50)
    
    # Create a parent group
    parent_group = create_test_group(
        "Parent Group", 
        "Parent group for hierarchy testing",
        is_hierarchical=True
    )
    
    response = requests.post(
        f"{BASE_URL}/groups", 
        json=parent_group,
        headers=get_auth_headers(TEST_USER)
    )
    parent_group_id = response.json()["id"]
    print_response(response, "Created Parent Group")
    
    # Create child groups
    child_groups = []
    for i in range(2):
        child = create_test_group(
            f"Child Group {i+1}",
            f"Child group {i+1} of {parent_group_id}",
            is_hierarchical=True
        )
        response = requests.post(
            f"{BASE_URL}/groups",
            json=child,
            headers=get_auth_headers(TEST_USER)
        )
        if response.status_code != 201:
            print(f"Error creating child group: {response.text}")
            raise Exception(f"Error creating child group: {response.text}")
        child_id = response.json()["id"]
        child_groups.append(child_id)
        
        # Link child to parent
        response = requests.post(
            f"{BASE_URL}/groups/{parent_group_id}/children",
            json={"child_group_id": child_id},
            headers=get_auth_headers(TEST_USER)
        )
        print_response(response, f"Linked child group {child_id} to parent {parent_group_id}")
    
    # Get group hierarchy
    response = requests.get(
        f"{BASE_URL}/groups/{parent_group_id}/hierarchy",
        headers=get_auth_headers(TEST_USER)
    )
    print_response(response, f"Hierarchy for group {parent_group_id}")

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
        test_group_hierarchy()
        
        print("\n🎉 All tests completed successfully!")
    except Exception as e:
        print(f"\n❌ Test failed: {str(e)}")
        raise

if __name__ == "__main__":
    run_all_tests()
