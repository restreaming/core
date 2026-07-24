package http

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/datarhei/core/v16/http/errorhandler"
	"github.com/datarhei/core/v16/http/handler"
	mwiplimit "github.com/datarhei/core/v16/http/middleware/iplimit"
	mwlog "github.com/datarhei/core/v16/http/middleware/log"
	mwsession "github.com/datarhei/core/v16/http/middleware/session"
	"github.com/datarhei/core/v16/log"
	"github.com/datarhei/core/v16/monitor"
	"github.com/datarhei/core/v16/net"
	"github.com/datarhei/core/v16/restream"
	"github.com/datarhei/core/v16/session"

	"github.com/labstack/echo/v4"
)

var ListenAndServe = http.ListenAndServe

type Config struct {
	Logger        log.Logger
	LogBuffer     log.BufferWriter
	Restream      restream.Restreamer
	Metrics       monitor.HistoryReader
	IPLimiter     net.IPLimiter
	Cors          CorsConfig
	Sessions      session.RegistryReader
	ReadOnly      bool
	InternalToken string
}

type CorsConfig struct {
	Origins []string
}

type Server interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

type server struct {
	logger        log.Logger
	restream      restream.Restreamer
	sessions      session.RegistryReader
	internalToken string
	router        *echo.Echo
}

func NewServer(config Config) (Server, error) {
	if config.Restream == nil {
		return nil, fmt.Errorf("restreamer is required")
	}
	if config.Sessions == nil {
		var err error
		config.Sessions, err = session.New(session.Config{})
		if err != nil {
			return nil, err
		}
	}
	if config.Logger == nil {
		config.Logger = log.New("HTTP")
	}

	s := &server{
		logger:        config.Logger,
		restream:      config.Restream,
		sessions:      config.Sessions,
		internalToken: config.InternalToken,
		router:        echo.New(),
	}
	s.router.HTTPErrorHandler = errorhandler.HTTPErrorHandler
	s.router.Use(mwlog.NewWithConfig(mwlog.Config{Logger: s.logger}))
	s.router.Use(recoverMiddleware)
	s.router.Use(mwsession.NewHTTPWithConfig(mwsession.HTTPConfig{
		Collector: config.Sessions.Collector("http"),
	}))

	if config.IPLimiter != nil {
		s.router.Use(mwiplimit.NewWithConfig(mwiplimit.Config{Limiter: config.IPLimiter}))
	}
	s.router.HideBanner = true
	s.router.HidePort = true
	s.router.Logger.SetOutput(newLogwrapper(s.logger))
	s.setRoutes()

	return s, nil
}

func recoverMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) (err error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				err = echo.NewHTTPError(http.StatusInternalServerError, "internal server error").SetInternal(fmt.Errorf("%v\n%s", recovered, debug.Stack()))
			}
		}()
		return next(c)
	}
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *server) setRoutes() {
	engine := s.router.Group("/engine/v1")
	engine.Use(s.internalTokenMiddleware)
	engine.POST("/process/start", s.startProcess)
	engine.POST("/process/stop", s.stopProcess)
	engine.GET("/process/status", s.processStatus)

	s.router.GET("/engine/ping", handler.NewPing().Ping)
}
