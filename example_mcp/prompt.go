package api

import (
	"net/http"

	"github.com/StevenCyb/SnAPI/pkg/runtime"
)

// @SnAPI.MCPPrompt("summarize_notes", "Ask the LLM to summarize a note")
// @SnAPI.MCPPromptArg("text", "the note text to summarize", "true")
func SummarizeNotes(req runtime.Request, resp runtime.Response) {
	resp.Text(http.StatusOK, "Please summarize this note in one sentence:\n"+req.QueryValue("text"))
}
