package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/docgen"
	"github.com/go-chi/render"
	"github.com/sirupsen/logrus"

	msm "github.com/piontec/go-chi-middleware-server/pkg/server/middleware"
)

const (
	defaultHTTPPort                = 8080
	defaultGracefulShutdownTimeSec = 30
)

// ChiServerOptions allows to override default ChiServer options
type ChiServerOptions struct {
	HTTPPort                int
	LoggerFields            logrus.Fields
	LoggerFieldFuncs        msm.LogrusFieldFuncs
	GracefulShutdownTimeSec int
	DisableOIDCMiddleware   bool
	DisableRequestID        bool
	DisableRealIP           bool
	DisableHeartbeat        bool
	DisableURLFormat        bool
	OIDCOptions             ChiOIDCMiddlewareOptions
	ContextSetterOptions    ChiContextSetterOptions
}

// ChiOIDCMiddlewareOptions configures OIDC Middleware
type ChiOIDCMiddlewareOptions struct {
	Audience           string
	Issuer             string
	JwksURL            string
	PublicURLsPrefixes []string
}

// ChiContextSetterOptions configures the ContextSetter Middleware
type ChiContextSetterOptions struct {
	ClaimToContextKeyMapping map[string]interface{}
}

func (o *ChiServerOptions) fillDefaults(logger *logrus.Logger) {
	if o.HTTPPort == 0 {
		o.HTTPPort = defaultHTTPPort
	}
	if o.GracefulShutdownTimeSec == 0 {
		o.GracefulShutdownTimeSec = defaultGracefulShutdownTimeSec
	}
	if o.DisableOIDCMiddleware == false && (o.OIDCOptions.Issuer == "" ||
		o.OIDCOptions.Audience == "") {
		logger.Panicf("OIDC middleware is enabled in server configuration, but no valid configuration was provided.")
	}
}

// ChiServer is an opinionated HTTP server based on go-chi middleware
type ChiServer struct {
	options  *ChiServerOptions
	logger   *logrus.Logger
	mux      *chi.Mux
	started  bool
	listener net.Listener
	stopChan chan interface{}
	server   *http.Server
}

// GetLogger returns a pointer to the logger used by the server
func (s *ChiServer) GetLogger() *logrus.Logger {
	return s.logger
}

// NewChiServer returns a HTTP chi server optionally configured with ChiServerOptions
func NewChiServer(routesRegistrationHandler func(r *chi.Mux), options *ChiServerOptions) *ChiServer {
	// initialize logrus as logger
	logger := logrus.New()
	logger.Formatter = &logrus.JSONFormatter{
		DisableTimestamp: true,
	}

	// if we didn't get any options, initialize with default struct
	if options == nil {
		options = &ChiServerOptions{}
	}
	// initialize default options
	options.fillDefaults(logger)

	r := chi.NewRouter()
	if !options.DisableRequestID {
		r.Use(middleware.RequestID)
	}
	if !options.DisableRealIP {
		r.Use(middleware.RealIP)
	}
	r.Use(msm.NewStructuredLogger(logger, options.LoggerFields, options.LoggerFieldFuncs))
	r.Use(middleware.Recoverer)
	if !options.DisableHeartbeat {
		r.Use(middleware.Heartbeat("/ping"))
	}
	if !options.DisableURLFormat {
		r.Use(middleware.URLFormat)
	}
	r.Use(render.SetContentType(render.ContentTypeJSON))
	if !options.DisableOIDCMiddleware {
		jwtAuth := msm.NewJWTAuthenticator(options.OIDCOptions.Audience, options.OIDCOptions.Issuer, options.OIDCOptions.JwksURL,
			options.OIDCOptions.PublicURLsPrefixes)
		r.Use(jwtAuth.GetHandler())
		r.Use(msm.NewContextSetter(options.ContextSetterOptions.ClaimToContextKeyMapping))
	}

	if routesRegistrationHandler != nil {
		routesRegistrationHandler(r)
	}

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", options.HTTPPort),
		Handler: r,
	}

	return &ChiServer{
		options:  options,
		logger:   logger,
		mux:      r,
		server:   server,
		stopChan: make(chan interface{}, 1),
	}
}

// GetRoutesDocs returns a JSON string describing all the registered routes
func (s *ChiServer) GetRoutesDocs() string {
	return docgen.JSONRoutesDoc(s.mux)
}

// Run starts the listeners, blocks and waits for interruption signal to quit
func (s *ChiServer) Run() {
	s.logger.Infof("Starting HTTP server on port :%d...", s.options.HTTPPort)

	go func() {
		s.logger.Infof("Server started")
		s.started = true
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Panicf("Could not listen on port %d: %v\n", s.options.HTTPPort, err)
		}
		s.stopChan <- ""
	}()

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt)
	select {
	case <-c:
	case <-s.stopChan:
	}

	s.Stop()
}

// Stop stops listening on server ports. Stopped server can't be Run() again.
func (s *ChiServer) Stop() {
	if !s.started {
		return
	}
	s.logger.Infof("Stopping the server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.server.SetKeepAlivesEnabled(false)
	if err := s.server.Shutdown(ctx); err != nil {
		s.logger.Errorf("Error shutting down server: %v", err)
	}
	s.started = false
	s.stopChan <- ""
	s.logger.Infof("Shutdown done")
}

// IsStarted returns true only of Run() was called and listeners are already started
func (s *ChiServer) IsStarted() bool {
	return s.started
}
