package realtime

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/onipixel/oniworks/framework/memory"
)

const presencePrefix = "oni:presence:"
const presenceTTL = 5 * time.Minute

// PresenceInfo is the data stored per-member in a presence channel.
type PresenceInfo struct {
	UserID int64          `json:"user_id"`
	ConnID string         `json:"conn_id"`
	Meta   map[string]any `json:"meta,omitempty"`
}

func init() {
	// Register so PresenceInfo survives gob encoding over the gossip transport
	// and can be decoded back into a concrete value on remote nodes.
	gob.Register(PresenceInfo{})
}

// toPresenceInfo coerces a stored value into PresenceInfo. Entries written
// locally are concrete PresenceInfo; entries arriving from another node via the
// Redis adapter (JSON) decode as map[string]any. Handling both means remote
// members are counted instead of silently dropped.
func toPresenceInfo(v any) (PresenceInfo, bool) {
	switch t := v.(type) {
	case PresenceInfo:
		return t, true
	case map[string]any:
		b, err := json.Marshal(t)
		if err != nil {
			return PresenceInfo{}, false
		}
		var pi PresenceInfo
		if err := json.Unmarshal(b, &pi); err != nil {
			return PresenceInfo{}, false
		}
		return pi, true
	}
	return PresenceInfo{}, false
}

// PresenceManager manages who is "online" in each channel.
// Presence state is stored in Oni Memory so it is visible to all nodes.
type PresenceManager struct {
	mem *memory.Store
}

func newPresenceManager(mem *memory.Store) *PresenceManager {
	return &PresenceManager{mem: mem}
}

// Join marks a user as present in a channel.
func (pm *PresenceManager) Join(channel string, info PresenceInfo) {
	key := presenceKey(channel, info.ConnID)
	pm.mem.Set(key, info, presenceTTL)
	pm.mem.Publish(fmt.Sprintf("oni:presence.join.%s", channel), info)
}

// Leave removes a user from a presence channel.
func (pm *PresenceManager) Leave(channel string, connID string) {
	key := presenceKey(channel, connID)
	if info, ok := pm.mem.Get(key); ok {
		pm.mem.Delete(key)
		pm.mem.Publish(fmt.Sprintf("oni:presence.leave.%s", channel), info)
	}
}

// Members returns all currently online members of a channel.
func (pm *PresenceManager) Members(channel string) []PresenceInfo {
	pattern := presencePrefix + sanitizeChannel(channel) + ":*"
	keys := pm.mem.Keys(pattern)
	members := make([]PresenceInfo, 0, len(keys))
	for _, k := range keys {
		if v, ok := pm.mem.Get(k); ok {
			if info, ok := toPresenceInfo(v); ok {
				members = append(members, info)
			}
		}
	}
	return members
}

// Count returns the number of online members in a channel.
func (pm *PresenceManager) Count(channel string) int {
	return len(pm.Members(channel))
}

// Refresh extends the TTL for a connection's presence (call periodically).
func (pm *PresenceManager) Refresh(channel, connID string) {
	key := presenceKey(channel, connID)
	pm.mem.Expire(key, presenceTTL)
}

// LeaveAll removes all presence entries for a connection across all channels.
func (pm *PresenceManager) LeaveAll(connID string) {
	keys := pm.mem.Keys(presencePrefix + "*:" + connID)
	for _, k := range keys {
		channel := extractChannel(k)
		if v, ok := pm.mem.Get(k); ok {
			pm.mem.Delete(k)
			pm.mem.Publish(fmt.Sprintf("oni:presence.leave.%s", channel), v)
		}
	}
}

func presenceKey(channel, connID string) string {
	return presencePrefix + sanitizeChannel(channel) + ":" + connID
}

func sanitizeChannel(ch string) string {
	return strings.NewReplacer(".", "_", "/", "_", " ", "_").Replace(ch)
}

func extractChannel(key string) string {
	// key format: "oni:presence:<channel>:<connID>"
	rest := strings.TrimPrefix(key, presencePrefix)
	parts := strings.SplitN(rest, ":", 2)
	if len(parts) > 0 {
		return strings.ReplaceAll(parts[0], "_", ".")
	}
	return ""
}
