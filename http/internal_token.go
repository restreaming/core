package http

import (
	"crypto/subtle"
	"strings"

	"github.com/labstack/echo/v4"
)

// internalTokenMiddleware protects engine control-plane routes. An empty token
// disables the check, which keeps local development deployments secret-free.
func internalTokenMiddleware(expected string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			token := c.Request().Header.Get("X-Internal-Token")
			if token == "" {
				authorization := c.Request().Header.Get("Authorization")
				const bearer = "Bearer "
				if strings.HasPrefix(authorization, bearer) {
					token = strings.TrimSpace(strings.TrimPrefix(authorization, bearer))
				}
			}

			if subtle.ConstantTimeCompare([]byte(token), []byte(expected)) != 1 {
				return echo.NewHTTPError(echo.ErrUnauthorized.Code, "unauthorized")
			}

			return next(c)
		}
	}
}
