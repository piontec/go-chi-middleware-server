package main

import (
	"flag"
	"net/http"
	"runtime"

	"github.com/go-chi/chi"
	"github.com/piontec/go-chi-middleware-server/pkg/server"
	"github.com/sirupsen/logrus"
)

var (
	version = "v0.1.0-dev-build"
	commit  = "unknown"
	date    = "unknown"
	routes  = flag.Bool("routes", false, "Generate router documentation")
)

func printVersion(l *logrus.Logger) {
	l.Infof("failedcloud-apiserver version: %s, commit: %s, build date: %s", version, commit, date)
	l.Infof("failedcloud-apiserver Go Version: %s, OS/Arch: %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

func main() {
	flag.Parse()

	r := server.NewChiServer(func(r *chi.Mux) {
		r.Get("/hello", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("Hello root"))
		})
		r.Get("/public", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("Hello public"))
		})
	}, &server.ChiServerOptions{
		LoggerFields: logrus.Fields{
			"testing": "test",
		},
		HTTPPort: 8080,
		OIDCOptions: server.ChiOIDCMiddlewareOptions{
			Audience:           "http://localhost",
			Issuer:             "https://failedcloud.eu.auth0.com/",
			JwksURL:            "https://failedcloud.eu.auth0.com/.well-known/jwks.json",
			PublicURLsPrefixes: []string{"/pub"},
		},
		ContextSetterOptions: server.ChiContextSetterOptions{
			ClaimToContextKeyMapping: map[string]interface{}{
				"sub": "user",
			},
		},
		// DisableOIDCMiddleware: true,
	})

	printVersion(r.GetLogger())
	r.Run()
}
