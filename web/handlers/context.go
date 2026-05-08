package handlers

type contextKey string

const (
	isAuthenticatedContextKey = contextKey("isAuthenticated")
	userIDContextKey          = contextKey("userId")
	isAdminContextKey         = contextKey("isAdmin")
	userThemeContextKey       = contextKey("userTheme")
)
