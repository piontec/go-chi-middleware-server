package middleware

import (
	"bytes"
	"crypto/rsa"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"

	jwtmiddleware "github.com/auth0/go-jwt-middleware"
	"github.com/dgrijalva/jwt-go"
)

const (
	// CtxJWTKey allows to get JWT token
	CtxJWTKey = "jwt_token"
	// ClaimUserKey JWT token claim with subject name
	// ClaimUserKey = "sub"
)

type jwks struct {
	Keys []jsonWebKey `json:"keys"`
}

type jsonWebKey struct {
	Kty string   `json:"kty"`
	Kid string   `json:"kid"`
	Use string   `json:"use"`
	Alg string   `json:"alg"`
	N   string   `json:"n"`
	E   string   `json:"e"`
	X5c []string `json:"x5c,omitempty"`
}

// JwtAuthenticator is a middleware for validating JWT auth tokens
type JwtAuthenticator struct {
	audience       string
	issuer         string
	jwksURL        string
	publicPrefixes []string
	loader         *JwksKeyLoader
}

// NewJWTAuthenticator returns a new authenticator for the given audience and issuer values
// expected in JWT tokens
func NewJWTAuthenticator(audience, issuer, jwksURL string, publicURLPrefixes []string) *JwtAuthenticator {
	return &JwtAuthenticator{
		audience:       audience,
		issuer:         issuer,
		jwksURL:        jwksURL,
		publicPrefixes: publicURLPrefixes,
		loader:         NewJwksKeyLoader(jwksURL),
	}
}

func (a *JwtAuthenticator) getRSAPublicKeyByID(keyID string) (*rsa.PublicKey, error) {
	keyCopy, err := a.loader.GetPublicKey(keyID)
	// key loading can fail because of cert expiry and renewal; try to reload in that case
	if err != nil {
		a.loader.Reload()
		keyCopy, reloadErr := a.loader.GetPublicKey(keyID)
		if reloadErr != nil {
			return nil, fmt.Errorf("can't load public key for JWT validation: %v", reloadErr)
		}
		return keyCopy, nil
	}

	return keyCopy, nil
}

// GetHandler returns new middleware handler
func (a *JwtAuthenticator) GetHandler() func(next http.Handler) http.Handler {
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
			// Load required RSA public key
			keyID := token.Header["kid"].(string)
			key, err := a.getRSAPublicKeyByID(keyID)
			if err != nil {
				return token, err
			}
			return key, err
		},
		SigningMethod: jwt.SigningMethodRS256,
		UserProperty:  CtxJWTKey,
	})

	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			// check if this is a public path that requires no authentication
			isPublic := false
			for _, prefix := range a.publicPrefixes {
				urlPath := r.URL.EscapedPath()
				if strings.HasPrefix(urlPath, prefix) {
					isPublic = true
					break
				}
			}
			if isPublic { // if this URL is public, skip auth path
				next.ServeHTTP(w, r)
			} else {
				jwtMiddleware.Handler(next).ServeHTTP(w, r)
			}
		}
		return http.HandlerFunc(fn)
	}
}

// JwksKeyLoader lazily loads and caches JWK certificate, but allows for forced reload
type JwksKeyLoader struct {
	certLock sync.RWMutex
	pubKey   *rsa.PublicKey
	once     *sync.Once
	jwksURL  string
}

// NewJwksKeyLoader returns new JwkCertLoader
func NewJwksKeyLoader(jwksURL string) *JwksKeyLoader {
	return &JwksKeyLoader{
		jwksURL: jwksURL,
		once:    &sync.Once{},
	}
}

// GetPublicKey loads the cert from the online JWKS if not yet loaded
// otherwise returns cached version
func (l *JwksKeyLoader) GetPublicKey(keyID string) (*rsa.PublicKey, error) {
	var doErr error
	l.once.Do(func() {
		var pubKey *rsa.PublicKey
		resp, err := http.Get(l.jwksURL)

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
				if len(keys.Keys[k].X5c) > 0 {
					newCert := "-----BEGIN CERTIFICATE-----\n" + keys.Keys[k].X5c[0] + "\n-----END CERTIFICATE-----"
					pubKey, err = jwt.ParseRSAPublicKeyFromPEM([]byte(newCert))
				} else {
					pubKey, err = l.loadKeysFromComponents(keys.Keys[k])
				}
			}
		}

		if pubKey == nil {
			doErr = errors.New("unable to find appropriate key")
			return
		}

		l.certLock.Lock()
		l.pubKey = pubKey
		l.certLock.Unlock()
		return
	})
	if doErr != nil {
		return nil, doErr
	}
	l.certLock.RLock()
	defer l.certLock.RUnlock()
	return l.pubKey, nil
}

// adapted from https://stackoverflow.com/questions/25179492/create-public-key-from-modulus-and-exponent-in-golang
func (l *JwksKeyLoader) loadKeysFromComponents(key jsonWebKey) (*rsa.PublicKey, error) {
	decN, err := base64.RawURLEncoding.DecodeString(key.N)
	if err != nil {
		return nil, err
	}
	n := big.NewInt(0)
	n.SetBytes(decN)

	decE, err := base64.RawURLEncoding.DecodeString(key.E)
	if err != nil {
		return nil, err
	}
	var eBytes []byte
	if len(decE) < 8 {
		eBytes = make([]byte, 8-len(decE), 8)
		eBytes = append(eBytes, decE...)
	} else {
		eBytes = decE
	}
	eReader := bytes.NewReader(eBytes)
	var e uint64
	err = binary.Read(eReader, binary.BigEndian, &e)
	if err != nil {
		return nil, err
	}
	pKey := &rsa.PublicKey{N: n, E: int(e)}

	return pKey, nil
}

// Reload force the certificate to be reloaded from the source on the next GetCert() call
func (l *JwksKeyLoader) Reload() {
	l.once = &sync.Once{}
}
