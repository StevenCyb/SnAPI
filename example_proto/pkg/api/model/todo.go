package model

type Todo struct {
	Id    string `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

type GetTodoRequest struct {
	Id string `json:"id"`
}

type ListTodosRequest struct {
}

type ListTodosResponse struct {
	Todos []*Todo `json:"todos"`
}

type CreateTodoRequest struct {
	Title string `json:"title"`
}

type DeleteTodoRequest struct {
	Id string `json:"id"`
}

type PingResponse struct {
	Message string `json:"message"`
}
