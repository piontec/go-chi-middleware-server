package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/form3tech-oss/jwt-go"
	"github.com/go-chi/render"
)

// NewContextSetter returns instance of UserInfoSetter middleware
// UserInfoSetter is a middleware, which sets user name, roles and admin flags based on
// JWT claims
func NewContextSetter(claimToContextKeyMapping map[string]interface{}) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			token, ok := r.Context().Value(CtxJWTKey).(*jwt.Token)
			// if there's no JWT token or no mapping configured, move to the next middleware
			if !ok || token == nil || len(claimToContextKeyMapping) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok || claims == nil {
				render.Render(w, r, ErrAuth(errors.New("claims not found in auth token in Context()")))
				return
			}

			ctx := r.Context()
			for claimKey, contextKey := range claimToContextKeyMapping {
				claim, found := claims[claimKey]
				if !found {
					render.Render(w, r, ErrAuth(fmt.Errorf("%s claim not found in claims", claimKey)))
					return
				}

				ctx = context.WithValue(ctx, contextKey, claim)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		}
		return http.HandlerFunc(fn)
	}
}
