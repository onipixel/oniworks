package controllers

import (
	"fmt"
	"time"

	"onigram/app/models"

	"github.com/onipixel/oniworks/framework/database"
	onihttp "github.com/onipixel/oniworks/framework/http"
)

type MessageController struct {
	NotifyFn func(userID int64, event string, payload any)
}

// Inbox returns all conversations for the authenticated user, newest first.
// GET /api/messages
func (ctrl *MessageController) Inbox(c *onihttp.Context) error {
	uid, _ := c.Get("user_id")
	userID, _ := uid.(int64)

	convos := make([]models.Conversation, 0)
	err := database.Table("conversations").
		Where("user1_id = ? OR user2_id = ?", userID, userID).
		OrderBy("last_message_at DESC NULLS LAST").
		Limit(50).
		All(&convos)
	if err != nil {
		return err
	}

	if len(convos) > 0 {
		// Batch load other users
		otherIDs := make([]any, 0, len(convos))
		seen := map[int64]bool{}
		for _, conv := range convos {
			otherID := conv.User2ID
			if conv.User2ID == userID {
				otherID = conv.User1ID
			}
			if !seen[otherID] {
				otherIDs = append(otherIDs, otherID)
				seen[otherID] = true
			}
		}
		var otherUsers []models.User
		_ = database.Table("users").Select("id", "username", "avatar_path").
			WhereIn("id", otherIDs...).All(&otherUsers)
		userMap := make(map[int64]*models.User)
		for i := range otherUsers {
			userMap[otherUsers[i].ID] = &otherUsers[i]
		}

		// Batch load last messages and unread counts
		convoIDs := make([]any, len(convos))
		for i, c := range convos {
			convoIDs[i] = c.ID
		}
		var lastMsgs []models.Message
		_ = database.Raw(`
			SELECT DISTINCT ON (conversation_id) id, conversation_id, sender_id, body, read, created_at
			FROM messages
			WHERE conversation_id = ANY($1::bigint[])
			ORDER BY conversation_id, created_at DESC
		`, fmt.Sprintf("{%s}", joinIDs(convoIDs))).All(&lastMsgs)
		lastMsgMap := map[int64]*models.Message{}
		for i := range lastMsgs {
			lastMsgMap[lastMsgs[i].ConversationID] = &lastMsgs[i]
		}

		for i := range convos {
			otherID := convos[i].User2ID
			if convos[i].User2ID == userID {
				otherID = convos[i].User1ID
			}
			convos[i].OtherUser = userMap[otherID]
			convos[i].LastMessage = lastMsgMap[convos[i].ID]

			// Unread count
			cnt, _ := database.Table("messages").
				Where("conversation_id = ? AND sender_id != ? AND read = ?", convos[i].ID, userID, false).
				Count()
			convos[i].UnreadCount = int(cnt)
		}
	}

	return c.JSON(200, map[string]any{"conversations": convos})
}

// Thread returns messages in a conversation with a specific user.
// GET /api/messages/:username
func (ctrl *MessageController) Thread(c *onihttp.Context) error {
	uid, _ := c.Get("user_id")
	userID, _ := uid.(int64)
	username := c.Param("username")

	var other models.User
	if err := database.Table("users").Where("username = ?", username).First(&other); err != nil {
		return c.Abort(404, "user not found")
	}

	// Find or create conversation
	conv, err := findOrCreateConversation(userID, other.ID)
	if err != nil {
		return err
	}

	// Mark all received messages as read and notify sender in real time
	unreadCount, _ := database.Table("messages").
		Where("conversation_id = ? AND sender_id = ? AND read = ?", conv.ID, other.ID, false).
		Count()
	if unreadCount > 0 {
		_ = database.Table("messages").
			Where("conversation_id = ? AND sender_id = ? AND read = ?", conv.ID, other.ID, false).
			Update(database.Map{"read": true})
		// Push read-receipt to sender's DM channel
		if ctrl.NotifyFn != nil {
			ctrl.NotifyFn(other.ID, "message_read", map[string]any{
				"conversation_id": conv.ID,
				"read_by":         userID,
			})
		}
	}

	msgs := make([]models.Message, 0)
	if err := database.Table("messages").
		Where("conversation_id = ?", conv.ID).
		OrderBy("created_at ASC").
		Limit(100).
		All(&msgs); err != nil {
		return err
	}

	return c.JSON(200, map[string]any{
		"conversation": conv,
		"messages":     msgs,
		"other_user":   other,
	})
}

// Send sends a message to a user.
// POST /api/messages/:username
func (ctrl *MessageController) Send(c *onihttp.Context) error {
	uid, _ := c.Get("user_id")
	userID, _ := uid.(int64)
	username := c.Param("username")

	var other models.User
	if err := database.Table("users").Where("username = ?", username).First(&other); err != nil {
		return c.Abort(404, "user not found")
	}
	if other.ID == userID {
		return c.Abort(422, "cannot message yourself")
	}

	var req struct {
		Body string `json:"body"`
	}
	if err := c.Bind(&req); err != nil || len(req.Body) == 0 {
		return c.Abort(422, "message body is required")
	}

	conv, err := findOrCreateConversation(userID, other.ID)
	if err != nil {
		return err
	}

	msg := &models.Message{
		ConversationID: conv.ID,
		SenderID:       userID,
		Body:           req.Body,
		Read:           false,
		CreatedAt:      time.Now(),
	}
	if err := database.Table("messages").Insert(msg); err != nil {
		return err
	}

	// Update conversation's last_message_at
	now := time.Now()
	_ = database.Table("conversations").
		Where("id = ?", conv.ID).
		Update(database.Map{"last_message_at": now})

	// Notify recipient over WebSocket
	if ctrl.NotifyFn != nil {
		ctrl.NotifyFn(other.ID, "dm", map[string]any{
			"conversation_id": conv.ID,
			"message":         msg,
			"from_username":   username,
		})
	}

	return c.JSON(201, msg)
}

func findOrCreateConversation(userID, otherID int64) (*models.Conversation, error) {
	// Normalize so user1_id < user2_id to ensure unique constraint works
	u1, u2 := userID, otherID
	if u1 > u2 {
		u1, u2 = u2, u1
	}

	var conv models.Conversation
	err := database.Table("conversations").
		Where("user1_id = ? AND user2_id = ?", u1, u2).
		First(&conv)
	if err == database.ErrNotFound {
		conv = models.Conversation{
			User1ID:   u1,
			User2ID:   u2,
			CreatedAt: time.Now(),
		}
		if insertErr := database.Table("conversations").Insert(&conv); insertErr != nil {
			return nil, insertErr
		}
	} else if err != nil {
		return nil, err
	}
	return &conv, nil
}

func joinIDs(ids []any) string {
	s := ""
	for i, id := range ids {
		if i > 0 {
			s += ","
		}
		s += fmt.Sprintf("%v", id)
	}
	return s
}
