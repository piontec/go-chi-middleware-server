package middleware

// ContextKey is used to identify keys in Context
type ContextKey int

const (
	// CtxUserKey allows to identify user id in the context (string)
	CtxUserKey ContextKey = iota
	// CtxRolesKey allows to identify user's roles in the context ([]string)
	CtxRolesKey
	// CtxIsAdminKey allows to check if user has admin role ([]string)
	CtxIsAdminKey

	// CtxTokenKey allows to get JWT token
	CtxTokenKey = "jwt_token"
	// ClaimUserKey JWT token claim with subject name
	ClaimUserKey = "sub"
)
