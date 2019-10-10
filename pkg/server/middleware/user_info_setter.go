package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/dgrijalva/jwt-go"
	"github.com/go-chi/render"
)

const (
	// DefaultUserRole as a string
	DefaultUserRole = "user"
	// AdminUserRole as a string
	AdminUserRole = "admin"
)

// NewUserInfoSetter returns instance of UserInfoSetter middleware
// UserInfoSetter is a middleware, which sets user name, roles and admin flags based on
// JWT claims
func NewUserInfoSetter(ctxTokenKey, claimUserKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			token, ok := r.Context().Value(ctxTokenKey).(*jwt.Token)
			if !ok || token == nil {
				next.ServeHTTP(w, r)
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok || claims == nil {
				render.Render(w, r, ErrAuth(errors.New("claims not found in auth token in Context()")))
				return
			}

			sub, found := claims[claimUserKey]
			if !found {
				render.Render(w, r, ErrAuth(errors.New("user subject not found in claims")))
				return
			}

			ctx := context.WithValue(r.Context(), CtxUserKey, sub)

			next.ServeHTTP(w, r.WithContext(ctx))
		}
		return http.HandlerFunc(fn)
	}
}
