package parser

import (
	"sync"

	"github.com/StevenCyb/SnAPI/internal/models"
)

type Parser struct {
	path    string
	project *models.Project
	mu      sync.Mutex
}

func NewParser(path string) *Parser {
	return &Parser{path: path, project: &models.Project{}}
}

func (p *Parser) Parse() (*models.Project, error) {
	if err := p.parseModule(); err != nil {
		return nil, err
	}
	if err := p.walkAndExtract(p.configExtractor, p.lifecycleExtractor, p.middlewareExtractor, p.handlerExtractor); err != nil {
		return nil, err
	}
	return p.project, nil
}
