package generator

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"html/template"
	"regexp"
	"strconv"
	"strings"
)

//go:embed template/swagger_ui.tmpl
var swaggerUITemplateString string

//nolint:gochecknoglobals // templates are immutable and safe for reuse
var swaggerUITemplate = template.Must(template.New("swagger-ui").Parse(swaggerUITemplateString))

// renderSwaggerUI returns the minified HTML for the Swagger UI page that
// inlines the given JSON spec.
func renderSwaggerUI(title, specJSON string) (string, error) {
	cfg := map[string]any{
		"spec":                   json.RawMessage(specJSON),
		"deepLinking":            true,
		"docExpansion":           "list",
		"displayRequestDuration": true,
		"layout":                 "BaseLayout",
	}
	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := swaggerUITemplate.Execute(&buf, map[string]any{
		"Title":      title,
		"ConfigJSON": template.JS(cfgJSON), //nolint:gosec // configJSON is produced by json.Marshal
	}); err != nil {
		return "", err
	}
	return minifyHTML(buf.String()), nil
}

//nolint:gochecknoglobals
var whitespaceRegex = regexp.MustCompile(`\s+`)

func minifyHTML(in string) string {
	s := strings.NewReplacer("\t", " ", "\n", " ", "\r", " ").Replace(in)
	s = whitespaceRegex.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "> <", "><")
	return strings.TrimSpace(s)
}

// escapeForGoStringLiteral produces a string safe to embed between double
// quotes in generated Go source.
func escapeForGoStringLiteral(s string) string {
	q := strconv.Quote(s)
	return q[1 : len(q)-1]
}
