package generator

import (
	"runtime"
	"sync"

	"github.com/StevenCyb/SnAPI/internal/models"
	"golang.org/x/sync/errgroup"
)

// step is a single generation unit. Steps are independent and may run concurrently.
type step func() error

// Generator emits the bootstrapped project on disk for a parsed Project.
type Generator struct {
	project *models.Project
	dst     string
	config  Config
	mu      sync.Mutex
}

// NewGenerator creates a new Generator targeting dst.
func NewGenerator(project *models.Project, dst string, cfg Config) *Generator {
	return &Generator{project: project, dst: dst, config: cfg}
}

// Generate runs all registered generation steps.
func (g *Generator) Generate() error {
	steps := []step{
		g.generateModule,
		g.generateServe,
		g.generateRoutes,
	}
	if g.config.Swagger != nil {
		steps = append(steps, g.generateSwagger)
	}
	if len(g.project.Config.StaticFiles) > 0 {
		steps = append(steps, g.generateStatic)
	}
	if hasMCPFeature(g.project) {
		steps = append(steps, g.generateMCP)
	}
	return g.runSteps(steps...)
}

// runSteps executes the given steps concurrently, returning the first error.
func (g *Generator) runSteps(steps ...step) error {
	eg := new(errgroup.Group)
	eg.SetLimit(runtime.GOMAXPROCS(0))
	for _, s := range steps {
		eg.Go(s)
	}
	return eg.Wait()
}
