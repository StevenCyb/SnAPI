package api

import (
	"net/http"

	"github.com/StevenCyb/SnAPI/pkg/runtime"
	"github.com/StevenCyb/example_proto/pkg/api/model"
)

// @SnAPI.POST("/todo.v1.PingService/Ping")
// @SnAPI.OperationID("todo.v1.PingService.Ping")
// @SnAPI.Tags("PingService")
// @SnAPI.Status(200, "OK")
// @SnAPI.Response(200, "application/json", model.PingResponse)
func PingServicePing(r runtime.Request, w runtime.Response) {
	w.Json(http.StatusOK, model.PingResponse{Message: "pong"})
}
