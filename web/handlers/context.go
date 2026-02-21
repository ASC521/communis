package handlers

type contextKey string

const (
	isAuthenticatedContextKey = contextKey("isAuthenticated")
	userIdContextKey          = contextKey("userId")
	isAdminContextKey         = contextKey("isAdmin")
)
