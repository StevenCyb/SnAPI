package api

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/StevenCyb/SnAPI/pkg/runtime"
)

// Notes groups a REST route (List) and MCP entries (Add, Read) on one
// struct to show that MCP reuses the same handler-struct mechanism as HTTP:
// Constructor/Destructor, shared state, and @SnAPI.UseMiddleware all work
// identically for both.
//
// @SnAPI.Path("/notes")
// @SnAPI.UseMiddleware(api.LoggingMiddleware)
type Notes struct {
	mu    sync.Mutex
	items map[string]Note
	seq   int
}

func (n *Notes) Constructor() error {
	n.items = map[string]Note{}
	return nil
}

func (n *Notes) Destructor() {
	n.items = nil
}

// @SnAPI.GET("/")
// @SnAPI.Response(200, "application/json", []api.Note)
func (n *Notes) List(req runtime.Request, resp runtime.Response) {
	n.mu.Lock()
	defer n.mu.Unlock()

	out := make([]Note, 0, len(n.items))
	for _, note := range n.items {
		out = append(out, note)
	}
	resp.Json(http.StatusOK, out)
}

// @SnAPI.MCPTool("add_note", "Adds a short text note and returns its id")
// @SnAPI.Request(api.AddNoteArgs)
// @SnAPI.MCPOutput(api.AddNoteResult)
func (n *Notes) Add(req runtime.Request, resp runtime.Response) {
	var args AddNoteArgs
	if err := req.FromJsonBody(&args); err != nil {
		resp.Error(http.StatusBadRequest, "invalid arguments")
		return
	}

	n.mu.Lock()
	n.seq++
	id := fmt.Sprintf("note-%d", n.seq)
	n.items[id] = Note{ID: id, Text: args.Text}
	n.mu.Unlock()

	resp.Json(http.StatusOK, AddNoteResult{ID: id})
}

// @SnAPI.MCPResource("note:///{id}", "Note", "Reads a single note by id", "text/plain")
func (n *Notes) Read(req runtime.Request, resp runtime.Response) {
	n.mu.Lock()
	note, ok := n.items[req.PathValue("id")]
	n.mu.Unlock()

	if !ok {
		resp.Error(http.StatusNotFound, "note not found")
		return
	}
	resp.Text(http.StatusOK, note.Text)
}
