package api

import (
	"net/http"

	cfgstore "github.com/datarhei/core/v16/config/store"
	coreapi "github.com/datarhei/core/v16/http/api"

	"github.com/labstack/echo/v4"
)

// ConfigHandler exposes the active engine configuration to compatible clients.
// Configuration writes remain owned by the control plane.
type ConfigHandler struct {
	store cfgstore.Store
}

func NewConfig(store cfgstore.Store) *ConfigHandler {
	return &ConfigHandler{store: store}
}

func (h *ConfigHandler) Get(c echo.Context) error {
	cfg := h.store.GetActive()
	response := coreapi.Config{}
	response.Unmarshal(cfg)

	return c.JSON(http.StatusOK, response)
}
