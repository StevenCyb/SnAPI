package parser

import (
	"strings"

	"github.com/StevenCyb/SnAPI/internal/models"
	"github.com/StevenCyb/SnAPI/internal/parser/utils"
)

// parseConfig is a convenience wrapper around walkAndExtract that runs only
// the config extractor. Intended primarily for tests.
func (p *Parser) parseConfig() error {
	return p.walkAndExtract(p.configExtractor)
}

// configExtractor scans the package-level doc comment of a parsed file for
// project-wide @snapi.* annotations (title, description, version, server,
// securityScheme) and merges them into p.project.Config.
func (p *Parser) configExtractor(fc fileCtx) error {
	if fc.File.Doc == nil {
		return nil
	}

	var (
		title, description, version string
		servers                     []models.ProjectServer
		schemes                     []models.SecurityScheme
	)
	for _, ann := range utils.ExtractAnnotation(commentText(fc.File.Doc)) {
		switch strings.ToLower(ann.Name) {
		case "title":
			if len(ann.Args) > 0 {
				title = ann.Args[0]
			}
		case "description":
			if len(ann.Args) > 0 {
				description = ann.Args[0]
			}
		case "version":
			if len(ann.Args) > 0 {
				version = ann.Args[0]
			}
		case "server":
			if len(ann.Args) == 0 {
				continue
			}
			s := models.ProjectServer{URL: ann.Args[0]}
			if len(ann.Args) > 1 {
				s.Description = ann.Args[1]
			}
			servers = append(servers, s)
		case "securityscheme":
			if len(ann.Args) < 2 {
				continue
			}
			sc := models.SecurityScheme{Name: ann.Args[0], Type: ann.Args[1]}
			if len(ann.Args) > 2 {
				sc.Scheme = ann.Args[2]
			}
			if len(ann.Args) > 3 {
				sc.BearerFormat = ann.Args[3]
			}
			schemes = append(schemes, sc)
		}
	}

	if title == "" && description == "" && version == "" && len(servers) == 0 && len(schemes) == 0 {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if title != "" {
		p.project.Config.Title = title
	}
	if description != "" {
		p.project.Config.Description = description
	}
	if version != "" {
		p.project.Config.Version = version
	}
	p.project.Config.Servers = append(p.project.Config.Servers, servers...)
	p.project.Config.SecuritySchemes = append(p.project.Config.SecuritySchemes, schemes...)
	return nil
}
