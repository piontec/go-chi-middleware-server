package server_test

import (
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/piontec/go-chi-middleware-server/pkg/server"

	"github.com/go-chi/chi"
	"github.com/stretchr/testify/assert"
)

type testHelper struct {
	options *server.ChiServerOptions
	server  *server.ChiServer
	client  *http.Client
}

func getTestHelper(options *server.ChiServerOptions) *testHelper {

	server := server.NewChiServer(func(r *chi.Mux) {
		r.Get("/hello", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("Hello root"))
		})
	}, options)
	go func() {
		server.Run()
	}()
	for {
		if server.IsStarted() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	return &testHelper{
		options: options,
		server:  server,
		client:  &http.Client{},
	}
}

func (th *testHelper) cleanup() {
	th.server.Stop()
}

func TestHealthcheck(t *testing.T) {
	h := getTestHelper(&server.ChiServerOptions{
		HTTPPort:              8080,
		DisableOIDCMiddleware: true,
	})
	defer h.cleanup()

	time.Sleep(100 * time.Millisecond)
	resp, err := h.client.Get("http://localhost:8080/ping")
	if err != nil {
		t.Fatalf("Server did not respond: %v", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	assert.Nil(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, ".", string(body))
}
