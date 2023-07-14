package internal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/txsvc/stdlib/v2"
)

const (
	// ShutdownDelay is the time to wait for all request, go-routines etc complete
	ShutdownDelay = 10 // seconds
)

var (
	// ErrInvalidConfiguration indicates that parameters used to configure the service were invalid
	ErrInvalidConfiguration = errors.New("invalid configuration")
)

type (
	// RouterFunc creates a mux
	RouterFunc func() *echo.Echo
	// ShutdownFunc is called before the server stops
	ShutdownFunc func(*echo.Echo)

	// Server is an interface for the HTTP server
	Server interface {
		StartBlocking()
		Stop()
	}

	server struct {
		mux              *echo.Echo
		shutdown         ShutdownFunc
		errorHandlerImpl echo.HTTPErrorHandler
	}
)

// New returns a new HTTP server
func NewHttp(router RouterFunc, shutdown ShutdownFunc, errorHandler echo.HTTPErrorHandler) (Server, error) {
	if router == nil || shutdown == nil {
		return nil, ErrInvalidConfiguration
	}

	return &server{
		mux:              router(),
		shutdown:         shutdown,
		errorHandlerImpl: errorHandler,
	}, nil
}

// Stop forces a shutdown
func (s *server) Stop() {
	// all the implementation specific shoutdown code to clean-up
	s.shutdown(s.mux)

	ctx, cancel := context.WithTimeout(context.Background(), ShutdownDelay*time.Second) // FIXME make the delay configurable
	defer cancel()
	if err := s.mux.Shutdown(ctx); err != nil {
		s.mux.Logger.Fatal(err)
	}
}

// StartBlocking starts a new server in the main process
func (s *server) StartBlocking() {

	// setup shutdown handling
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		s.Stop()
	}()

	// add the central error handler
	if s.errorHandlerImpl != nil {
		s.mux.HTTPErrorHandler = s.errorHandlerImpl
	}

	s.mux.HideBanner = true

	// start the server
	port := fmt.Sprintf(":%s", stdlib.GetString("PORT", "8080"))
	s.mux.Logger.Fatal(s.mux.Start(port))
}
