package model

import "github.com/google/uuid"

type Todo struct {
	ID    string
	Title string
	Done  bool
}

func NewTodo(title string) *Todo {
	return &Todo{
		ID:    uuid.New().String(),
		Title: title,
		Done:  false,
	}
}

func (t Todo) GetID() string {
	return t.ID
}
