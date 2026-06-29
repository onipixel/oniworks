// Package roles provides a simple, performant RBAC (Role-Based Access Control) system.
package roles

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// Permission is a string-based permission identifier (e.g. "users:create", "posts:delete").
type Permission string

// Role groups a set of Permissions under a name.
type Role struct {
	Name        string
	Permissions map[Permission]bool
}

// Manager is the central RBAC registry.
// Roles and their permissions are defined at startup and never change at runtime.
type Manager struct {
	mu    sync.RWMutex
	roles map[string]*Role

	// userRolesFn retrieves the roles for a user ID. You provide this function.
	userRolesFn func(ctx context.Context, userID int64) ([]string, error)
}

// New creates a Manager with a function to resolve user roles.
//
//	roles.New(func(ctx context.Context, userID int64) ([]string, error) {
//	    var r []string
//	    err := db.Table("user_roles").Where("user_id = ?", userID).Pluck("role", &r)
//	    return r, err
//	})
func New(userRolesFn func(ctx context.Context, userID int64) ([]string, error)) *Manager {
	return &Manager{
		roles:       make(map[string]*Role),
		userRolesFn: userRolesFn,
	}
}

// Define registers a role and its permissions.
//
//	m.Define("admin", "users:*", "posts:*")
//	m.Define("editor", "posts:create", "posts:update")
func (m *Manager) Define(name string, permissions ...Permission) *Manager {
	m.mu.Lock()
	defer m.mu.Unlock()
	perms := make(map[Permission]bool, len(permissions))
	for _, p := range permissions {
		perms[p] = true
	}
	m.roles[name] = &Role{Name: name, Permissions: perms}
	return m
}

// HasRole reports whether the user (by ID) has the given role.
func (m *Manager) HasRole(ctx context.Context, userID int64, role string) (bool, error) {
	roles, err := m.userRolesFn(ctx, userID)
	if err != nil {
		return false, err
	}
	for _, r := range roles {
		if r == role {
			return true, nil
		}
	}
	return false, nil
}

// Can reports whether the user can perform the given permission.
// It checks all roles the user has and returns true if any grants the permission.
// Wildcard permissions (e.g. "users:*") match any sub-permission (e.g. "users:create").
func (m *Manager) Can(ctx context.Context, userID int64, permission Permission) (bool, error) {
	userRoles, err := m.userRolesFn(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("roles: resolve user roles: %w", err)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, roleName := range userRoles {
		role, ok := m.roles[roleName]
		if !ok {
			continue
		}
		if matchesPermission(role.Permissions, permission) {
			return true, nil
		}
	}
	return false, nil
}

// Cannot is the inverse of Can.
func (m *Manager) Cannot(ctx context.Context, userID int64, permission Permission) (bool, error) {
	ok, err := m.Can(ctx, userID, permission)
	return !ok, err
}

// AllPermissions returns all permissions granted to the user across all their roles.
func (m *Manager) AllPermissions(ctx context.Context, userID int64) ([]Permission, error) {
	userRoles, err := m.userRolesFn(ctx, userID)
	if err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	seen := make(map[Permission]bool)
	var perms []Permission
	for _, roleName := range userRoles {
		role, ok := m.roles[roleName]
		if !ok {
			continue
		}
		for p := range role.Permissions {
			if !seen[p] {
				seen[p] = true
				perms = append(perms, p)
			}
		}
	}
	return perms, nil
}

// matchesPermission checks if perms contains the given permission, including wildcard support.
// "users:*" matches "users:create", "users:delete", etc.
func matchesPermission(perms map[Permission]bool, target Permission) bool {
	if perms[target] {
		return true
	}
	// Wildcard check: e.g. "users:*" matches "users:create"
	for p := range perms {
		if isWildcardMatch(string(p), string(target)) {
			return true
		}
	}
	return false
}

// isWildcardMatch matches a permission pattern against a target SEGMENT-BY-
// SEGMENT (segments delimited by ":"). Only a full-segment "*" is a wildcard:
//
//   - "*"          matches everything
//   - "users:*"    matches "users:create", "users:posts:edit" (any sub-permission)
//   - "org:*:read"  matches "org:5:read"
//
// A partial token like "user*" is NOT a wildcard — it matches literally and so
// will not match "users:delete". This prevents the prefix-match over-grant where
// "user*" wrongly matched both "users:*" and unrelated "userprofile:*".
func isWildcardMatch(pattern, target string) bool {
	if pattern == "" {
		return false
	}
	if pattern == "*" {
		return true
	}
	if pattern == target {
		return true
	}
	pSegs := strings.Split(pattern, ":")
	tSegs := strings.Split(target, ":")
	for i, ps := range pSegs {
		// A trailing "*" segment matches all remaining (at least one) target segments.
		if ps == "*" && i == len(pSegs)-1 {
			return len(tSegs) > i
		}
		if i >= len(tSegs) {
			return false
		}
		if ps == "*" {
			continue // single-segment wildcard
		}
		if ps != tSegs[i] {
			return false
		}
	}
	// All pattern segments consumed with no trailing wildcard → lengths must match.
	return len(pSegs) == len(tSegs)
}

// Policy is a named gate function for fine-grained authorization logic.
// Policies supplement role-based rules with model-level checks.
type Policy func(ctx context.Context, userID int64, resource any) (bool, error)

// Gate stores named policy functions.
type Gate struct {
	policies map[string]Policy
	mu       sync.RWMutex
}

// NewGate creates a Gate.
func NewGate() *Gate { return &Gate{policies: make(map[string]Policy)} }

// Define registers a policy under a name.
func (g *Gate) Define(name string, policy Policy) {
	g.mu.Lock()
	g.policies[name] = policy
	g.mu.Unlock()
}

// Allows runs the named policy and returns whether it passes.
func (g *Gate) Allows(ctx context.Context, name string, userID int64, resource any) (bool, error) {
	g.mu.RLock()
	policy, ok := g.policies[name]
	g.mu.RUnlock()
	if !ok {
		return false, fmt.Errorf("roles: policy %q not defined", name)
	}
	return policy(ctx, userID, resource)
}
