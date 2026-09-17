package proxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mattwalters/bubble/internal/state"
)

func TestProxyRoutingAndResponses(t *testing.T) {
	// 1. Mock upstream HTTP server
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("response from upstream"))
	}))
	defer upstreamServer.Close()

	upstreamHost := strings.TrimPrefix(upstreamServer.URL, "http://")

	// 2. Set up proxy server
	srv := NewServer(19100)
	srv.routes["pw-142"] = state.Route{
		ID:             "pw-142",
		Upstream:       upstreamHost,
		Dir:            t.TempDir(),
		ComposeProject: "pw-142",
		CreatedAt:      time.Now(),
	}

	// Test Health Endpoint
	reqHealth := httptest.NewRequest("GET", "http://localhost:19100/_bubble/health", nil)
	recHealth := httptest.NewRecorder()
	srv.ServeHTTP(recHealth, reqHealth)
	if recHealth.Code != http.StatusOK || !strings.Contains(recHealth.Body.String(), `"status":"ok"`) {
		t.Errorf("health check failed: code %d, body %s", recHealth.Code, recHealth.Body.String())
	}

	// Test Dashboard Endpoint (localhost without subdomain)
	reqDash := httptest.NewRequest("GET", "http://localhost:19100/", nil)
	recDash := httptest.NewRecorder()
	srv.ServeHTTP(recDash, reqDash)
	if recDash.Code != http.StatusOK || !strings.Contains(recDash.Body.String(), "Bubble Dashboard") {
		t.Errorf("dashboard failed: code %d, body %s", recDash.Code, recDash.Body.String())
	}

	// Test Subdomain Upstream Proxying
	reqSub := httptest.NewRequest("GET", "http://pw-142.localhost:19100/items", nil)
	recSub := httptest.NewRecorder()
	srv.ServeHTTP(recSub, reqSub)
	if recSub.Code != http.StatusOK || recSub.Body.String() != "response from upstream" {
		t.Errorf("upstream proxy failed: code %d, body %s", recSub.Code, recSub.Body.String())
	}

	// Test Landing Page
	reqLanding := httptest.NewRequest("GET", "http://pw-142.localhost:19100/_bubble/", nil)
	recLanding := httptest.NewRecorder()
	srv.ServeHTTP(recLanding, reqLanding)
	if recLanding.Code != http.StatusOK || !strings.Contains(recLanding.Body.String(), "Bubble: pw-142") {
		t.Errorf("landing page failed: code %d, body %s", recLanding.Code, recLanding.Body.String())
	}

	// Test Unknown Subdomain (404)
	reqUnknown := httptest.NewRequest("GET", "http://unknown.localhost:19100/", nil)
	recUnknown := httptest.NewRecorder()
	srv.ServeHTTP(recUnknown, reqUnknown)
	if recUnknown.Code != http.StatusNotFound || !strings.Contains(recUnknown.Body.String(), "404: Unknown Bubble") {
		t.Errorf("unknown bubble 404 failed: code %d, body %s", recUnknown.Code, recUnknown.Body.String())
	}

	// Test Dead Upstream (502)
	srv.routes["dead-bubble"] = state.Route{
		ID:       "dead-bubble",
		Upstream: "127.0.0.1:59999", // closed port
	}
	reqDead := httptest.NewRequest("GET", "http://dead-bubble.localhost:19100/", nil)
	recDead := httptest.NewRecorder()
	srv.ServeHTTP(recDead, reqDead)
	if recDead.Code != http.StatusBadGateway || !strings.Contains(recDead.Body.String(), "bubble doctor dead-bubble --fix") {
		t.Errorf("dead upstream failed: code %d, body %s", recDead.Code, recDead.Body.String())
	}
}

func TestProxyServerLiveHTTP(t *testing.T) {
	// Test full server listener with context cancellation
	srv := NewServer(0) // system allocated port
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Get a free port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	srv.Port = port

	go func() {
		_ = srv.Start(ctx)
	}()

	// Wait for server to accept connections
	var client http.Client
	var resp *http.Response
	for i := 0; i < 20; i++ {
		time.Sleep(50 * time.Millisecond)
		resp, err = client.Get(fmt.Sprintf("http://127.0.0.1:%d/_bubble/health", port))
		if err == nil {
			break
		}
	}
	if err != nil {
		t.Fatalf("failed to connect to live proxy server: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"status":"ok"`) {
		t.Errorf("unexpected body: %s", string(body))
	}
}
