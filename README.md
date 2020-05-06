# go chi HTTP middleware server

This project provides a ready to use, opinionated server stub that can be used to serve HTTP APIs using the great [chi](https://github.com/go-chi/chi) project.

It is entirely based on the work done by the chi project and relevant middleware projects.

The stack includes the following:

- no needed configuration (sane defaults), but configuration options are available if needed
- ability to easily register your routes and paths with chi router
- structured logging based on [logrus](https://github.com/sirupsen/logrus)
- implementation of the `/ping` health checking endpoint
- automatic panic recovery
- authentication support for OIDC compliant providers, with an option to configure JWT claims to `Context()` keys

## The same, but for gRPC

I created a similar starter setup for gRPC servers [over here](https://github.com/piontec/grpc-middleware-server).

## Examples

Writing a simple server, that just offers the health check `/ping` endpoint, has recovery enabled, skips OIDC (as it needs a separate setup) and serves a very simple `/hello` path is as simple as:

```go
package main

import (
    "net/http"

    "github.com/go-chi/chi"
    "github.com/piontec/go-chi-middleware-server/pkg/server"
)

func main() {
    r := server.NewChiServer(func(r *chi.Mux) {
        r.Get("/hello", func(w http.ResponseWriter, r *http.Request) {
            w.Write([]byte("Hello world"))
        })
    }, &server.ChiServerOptions{
        HTTPPort: 8080,
        DisableOIDCMiddleware: true,
    })
    r.Run()
}
```

As you can see in the example above, to register your own paths with chi's router, you have a function that offers you access to the router object. You can learn more from [chi's docs](https://github.com/go-chi/chi#router-design).

Full code examples for using go-chi-middleware-server can be found in [server_test.go](./pkg/server/server_test.go).

## Configuration

The second argument to `NewChiServer()` is the `ChiServerOptions` struct. You can use it like that:

```go
r := server.NewChiServer(func(r *chi.Mux) {
    r.Get("/hello", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("Hello world"))
    })
}, &server.ChiServerOptions{
    HTTPPort: 8080, // TCP port to listen on; 8080 is the default
    // normally, all middlewares are by default enabled; you have to explicitly disable them
    DisableOIDCMiddleware: true, // disable the OIDC authentication middleware; disables the
                                 // disables the related ContextSetter as well - see below
    DisableRequestID: true, // disables the request tracking middleware: https://github.com/go-chi/chi#core-middlewares
    DisableRealIP: true, // disables the real IP middleware: https://github.com/go-chi/chi#core-middlewares
    DisableHeartbeat: true, // disables the `/ping` health checking endpoint
    DisableURLFormat: true, // disables URL formatting middleware: https://github.com/go-chi/chi#core-middlewares
    LoggerFields: logrus.Fields{ // optional; this configures logging to include all "key": "value" pairs in each log message
        "testing": "test",
    },
    LoggerFieldFuncs        msm.LogrusFieldFuncs{ // optional; allows to use additional field based on function call in log entries
        "url": func(r *http.Request) string {
            return r.URL
        },
    },
    OIDCOptions: server.ChiOIDCMiddlewareOptions{ // provide only when OIDC middleware is enabled (default setting)
        Audience:           "http://localhost", // audience claim expected in the JWT token
        Issuer:             "https://your-oidc-provider.com/", // issuer claim expected in the JWT token
        JwksURL:            "https://your-oidc-provider.com/.well-known/jwks.json", // URL to the JWKS document of your provider
        PublicURLsPrefixes: []string{"/pub"}, // optional; all your registered paths starting with any of the prefixes listed
                                              // here are not checked for OIDC authentication and available publicly
    },
    ContextSetterOptions: server.ChiContextSetterOptions{ // optional; possible only when OIDC middleware is enabled (default setting)
        ClaimToContextKeyMapping: map[string]interface{}{ // a map that shows which claims should available in request.Context()
            "sub": "user", // this will put the value of "sub" claim of the JWT token into Context() under the "user" key
        },
    },
})
```
