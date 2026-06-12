package docs

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func serveSwaggerJSON(mux *http.ServeMux, endpointPath string, envVar string, defaultFilePath string) {
	specPath := strings.TrimSpace(os.Getenv(envVar))
	if specPath == "" {
		specPath = defaultFilePath
	}

	mux.HandleFunc(endpointPath, func(w http.ResponseWriter, r *http.Request) {
		pathsToTry := []string{specPath}
		if !filepath.IsAbs(specPath) && !strings.HasPrefix(specPath, "../") {
			pathsToTry = append(pathsToTry, filepath.Join("..", specPath))
		}

		var data []byte
		for _, p := range pathsToTry {
			b, err := os.ReadFile(p)
			if err == nil {
				data = b
				break
			}
		}
		if len(data) == 0 {
			http.Error(w, "swagger spec not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write(data)
	})
}

// Register wires up swagger-ui proxy endpoints and swagger spec endpoints.
//
// - /docs/*: swagger-ui assets (proxied to http://swagger-ui:8080)
// - /swagger/*.swagger.json: swagger specs served from local files
func Register(mux *http.ServeMux) {
	serveSwaggerJSON(mux, "/swagger/admin.swagger.json", "GATEWAY_SWAGGER_ADMIN_JSON_PATH", "docs/swagger/admin.swagger.json")
	serveSwaggerJSON(mux, "/swagger/business.swagger.json", "GATEWAY_SWAGGER_BUSINESS_JSON_PATH", "docs/swagger/business.swagger.json")
	serveSwaggerJSON(mux, "/swagger/swap.swagger.json", "GATEWAY_SWAGGER_SWAP_JSON_PATH", "docs/swagger/swap.swagger.json")
	serveSwaggerJSON(mux, "/swagger/chainrpc.swagger.json", "GATEWAY_SWAGGER_CHAINRPC_JSON_PATH", "docs/swagger/chainrpc.swagger.json")

	swaggerUIURL := strings.TrimSpace(os.Getenv("GATEWAY_SWAGGER_UI_URL"))
	if swaggerUIURL == "" {
		// Default for Docker Compose (api-gateway running in the same compose network).
		swaggerUIURL = "http://swagger-ui:8080"
	}

	target, err := url.Parse(swaggerUIURL)
	if err != nil {
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		http.Error(w, "swagger-ui unavailable", http.StatusBadGateway)
	}

	mux.HandleFunc("/docs", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/docs/", http.StatusMovedPermanently)
	})
	mux.Handle("/docs/", http.StripPrefix("/docs", proxy))
}
