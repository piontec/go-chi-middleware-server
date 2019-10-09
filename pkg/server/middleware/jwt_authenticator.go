package middleware

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"

	jwtmiddleware "github.com/auth0/go-jwt-middleware"
	"github.com/dgrijalva/jwt-go"
)

type jwks struct {
	Keys []jsonWebKeys `json:"keys"`
}

type jsonWebKeys struct {
	Kty string   `json:"kty"`
	Kid string   `json:"kid"`
	Use string   `json:"use"`
	N   string   `json:"n"`
	E   string   `json:"e"`
	X5c []string `json:"x5c"`
}

// JwtAuthenticator is a middleware for validating JWT auth tokens
type JwtAuthenticator struct {
	certLock sync.RWMutex
	cert     string
	audience string
	issuer   string
	loader   *JwkCertLoader
}

// NewJWTAuthenticator returns a new authenticator for the given audience and issuer values
// expected in JWT tokens
func NewJWTAuthenticator(audience, issuer string) *JwtAuthenticator {
	return &JwtAuthenticator{
		audience: audience,
		issuer:   issuer,
		loader:   NewJwkCertLoader(issuer),
	}
}

// GetHandler returns new middleware handler
func (a *JwtAuthenticator) GetHandler() func(next http.Handler) http.Handler {
	// !FIXME: we need to check for no-auth URL prefixes here!
	// !FIXME: extrac validationKeyGetter so it can be replaced for test
	jwtMiddleware := jwtmiddleware.New(jwtmiddleware.Options{
		ValidationKeyGetter: func(token *jwt.Token) (interface{}, error) {
			// Verify 'aud' claim
			checkAud := token.Claims.(jwt.MapClaims).VerifyAudience(a.audience, false)
			if !checkAud {
				return token, errors.New("invalid audience")
			}
			// Verify 'iss' claim
			checkIss := token.Claims.(jwt.MapClaims).VerifyIssuer(a.issuer, false)
			if !checkIss {
				return token, errors.New("invalid issuer")
			}

			keyID := token.Header["kid"].(string)
			certCopy, err := a.loader.GetCert(keyID)
			if err != nil {
				return token, err
			}

			result, err := jwt.ParseRSAPublicKeyFromPEM([]byte(certCopy))
			if err != nil {
				a.loader.Reload()
				certCopy, reloadErr := a.loader.GetCert(keyID)
				if reloadErr != nil {
					return result, fmt.Errorf("can't load public certificate for JWT validation: %v", reloadErr)
				}
				return jwt.ParseRSAPublicKeyFromPEM([]byte(certCopy))
			}

			return result, err
		},
		SigningMethod: jwt.SigningMethodRS256,
		UserProperty:  CtxTokenKey,
	})
	return jwtMiddleware.Handler
}

// JwkCertLoader lazily loads and caches JWK certificate, but allows for forced reload
type JwkCertLoader struct {
	certLock sync.RWMutex
	cert     string
	once     *sync.Once
	domain   string
}

// NewJwkCertLoader returns new JwkCertLoader
func NewJwkCertLoader(domain string) *JwkCertLoader {
	return &JwkCertLoader{
		domain: domain,
		once:   &sync.Once{},
	}
}

// GetCert loads the cert from the Internet if not available, else returns cached cert
func (l *JwkCertLoader) GetCert(keyID string) (string, error) {
	var doErr error
	l.once.Do(func() {
		newCert := ""
		certURL := fmt.Sprintf("%s.well-known/jwks.json", l.domain)
		resp, err := http.Get(certURL)

		if err != nil {
			doErr = err
			return
		}
		defer resp.Body.Close()

		var keys = jwks{}
		err = json.NewDecoder(resp.Body).Decode(&keys)

		if err != nil {
			doErr = err
			return
		}

		for k := range keys.Keys {
			if keyID == keys.Keys[k].Kid {
				newCert = "-----BEGIN CERTIFICATE-----\n" + keys.Keys[k].X5c[0] + "\n-----END CERTIFICATE-----"
			}
		}

		if newCert == "" {
			doErr = errors.New("unable to find appropriate key")
			return
		}

		l.certLock.Lock()
		l.cert = newCert
		l.certLock.Unlock()
		return
	})
	if doErr != nil {
		return "", doErr
	}
	l.certLock.RLock()
	defer l.certLock.RUnlock()
	return l.cert, nil
}

// Reload force the certificate to be reloaded from the source on the next GetCert() call
func (l *JwkCertLoader) Reload() {
	l.once = &sync.Once{}
}
