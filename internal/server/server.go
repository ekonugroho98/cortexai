package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cortexai/cortexai/internal/config"
	"github.com/cortexai/cortexai/internal/service"
	"github.com/rs/zerolog/log"
)

type Server struct {
	cfg   *config.Config
	http  *http.Server
	bqSvc *service.BigQueryService // FIX #7: held for graceful close
}

func New(cfg *config.Config) (*Server, error) {
	s := &Server{cfg: cfg}

	router, bqSvc, err := s.setupRoutes()
	if err != nil {
		return nil, fmt.Errorf("setup routes: %w", err)
	}
	s.bqSvc = bqSvc

	s.http = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return s, nil
}

func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		log.Info().Msg("graceful shutdown initiated")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := s.http.Shutdown(shutdownCtx)

		// FIX #7: close BigQuery client on shutdown
		if s.bqSvc != nil {
			if closeErr := s.bqSvc.Close(); closeErr != nil {
				log.Warn().Err(closeErr).Msg("error closing BigQuery client")
			} else {
				log.Info().Msg("BigQuery client closed")
			}
		}

		return err
	case err := <-errCh:
		return err
	}
}
