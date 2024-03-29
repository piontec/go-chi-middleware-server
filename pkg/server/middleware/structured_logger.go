package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/middleware"
	"github.com/sirupsen/logrus"
)

// adapted from: https://github.com/go-chi/chi/blob/master/_examples/logging/main.go

// LogrusFieldFuncs is a map that sets additional fields in logs (based on keys)
// using a function acting on the http.Request
type LogrusFieldFuncs map[string](func(r *http.Request) string)

// NewStructuredLogger is a simple, but powerful implementation of a custom structured
// logger backed on logrus. I encourage users to copy it, adapt it and make it their
// own. Also take a look at https://github.com/pressly/lg for a dedicated pkg based
// on this work, designed for context-based http routers.
func NewStructuredLogger(logger *logrus.Logger, extraFields logrus.Fields,
	extraFieldFuncs LogrusFieldFuncs) func(next http.Handler) http.Handler {
	return middleware.RequestLogger(&StructuredLogger{
		Logger:          logger,
		ExtraFields:     extraFields,
		ExtraFieldFuncs: extraFieldFuncs,
	})
}

// StructuredLogger implements custom structured middleware logger
type StructuredLogger struct {
	Logger          *logrus.Logger
	ExtraFields     logrus.Fields
	ExtraFieldFuncs LogrusFieldFuncs
}

// NewLogEntry creates new log entry using information from the http.Request
func (l *StructuredLogger) NewLogEntry(r *http.Request) middleware.LogEntry {
	entry := &StructuredLoggerEntry{Logger: logrus.NewEntry(l.Logger)}
	var logFields logrus.Fields
	if l.ExtraFields != nil {
		logFields = l.ExtraFields
	} else {
		logFields = logrus.Fields{}
	}

	// add logfields coming from function calls
	for key, fun := range l.ExtraFieldFuncs {
		logFields[key] = fun(r)
	}

	logFields["ts"] = time.Now().UTC().Format(time.RFC1123)

	if reqID := middleware.GetReqID(r.Context()); reqID != "" {
		logFields["req_id"] = reqID
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	logFields["http_scheme"] = scheme
	logFields["http_proto"] = r.Proto
	logFields["http_method"] = r.Method

	logFields["remote_addr"] = r.RemoteAddr
	logFields["user_agent"] = r.UserAgent()

	logFields["uri"] = fmt.Sprintf("%s://%s%s", scheme, r.Host, r.RequestURI)

	entry.Logger = entry.Logger.WithFields(logFields)

	entry.Logger.Infoln("request started")

	return entry
}

// StructuredLoggerEntry implements single structured log entry
type StructuredLoggerEntry struct {
	Logger logrus.FieldLogger
}

// Write writes end-of-request log message
func (l *StructuredLoggerEntry) Write(status, bytes int, header http.Header, elapsed time.Duration, iface interface{}) {
	l.Logger = l.Logger.WithFields(logrus.Fields{
		"resp_status": status, "resp_bytes_length": bytes,
		"resp_elapsed_ms": float64(elapsed.Nanoseconds()) / 1000000.0,
	})

	l.Logger.Infoln("request complete")
}

// Panic logs a panic
func (l *StructuredLoggerEntry) Panic(v interface{}, stack []byte) {
	l.Logger = l.Logger.WithFields(logrus.Fields{
		"stack": string(stack),
		"panic": fmt.Sprintf("%+v", v),
	})
}

// Helper methods used by the application to get the request-scoped
// logger entry and set additional fields between handlers.
//
// This is a useful pattern to use to set state on the entry as it
// passes through the handler chain, which at any point can be logged
// with a call to .Print(), .Info(), etc.

// GetLogEntry from a request
func GetLogEntry(r *http.Request) logrus.FieldLogger {
	entry := middleware.GetLogEntry(r).(*StructuredLoggerEntry)
	return entry.Logger
}

// LogEntrySetField adds new key and corresponding value to existing log entry
func LogEntrySetField(r *http.Request, key string, value interface{}) {
	if entry, ok := r.Context().Value(middleware.LogEntryCtxKey).(*StructuredLoggerEntry); ok {
		entry.Logger = entry.Logger.WithField(key, value)
	}
}

// LogEntrySetFields adds a map of new key and corresponding value to existing log entry
func LogEntrySetFields(r *http.Request, fields map[string]interface{}) {
	if entry, ok := r.Context().Value(middleware.LogEntryCtxKey).(*StructuredLoggerEntry); ok {
		entry.Logger = entry.Logger.WithFields(fields)
	}
}
