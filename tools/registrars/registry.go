package registrars

import (
	"strings"

	"github.com/fenilmodi00/ipo-backend/tools/registrars/bigshare"
	"github.com/fenilmodi00/ipo-backend/tools/registrars/kfin"
	"github.com/fenilmodi00/ipo-backend/tools/registrars/mufg"
)

var clients map[string]RegistrarClient

func init() {
	clients = map[string]RegistrarClient{
		"BIGSHARE": bigshare.NewClient(),
		"KFIN":     kfin.NewClient(),
		"MUFG":     mufg.NewClient(),
	}
}

// GetClient returns a client for the specific registrar ID, or nil if not found.
func GetClient(registrarID string) RegistrarClient {
	if registrarID == "" {
		return nil
	}
	// Attempt exact match
	if client, ok := clients[strings.ToUpper(registrarID)]; ok {
		return client
	}

	// Fallback fuzzy match
	name := strings.ToUpper(strings.TrimSpace(registrarID))
	switch {
	case strings.Contains(name, "BIGSHARE"):
		return clients["BIGSHARE"]
	case strings.Contains(name, "MUFG") || strings.Contains(name, "INTIME") || strings.Contains(name, "LINK"):
		return clients["MUFG"]
	case strings.Contains(name, "KFIN") || strings.Contains(name, "KFINTECH"):
		return clients["KFIN"]
	}

	return nil
}
