package parser

import (
	"sync"

	"github.com/StevenCyb/SnAPI/internal/models"
)

type Parser struct {
	path             string
	project          *models.Project
	mu               sync.Mutex
	structCandidates map[string]*partialStructData // key: importPath+"."+TypeName
}

func NewParser(path string) *Parser {
	return &Parser{
		path:             path,
		project:          &models.Project{},
		structCandidates: make(map[string]*partialStructData),
	}
}

func (p *Parser) Parse() (*models.Project, error) {
	if err := p.parseModule(); err != nil {
		return nil, err
	}
	if err := p.walkAndExtract(p.configExtractor, p.lifecycleExtractor, p.middlewareExtractor, p.handlerExtractor, p.handlerStructExtractor); err != nil {
		return nil, err
	}
	if err := p.assembleHandlerStructs(); err != nil {
		return nil, err
	}
	return p.project, nil
}
