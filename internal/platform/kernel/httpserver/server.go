package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Server struct {
	name   string
	addr   string
	logger *slog.Logger
	mux    *http.ServeMux
}

func New(name string, addr string, logger *slog.Logger) *Server {
	mux := http.NewServeMux()
	server := &Server{
		name:   name,
		addr:   addr,
		logger: logger,
		mux:    mux,
	}
	server.registerHealth()

	return server
}

func (s *Server) Mux() *http.ServeMux {
	return s.mux
}

func (s *Server) Run(ctx context.Context) error {
	httpServer := &http.Server{
		Addr:              s.addr,
		Handler:           s.mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("http server starting", "addr", s.addr)
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return err
		}
		s.logger.Info("http server stopped")
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func SignalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

func AddrFromEnv(envKey string, fallback string) string {
	if value := os.Getenv(envKey); value != "" {
		return value
	}

	return fallback
}

func (s *Server) registerHealth() {
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{
			"service": s.name,
			"status":  "ok",
		})
	})
}

func WriteJSON(w http.ResponseWriter, statusCode int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(value)
}

func WriteError(w http.ResponseWriter, err error) {
	statusCode := http.StatusInternalServerError
	code := string(apperrors.CodeInternal)
	message := "internal error"

	var appErr *apperrors.Error
	if errors.As(err, &appErr) {
		code = string(appErr.Code)
		message = appErr.Message
		statusCode = statusFromCode(appErr.Code)
	}

	WriteJSON(w, statusCode, map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

func statusFromCode(code apperrors.Code) int {
	switch code {
	case apperrors.CodeInvalidArgument:
		return http.StatusBadRequest
	case apperrors.CodeNotFound:
		return http.StatusNotFound
	case apperrors.CodeConflict:
		return http.StatusConflict
	case apperrors.CodeUnauthorized:
		return http.StatusUnauthorized
	case apperrors.CodeForbidden:
		return http.StatusForbidden
	case apperrors.CodeUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}
