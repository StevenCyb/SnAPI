package api

import (
	"fmt"

	"github.com/StevenCyb/example/pkg/model"
	"github.com/StevenCyb/example/pkg/repository"
)

var todoRepo repository.Repository[model.Todo]

// @snapi.setup
func Setup() {
	fmt.Println("Setting up repository...")
	todoRepo = repository.NewMemoryRepository[model.Todo]()
}

// @snapi.teardown
func Teardown() {
	fmt.Println("Tearing down...")
	todoRepo = nil
}
