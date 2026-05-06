package controllers

import (
	"regexp"
	"strings"
	"time"

	"onigram/app/models"

	"github.com/onipixel/oniworks/framework/database"
)

var mentionRe = regexp.MustCompile(`@(\w+)`)

// notifyMentions extracts @username mentions from text and creates a mention
// notification for each mentioned user that is not the actor.
func notifyMentions(actorID int64, postID *int64, text string, notifyFn func(*models.Notification)) {
	matches := mentionRe.FindAllStringSubmatch(strings.ToLower(text), -1)
	seen := map[string]bool{}
	for _, m := range matches {
		username := m[1]
		if seen[username] {
			continue
		}
		seen[username] = true

		var target models.User
		if err := database.Table("users").
			Select("id").
			Where("username = ?", username).
			First(&target); err != nil {
			continue
		}
		if target.ID == actorID {
			continue
		}

		// Skip if already notified for this post (avoid duplicate mention notifs on edit)
		if postID != nil {
			already, _ := database.Table("notifications").
				Where("user_id = ? AND actor_id = ? AND type = ? AND post_id = ?",
					target.ID, actorID, models.NotifMention, *postID).
				Exists()
			if already {
				continue
			}
		}

		notif := &models.Notification{
			UserID:    target.ID,
			ActorID:   actorID,
			Type:      models.NotifMention,
			PostID:    postID,
			Read:      false,
			CreatedAt: time.Now(),
		}
		if err := database.Table("notifications").Insert(notif); err == nil && notifyFn != nil {
			notifyFn(notif)
		}
	}
}
