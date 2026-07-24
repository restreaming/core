// Package cors validates configured origins.
package cors

import (
	"errors"
	"strings"
)

var DefaultSchemas = []string{"http://", "https://"}

func Validate(origins []string) error {
	for _, origin := range origins {
		if !strings.Contains(origin, "*") && !validateAllowedSchemas(origin) {
			return errors.New("bad origin: origins must contain '*' or include http://, or https://")
		}
	}
	return nil
}

func validateAllowedSchemas(origin string) bool {
	for _, schema := range DefaultSchemas {
		if strings.HasPrefix(origin, schema) {
			return true
		}
	}
	return false
}
