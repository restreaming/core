package http

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	coreapp "github.com/datarhei/core/v16/restream/app"
	"github.com/labstack/echo/v4"
)

// internalTokenMiddleware authenticates the private control-plane API without
// allocating or maintaining a session. A constant-time comparison prevents
// token checks from becoming a timing side channel.
func (s *server) internalTokenMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if s.internalToken == "" {
			return echo.NewHTTPError(http.StatusServiceUnavailable, "internal API token is not configured")
		}

		provided := c.Request().Header.Get("X-Internal-Token")
		if provided == "" {
			provided = strings.TrimPrefix(c.Request().Header.Get("Authorization"), "Bearer ")
		}
		if subtle.ConstantTimeCompare([]byte(provided), []byte(s.internalToken)) != 1 {
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid internal API token")
		}

		return next(c)
	}
}

type startProcessRequest struct {
	ID        string          `json:"id"`
	Reference string          `json:"reference,omitempty"`
	Args      []string        `json:"args,omitempty"`
	Config    *coreapp.Config `json:"config,omitempty"`
}

type stopProcessRequest struct {
	ID string `json:"id"`
}

type processStatus struct {
	ID          string  `json:"id"`
	Reference   string  `json:"reference,omitempty"`
	State       string  `json:"state"`
	CPUPercent  float64 `json:"cpu_percent"`
	MemoryBytes uint64  `json:"memory_bytes"`
	FPS         float64 `json:"fps"`
	Bitrate     float64 `json:"bitrate"`
}

type processStatusResponse struct {
	Processes         []processStatus `json:"processes"`
	CPUPercent        float64         `json:"cpu_percent"`
	MemoryBytes       uint64          `json:"memory_bytes"`
	FPS               float64         `json:"fps"`
	Bitrate           float64         `json:"bitrate"`
	ActiveConnections uint64          `json:"active_connections"`
}

func (s *server) startProcess(c echo.Context) error {
	var request startProcessRequest
	if err := json.NewDecoder(c.Request().Body).Decode(&request); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid process payload")
	}
	if request.Config == nil {
		request.Config = &coreapp.Config{ID: request.ID, Reference: request.Reference, Options: request.Args}
	} else {
		if request.Config.ID == "" {
			request.Config.ID = request.ID
		}
		if request.Config.Reference == "" {
			request.Config.Reference = request.Reference
		}
	}
	if request.Config.ID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "process id is required")
	}
	request.Config.Autostart = true

	if err := s.restream.AddProcess(request.Config); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	return c.JSON(http.StatusAccepted, map[string]string{"id": request.Config.ID, "status": "starting"})
}

func (s *server) stopProcess(c echo.Context) error {
	var request stopProcessRequest
	if err := json.NewDecoder(c.Request().Body).Decode(&request); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid process payload")
	}
	if request.ID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "process id is required")
	}
	if err := s.restream.StopProcess(request.ID); err != nil {
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}
	return c.JSON(http.StatusAccepted, map[string]string{"id": request.ID, "status": "stopping"})
}

func (s *server) processStatus(c echo.Context) error {
	ids := s.restream.GetProcessIDs("", "")
	if id := c.QueryParam("id"); id != "" {
		ids = []string{id}
	}
	status := make([]processStatus, 0, len(ids))
	response := processStatusResponse{Processes: status}
	for _, id := range ids {
		state, err := s.restream.GetProcessState(id)
		if err != nil {
			continue
		}
		process := processStatus{ID: id, State: state.State, CPUPercent: state.CPU, MemoryBytes: state.Memory,
			FPS: state.Progress.FPS, Bitrate: state.Progress.Bitrate}
		if p, err := s.restream.GetProcess(id); err == nil {
			process.Reference = p.Reference
		}
		status = append(status, process)
		response.CPUPercent += process.CPUPercent
		response.MemoryBytes += process.MemoryBytes
		response.FPS += process.FPS
		response.Bitrate += process.Bitrate
	}
	if s.sessions != nil {
		for _, collector := range s.sessions.Collectors() {
			response.ActiveConnections += uint64(len(s.sessions.Active(collector)))
		}
	}
	response.Processes = status
	return c.JSON(http.StatusOK, response)
}
