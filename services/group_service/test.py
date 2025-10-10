import requests

BASE_URL = "http://localhost:8003"  # Adjust if your service runs on a different host or port

# Helper function to print response

def print_response(response):
    print(f"Status Code: {response.status_code}")
    try:
        print("Response JSON:", response.json())
    except Exception:
        print("Response Text:", response.text)
    print("---")


def test_create_group():
    print("Testing Create Group")
    payload = {
        "name": "Test Group",
        "description": "This is a test group"
    }
    response = requests.post(f"{BASE_URL}/groups", json=payload)
    print_response(response)
    return response.json().get("id") if response.status_code == 201 else None


def test_get_group(group_id):
    print("Testing Get Group")
    response = requests.get(f"{BASE_URL}/groups/{group_id}")
    print_response(response)


def test_update_group(group_id):
    print("Testing Update Group")
    payload = {
        "name": "Updated Test Group",
        "description": "Updated description"
    }
    response = requests.put(f"{BASE_URL}/groups/{group_id}", json=payload)
    print_response(response)


def test_delete_group(group_id):
    print("Testing Delete Group")
    response = requests.delete(f"{BASE_URL}/groups/{group_id}")
    print_response(response)


def test_list_groups():
    print("Testing List Groups")
    response = requests.get(f"{BASE_URL}/groups")
    print_response(response)


def test_add_member(group_id, member_id):
    print("Testing Add Member")
    payload = {"member_id": member_id}
    response = requests.post(f"{BASE_URL}/groups/{group_id}/members", json=payload)
    print_response(response)


def test_remove_member(group_id, member_id):
    print("Testing Remove Member")
    response = requests.delete(f"{BASE_URL}/groups/{group_id}/members/{member_id}")
    print_response(response)


def test_all():
    # Run all tests in sequence
    group_id = test_create_group()
    if not group_id:
        print("Failed to create group, aborting tests.")
        return

    test_get_group(group_id)
    test_update_group(group_id)
    test_list_groups()

    # For member tests, replace with actual member IDs as needed
    test_add_member(group_id, "member1")
    test_remove_member(group_id, "member1")

    test_delete_group(group_id)


if __name__ == "__main__":
    test_all()
