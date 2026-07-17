package api

import (
	"net/http"

	"github.com/StevenCyb/SnAPI/pkg/runtime"
)

type Book struct {
	Title string `json:"title"`
}

// @SnAPI.Path("/books")
// @SnAPI.Tags("books")
// @SnAPI.UseMiddleware(api.LoggingMiddleware)
type CRUD struct {
	books []Book
}

func (c *CRUD) Constructor() error {
	c.books = []Book{}
	return nil
}

func (c *CRUD) Destructor() {
	c.books = nil
}

// @SnAPI.GET("/")
// @SnAPI.Response(200, "application/json", []api.Book)
func (c *CRUD) List(req runtime.Request, resp runtime.Response) {
	resp.Json(http.StatusOK, c.books)
}

// @SnAPI.POST("/")
// @SnAPI.Request("application/json", api.Book)
// @SnAPI.Status(201)
func (c *CRUD) Create(req runtime.Request, resp runtime.Response) {
	var newBook Book
	if err := req.FromJsonBody(&newBook); err != nil {
		resp.Error(http.StatusBadRequest, "Invalid JSON")
		return
	}
	c.books = append(c.books, newBook)
	resp.Json(http.StatusCreated, newBook)
}

// @SnAPI.PATCH("/{book}")
// @SnAPI.Request("application/json", api.Book)
// @SnAPI.Status(200)
// @SnAPI.Status(404, "Book not found")
func (c *CRUD) Rename(req runtime.Request, resp runtime.Response) {
	book := req.PathValue("book")
	for i, b := range c.books {
		if b.Title == book {
			var updated Book
			if err := req.FromJsonBody(&updated); err != nil {
				resp.Error(http.StatusBadRequest, "Invalid JSON")
				return
			}
			c.books[i] = updated
			resp.Json(http.StatusOK, updated)
			return
		}
	}
	resp.Error(http.StatusNotFound, "Book not found")
}

// @SnAPI.DELETE("/{book}")
// @SnAPI.Status(204)
// @SnAPI.Status(404, "Book not found")
func (c *CRUD) Delete(req runtime.Request, resp runtime.Response) {
	book := req.PathValue("book")
	for i, b := range c.books {
		if b.Title == book {
			c.books = append(c.books[:i], c.books[i+1:]...)
			resp.Status(http.StatusNoContent)
			return
		}
	}
	resp.Error(http.StatusNotFound, "Book not found")
}
