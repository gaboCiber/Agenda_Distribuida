package events

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Event represents a domain event
type Event struct {
	ID        string      `json:"id"`
	Type      string      `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Payload   interface{} `json:"payload"`
}

// Publisher handles publishing events
type Publisher struct {
	redisClient *RedisClient
}

// NewPublisher creates a new event publisher
func NewPublisher(redisClient *RedisClient) *Publisher {
	return &Publisher{
		redisClient: redisClient,
	}
}

// Publish publishes an event to the specified channel
func (p *Publisher) Publish(channel, eventType string, payload interface{}) error {
	event := Event{
		ID:        uuid.New().String(),
		Type:      eventType,
		Timestamp: time.Now().UTC(),
		Payload:   payload,
	}

	return p.redisClient.Publish(channel, event)
}

// PublishGroupCreated publishes a group created event
func (p *Publisher) PublishGroupCreated(groupID, name, createdBy string) error {
	payload := map[string]interface{}{
		"group_id":   groupID,
		"name":       name,
		"created_by": createdBy,
	}
	return p.Publish("groups", "group_created", payload)
}

// PublishGroupUpdated publishes a group updated event
func (p *Publisher) PublishGroupUpdated(groupID, name, updatedBy string) error {
	payload := map[string]interface{}{
		"group_id":   groupID,
		"name":       name,
		"updated_by": updatedBy,
	}
	return p.Publish("groups", "group_updated", payload)
}

// PublishGroupDeleted publishes a group deleted event
func (p *Publisher) PublishGroupDeleted(groupID, deletedBy string) error {
	payload := map[string]interface{}{
		"group_id":   groupID,
		"deleted_by": deletedBy,
	}
	return p.Publish("groups", "group_deleted", payload)
}

// PublishMemberAdded publishes a member added to group event
func (p *Publisher) PublishMemberAdded(groupID, userID, addedBy string) error {
	payload := map[string]interface{}{
		"group_id": groupID,
		"user_id":  userID,
		"added_by": addedBy,
	}
	return p.Publish("groups", "member_added", payload)
}

// PublishMemberRemoved publishes a member removed from group event
func (p *Publisher) PublishMemberRemoved(groupID, userID, removedBy string) error {
	payload := map[string]interface{}{
		"group_id":   groupID,
		"user_id":    userID,
		"removed_by": removedBy,
	}
	return p.Publish("groups", "member_removed", payload)
}

// PublishInvitationSent publishes a group invitation sent event
func (p *Publisher) PublishInvitationSent(invitationID, groupID, userID, invitedBy string) error {
	payload := map[string]interface{}{
		"invitation_id": invitationID,
		"group_id":      groupID,
		"user_id":       userID,
		"invited_by":    invitedBy,
	}
	return p.Publish("groups", "invitation_sent", payload)
}

// PublishInvitationAccepted publishes a group invitation accepted event
func (p *Publisher) PublishInvitationAccepted(invitationID, groupID, userID string) error {
	payload := map[string]interface{}{
		"invitation_id": invitationID,
		"group_id":      groupID,
		"user_id":       userID,
	}
	return p.Publish("groups", "invitation_accepted", payload)
}

// PublishInvitationRejected publishes a group invitation rejected event
func (p *Publisher) PublishInvitationRejected(invitationID, groupID, userID string) error {
	payload := map[string]interface{}{
		"invitation_id": invitationID,
		"group_id":      groupID,
		"user_id":       userID,
	}
	return p.Publish("groups", "invitation_rejected", payload)
}

// PublishGroupEventAdded publishes a group event added event
func (p *Publisher) PublishGroupEventAdded(groupID, eventID, addedBy string) error {
	payload := map[string]interface{}{
		"group_id": groupID,
		"event_id": eventID,
		"added_by": addedBy,
	}
	return p.Publish("groups", "group_event_added", payload)
}

// PublishGroupEventRemoved publishes a group event removed event
func (p *Publisher) PublishGroupEventRemoved(groupID, eventID, removedBy string) error {
	payload := map[string]interface{}{
		"group_id":   groupID,
		"event_id":   eventID,
		"removed_by": removedBy,
	}
	return p.Publish("groups", "group_event_removed", payload)
}
