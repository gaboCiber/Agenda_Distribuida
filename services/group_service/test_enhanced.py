import os
import sys
import requests
import sqlite3
from pathlib import Path

# Configuration
BASE_URL = "http://localhost:8003"
DB_PATH = "./data/groups.db"

# Helper function to print response
def print_response(response, show_headers=False):
    print(f"URL: {response.url}")
    print(f"Status Code: {response.status_code}")
    if show_headers:
        print("Response Headers:", response.headers)
    try:
        print("Response JSON:", response.json())
    except Exception:
        print("Response Text:", response.text)
    print("---")

def check_database():
    """Check if the database file exists and is accessible"""
    db_file = Path(DB_PATH)
    if not db_file.exists():
        print(f"❌ Database file not found at: {DB_PATH}")
        print("Make sure the service has been started to create the database.")
        return False
    
    try:
        conn = sqlite3.connect(DB_PATH)
        cursor = conn.cursor()
        cursor.execute("SELECT name FROM sqlite_master WHERE type='table';")
        tables = cursor.fetchall()
        print("✅ Database exists and is accessible")
        print(f"Found tables: {[t[0] for t in tables]}")
        conn.close()
        return True
    except Exception as e:
        print(f"❌ Error accessing database: {e}")
        return False

def test_health_check():
    """Test the health check endpoint"""
    print("\n🔍 Testing health check...")
    response = requests.get(f"{BASE_URL}/health")
    print_response(response)
    return response.status_code == 200

def test_create_group():
    """Test creating a new group"""
    print("\n🔍 Testing Create Group")
    payload = {
        "name": "Test Group",
        "description": "This is a test group"
    }
    headers = {
        "Content-Type": "application/json",
        "X-User-ID": "test-user-123"  # Required for authentication
    }
    
    print(f"Making POST request to {BASE_URL}/groups")
    print(f"Payload: {payload}")
    
    response = requests.post(
        f"{BASE_URL}/groups",
        json=payload,
        headers=headers
    )
    
    print_response(response, show_headers=True)
    
    if response.status_code == 201:
        data = response.json()
        print(f"✅ Group created with ID: {data.get('id')}")
        return data.get("id")
    else:
        print("❌ Failed to create group")
        return None

def test_get_group(group_id):
    """Test retrieving a group by ID"""
    if not group_id:
        print("Skipping get group test - no group ID")
        return
        
    print(f"\n🔍 Testing Get Group {group_id}")
    response = requests.get(f"{BASE_URL}/groups/{group_id}")
    print_response(response)
    return response.status_code == 200

def test_all():
    """Run all tests"""
    print("🚀 Starting Group Service Tests")
    print("=" * 50)
    
    # Check database first
    if not check_database():
        print("\n⚠️  Database check failed. Make sure the service is running and can create the database.")
    
    # Test health check
    if not test_health_check():
        print("\n❌ Health check failed. Is the service running?")
        print(f"   Try running: go run cmd/api/main.go")
        return
    
    # Test group operations
    group_id = test_create_group()
    if group_id:
        test_get_group(group_id)
    
    print("\n✅ Tests completed")

if __name__ == "__main__":
    test_all()
