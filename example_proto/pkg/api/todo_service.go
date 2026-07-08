package api

import (
	"fmt"
	"net/http"

	"github.com/StevenCyb/SnAPI/pkg/runtime"
	"github.com/StevenCyb/example_proto/pkg/api/model"
)

type TodoService struct{}

// @SnAPI.GET("/v1/todos/{id}")
// @SnAPI.OperationID("todo.v1.TodoService.GetTodo")
// @SnAPI.Tags("TodoService")
// @SnAPI.Path("id", "string")
// @SnAPI.Status(200, "OK")
// @SnAPI.Status(404, "todo not found")
// @SnAPI.Response(200, "application/json", model.Todo)
func (s *TodoService) GetTodo(r runtime.Request, w runtime.Response) {
	id := r.PathValue("id")
	todo, ok := todos.get(id)
	if !ok {
		w.Error(http.StatusNotFound, fmt.Sprintf("todo %q not found", id))
		return
	}
	w.Json(http.StatusOK, todo)
}

// @SnAPI.GET("/v1/todos")
// @SnAPI.OperationID("todo.v1.TodoService.ListTodos")
// @SnAPI.Tags("TodoService")
// @SnAPI.Status(200, "OK")
// @SnAPI.Response(200, "application/json", model.ListTodosResponse)
func (s *TodoService) ListTodos(r runtime.Request, w runtime.Response) {
	list := todos.list()
	resp := model.ListTodosResponse{Todos: make([]*model.Todo, 0, len(list))}
	for i := range list {
		resp.Todos = append(resp.Todos, &list[i])
	}
	w.Json(http.StatusOK, resp)
}

// @SnAPI.POST("/v1/todos")
// @SnAPI.OperationID("todo.v1.TodoService.CreateTodo")
// @SnAPI.Tags("TodoService")
// @SnAPI.Request("application/json", model.CreateTodoRequest)
// @SnAPI.Status(200, "OK")
// @SnAPI.Response(200, "application/json", model.Todo)
func (s *TodoService) CreateTodo(r runtime.Request, w runtime.Response) {
	var req model.CreateTodoRequest
	if err := r.FromJsonBody(&req); err != nil {
		w.Error(http.StatusBadRequest, err.Error())
		return
	}
	w.Json(http.StatusOK, todos.create(req.Title))
}

// @SnAPI.DELETE("/v1/todos/{id}")
// @SnAPI.OperationID("todo.v1.TodoService.DeleteTodo")
// @SnAPI.Tags("TodoService")
// @SnAPI.Path("id", "string")
// @SnAPI.Status(204, "No Content")
// @SnAPI.Status(404, "todo not found")
func (s *TodoService) DeleteTodo(r runtime.Request, w runtime.Response) {
	id := r.PathValue("id")
	if !todos.delete(id) {
		w.Error(http.StatusNotFound, fmt.Sprintf("todo %q not found", id))
		return
	}
	w.Status(http.StatusNoContent)
}
