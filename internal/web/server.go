package web

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"lrss/internal/settings"
)

// Status is the runtime view of the web server for the desktop UI.
type Status struct {
	Running  bool   `json:"running"`
	URL      string `json:"url"`
	LanURL   string `json:"lanUrl,omitempty"`
	Bind     string `json:"bind"`
	Port     int    `json:"port"`
	Error    string `json:"error,omitempty"`
	HasToken bool   `json:"hasToken"`
}

// Server serves the SPA + read API for browser access.
type Server struct {
	mu     sync.Mutex
	http   *http.Server
	cfg    settings.WebAccessConfig
	deps   APIDeps
	assets fs.FS
	status Status
}

// New constructs a stopped server. assets should be frontend/dist (or sub-FS).
func New(deps APIDeps, assets fs.FS) *Server {
	return &Server{deps: deps, assets: assets}
}

// Status returns a copy of the last known status.
func (s *Server) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// Config returns the last applied config (may be zero if never applied).
func (s *Server) Config() settings.WebAccessConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg
}

// Apply starts, restarts, or stops the server to match cfg.
func (s *Server) Apply(ctx context.Context, cfg settings.WebAccessConfig) (Status, error) {
	cfg, _ = cfg.EnsureTokenForLAN()
	cfg = cfg.Normalize()

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.stopLocked(ctx); err != nil {
		s.status.Error = err.Error()
		return s.status, err
	}

	s.cfg = cfg
	if !cfg.Enabled {
		s.status = Status{
			Running:  false,
			Bind:     cfg.Bind,
			Port:     cfg.Port,
			HasToken: cfg.Token != "",
		}
		return s.status, nil
	}

	src := resolveSPA(s.assets)
	log.Printf("web access UI assets: %s", src.desc)
	handler := s.buildHandler(cfg.Token, src)
	addr := listenAddr(cfg.Bind, cfg.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		s.status = Status{
			Running:  false,
			Bind:     cfg.Bind,
			Port:     cfg.Port,
			HasToken: cfg.Token != "",
			Error:    err.Error(),
		}
		return s.status, fmt.Errorf("web listen %s: %w", addr, err)
	}

	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	s.http = srv

	localURL := buildURL("127.0.0.1", cfg.Port, cfg.Token)
	lanURL := ""
	if cfg.Bind == "lan" {
		if ip := firstLANIPv4(); ip != "" {
			lanURL = buildURL(ip, cfg.Port, cfg.Token)
		}
	}
	s.status = Status{
		Running:  true,
		URL:      localURL,
		LanURL:   lanURL,
		Bind:     cfg.Bind,
		Port:     cfg.Port,
		HasToken: cfg.Token != "",
	}

	go func() {
		log.Printf("web access listening on %s (bind=%s)", addr, cfg.Bind)
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("web access serve: %v", err)
			s.mu.Lock()
			s.status.Running = false
			s.status.Error = err.Error()
			s.mu.Unlock()
		}
	}()

	return s.status, nil
}

// Stop shuts down the HTTP server if running.
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopLocked(ctx)
}

func (s *Server) stopLocked(ctx context.Context) error {
	if s.http == nil {
		return nil
	}
	shutdownCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	err := s.http.Shutdown(shutdownCtx)
	s.http = nil
	s.status.Running = false
	s.status.URL = ""
	s.status.LanURL = ""
	s.status.Error = ""
	return err
}

func (s *Server) buildHandler(token string, src spaSource) http.Handler {
	mux := http.NewServeMux()
	s.mountAPI(mux)
	mux.Handle("/", spaHandler(src))
	return withAPIAuth(token, mux)
}

func listenAddr(bind string, port int) string {
	host := "127.0.0.1"
	if bind == "lan" {
		host = "0.0.0.0"
	}
	return fmt.Sprintf("%s:%d", host, port)
}

func buildURL(host string, port int, token string) string {
	u := fmt.Sprintf("http://%s:%d/", host, port)
	if token != "" {
		u += "?token=" + token
	}
	return u
}

func firstLANIPv4() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue
			}
			return ip.String()
		}
	}
	return ""
}


