package proxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/mattwalters/bubble/internal/state"
)

// Server is the HTTP and WebSocket reverse proxy.
type Server struct {
	Port     int
	mu       sync.RWMutex
	routes   map[string]state.Route
	watcher  *fsnotify.Watcher
	httpSrv  *http.Server
	shutdown chan struct{}
}

// NewServer creates a new proxy server instance.
func NewServer(port int) *Server {
	if port <= 0 {
		port = 19100
	}
	return &Server{
		Port:     port,
		routes:   make(map[string]state.Route),
		shutdown: make(chan struct{}),
	}
}

// ReloadRoutes reads all ~/.bubble/routes/*.json files into memory.
func (s *Server) ReloadRoutes() error {
	routes, err := state.ListRoutes()
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	newRoutes := make(map[string]state.Route)
	for _, r := range routes {
		newRoutes[r.ID] = r
	}
	s.routes = newRoutes
	return nil
}

func (s *Server) getRoute(id string) (state.Route, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.routes[id]
	return r, ok
}

func (s *Server) getAllRoutes() []state.Route {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]state.Route, 0, len(s.routes))
	for _, r := range s.routes {
		list = append(list, r)
	}
	return list
}

// StartWatcher starts the fsnotify watcher on ~/.bubble/routes/.
func (s *Server) StartWatcher(ctx context.Context) error {
	routesDir := state.RoutesDir()
	_ = os.MkdirAll(routesDir, 0755)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("creating fsnotify watcher: %w", err)
	}
	s.watcher = watcher

	if err := watcher.Add(routesDir); err != nil {
		_ = watcher.Close()
		return fmt.Errorf("watching routes dir %s: %w", routesDir, err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				_ = watcher.Close()
				return
			case <-s.shutdown:
				_ = watcher.Close()
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if filepath.Ext(event.Name) == ".json" {
					_ = s.ReloadRoutes()
				}
			case _, ok := <-watcher.Errors:
				if !ok {
					return
				}
			}
		}
	}()

	return nil
}

// ServeHTTP handles routing for all incoming HTTP/WebSocket requests.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if h, _, err := net.SplitHostPort(r.Host); err == nil {
		host = h
	}

	// 1. Health check endpoint
	if r.URL.Path == "/_bubble/health" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
		return
	}

	// 2. Exact localhost or 127.0.0.1 without subdomain -> Root Dashboard
	if host == "localhost" || host == "127.0.0.1" {
		renderDashboard(w, s.getAllRoutes(), s.Port)
		return
	}

	// 3. Extract subdomain from <id>.localhost
	subdomain := ""
	if strings.HasSuffix(host, ".localhost") {
		subdomain = strings.TrimSuffix(host, ".localhost")
	} else if parts := strings.Split(host, "."); len(parts) > 1 {
		subdomain = parts[0]
	}

	if subdomain == "" {
		renderDashboard(w, s.getAllRoutes(), s.Port)
		return
	}

	route, exists := s.getRoute(subdomain)
	if !exists {
		renderNotFound(w, subdomain, s.getAllRoutes(), s.Port)
		return
	}

	// 4. Special route: <id>.localhost/_bubble/ -> landing page
	if strings.HasPrefix(r.URL.Path, "/_bubble/") || r.URL.Path == "/_bubble" {
		var ports map[string]int
		tier := "unknown"
		if st, err := state.ReadWorktreeState(route.Dir); err == nil {
			ports = st.DiscoveredPorts
			tier = st.Tier
		}
		renderLanding(w, route.ID, route.Dir, tier, route.Upstream, s.Port, ports)
		return
	}

	// 5. If route has no upstream configured
	if route.Upstream == "" {
		renderLanding(w, route.ID, route.Dir, "compose-only", "", s.Port, nil)
		return
	}

	// 6. Check for WebSocket Upgrade
	if isWebSocketUpgrade(r) {
		s.proxyWebSocket(w, r, route)
		return
	}

	// 7. Standard HTTP reverse proxy
	targetURL, err := url.Parse("http://" + route.Upstream)
	if err != nil {
		http.Error(w, "Bad upstream configuration", http.StatusInternalServerError)
		return
	}

	rp := httputil.NewSingleHostReverseProxy(targetURL)
	rp.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
		renderDeadUpstream(rw, route.ID, route.Upstream)
	}

	// Update request headers
	r.Header.Set("X-Forwarded-Host", r.Host)
	r.Header.Set("X-Bubble-ID", route.ID)

	rp.ServeHTTP(w, r)
}

func isWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket") ||
		strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade")
}

func (s *Server) proxyWebSocket(w http.ResponseWriter, r *http.Request, route state.Route) {
	upstreamConn, err := net.DialTimeout("tcp", route.Upstream, 5*time.Second)
	if err != nil {
		renderDeadUpstream(w, route.ID, route.Upstream)
		return
	}
	defer upstreamConn.Close()

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Websocket hijacking not supported", http.StatusInternalServerError)
		return
	}

	clientConn, clientBuf, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, fmt.Sprintf("hijack error: %v", err), http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()

	// Write original request line and headers to upstream
	if err := r.Write(upstreamConn); err != nil {
		return
	}

	errChan := make(chan error, 2)
	go func() {
		// client -> upstream
		var reader io.Reader = clientConn
		if clientBuf.Reader.Buffered() > 0 {
			reader = io.MultiReader(clientBuf, clientConn)
		}
		_, err := io.Copy(upstreamConn, reader)
		errChan <- err
	}()

	go func() {
		// upstream -> client
		_, err := io.Copy(clientConn, upstreamConn)
		errChan <- err
	}()

	<-errChan
}

// Start runs the HTTP proxy server on s.Port.
func (s *Server) Start(ctx context.Context) error {
	if err := s.ReloadRoutes(); err != nil {
		return err
	}
	if err := s.StartWatcher(ctx); err != nil {
		return err
	}

	addr := fmt.Sprintf("127.0.0.1:%d", s.Port)
	s.httpSrv = &http.Server{
		Addr:    addr,
		Handler: s,
	}

	go func() {
		<-ctx.Done()
		_ = s.Stop()
	}()

	err := s.httpSrv.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// Stop stops the HTTP server.
func (s *Server) Stop() error {
	close(s.shutdown)
	if s.httpSrv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return s.httpSrv.Shutdown(ctx)
	}
	return nil
}
