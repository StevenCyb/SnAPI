package api

import (
	"log"

	"github.com/StevenCyb/SnAPI/pkg/runtime"
)

// @SnAPI.Middleware
func LoggingMiddleware(req runtime.Request, resp runtime.Response, next runtime.HandlerFunc) {
	log.Printf("call: %s", req.Raw().URL.Path)
	next(req, resp)
}
