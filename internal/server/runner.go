package server

import (
	"context"
	"go.uber.org/zap"
)

// Run runs the given server.
func Run(s Server) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.logger.Debug("Listening...", zap.String("address", s.listener.Addr().String()))
	if err := s.Run(ctx); err != nil {
		return err
	}
	return s.Shutdown(ctx)
}
