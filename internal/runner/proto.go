package runner

import (
	"github.com/StevenCyb/SnAPI/internal/logger"
	"github.com/StevenCyb/SnAPI/internal/protogen"
)

// GenerateProto generates an annotated `api` package plus DTOs from specPath
// into outputDir. outputDir must already contain a go.mod. Exits the process
// on failure, matching Bootstrap/Build.
func GenerateProto(specPath, outputDir string) {
	log := logger.Scope("proto")
	log.Info("Generating API from %s into %s", specPath, outputDir)
	if err := protogen.Generate(specPath, outputDir); err != nil {
		log.Fatal("failed to generate from proto: %v", err)
	}
	log.Debug("generation done")
}
