package api

import (
	"log"

	"github.com/StevenCyb/SnAPI/pkg/runtime"
)

// @SnAPI.Middleware
func LoggingMiddleware(r runtime.Request, w runtime.Response, next runtime.HandlerFunc) {
	log.Printf("Received %s request for %s", r.Raw().Method, r.Raw().URL.Path)
	next(r, w)
}
