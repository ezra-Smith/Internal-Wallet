package selfhosted

import (
	"strings"

	"internalwallet/proto/pb"
)

// Config is self-hosted node configuration.
// Type is one of: "ethereum", "bsc", "tron".
type Config struct {
	Endpoint string
	Name     string
	Chain    pb.BlockChainType
	ChainID  int64
	APIKey   string
	Type     string
	Weight   float64
}

func (c *Config) endpoint() string {
	base := strings.TrimRight(c.Endpoint, "/")
	switch c.Type {
	case "ethereum":
		if strings.HasSuffix(base, "/eth") {
			return base
		}
		return base + "/eth"
	case "bsc":
		if strings.HasSuffix(base, "/bsc") {
			return base
		}
		return base + "/bsc"
	case "tron":
		if strings.HasSuffix(base, "/tron") {
			return base
		}
		return base + "/tron"
	default:
		return base
	}
}
