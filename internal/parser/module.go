package parser

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/StevenCyb/SnAPI/internal/models"
)

// SnAPIModulePath is the import path of the SnAPI runtime module. Requires
// matching this path are kept as direct (non-indirect) dependencies in the
// generated go.mod, since the generated main always imports the runtime.
const SnAPIModulePath = "github.com/StevenCyb/SnAPI"

// parseModule parses the go.mod file in p.path and stores the
// resulting Module on p.project.MainModule.
func (p *Parser) parseModule() error {
	dir, err := filepath.Abs(p.path)
	if err != nil {
		return &AbsolutePathResolutionError{Path: p.path, Err: err}
	}
	modFile := filepath.Join(dir, "go.mod")
	content, err := os.ReadFile(modFile) //nolint:gosec // path is provided by callers, not from user input
	if err != nil {
		return &ReadGoModError{Dir: dir, Err: err}
	}

	lines := strings.Split(string(content), "\n")
	var (
		modPath  string
		goVer    string
		requires []models.Require
		replaces []models.Replace
	)
	for i := 0; i < len(lines); i++ {
		line := stripComment(strings.TrimSpace(lines[i]))
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "module "):
			modPath = strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "module ")), `"`)
		case strings.HasPrefix(line, "go "):
			goVer = strings.TrimSpace(strings.TrimPrefix(line, "go "))
		case strings.HasPrefix(line, "require ("):
			block, next := parseBlock(lines, i+1)
			requires = append(requires, parseRequireBlock(block)...)
			i = next
		case strings.HasPrefix(line, "replace ("):
			block, next := parseBlock(lines, i+1)
			replaces = append(replaces, parseReplaceBlock(block)...)
			i = next
		case strings.HasPrefix(line, "require "):
			if req, ok := parseRequireLine(strings.TrimPrefix(line, "require ")); ok {
				requires = append(requires, req)
			}
		case strings.HasPrefix(line, "replace "):
			if rep, ok := parseReplaceLine(strings.TrimPrefix(line, "replace ")); ok {
				replaces = append(replaces, rep)
			}
		}
	}
	if modPath == "" {
		return ErrModulePathNotFound
	}

	requires = append(requires, models.Require{Path: modPath, Version: "v0.0.0"})
	for i := range replaces {
		if replaces[i].NewVersion == "" && !filepath.IsAbs(replaces[i].NewPath) {
			replaces[i].NewPath = filepath.Join(dir, replaces[i].NewPath)
		}
	}
	replaces = append(replaces, models.Replace{OldPath: modPath, NewPath: dir})
	for i := range requires {
		if requires[i].Path != modPath && requires[i].Path != SnAPIModulePath {
			requires[i].Indirect = true
		}
	}

	p.project.MainModule = &models.Module{
		Path:      modPath,
		GoVersion: goVer,
		Requires:  requires,
		Replaces:  replaces,
		Dir:       dir,
	}
	return nil
}

// stripComment removes any trailing "// ..." comment from a line.
func stripComment(line string) string {
	if idx := strings.Index(line, "//"); idx >= 0 {
		return strings.TrimSpace(line[:idx])
	}
	return line
}

// parseBlock extracts lines in a parenthesis block and returns the block and the index after the block.
func parseBlock(lines []string, start int) ([]string, int) {
	var block []string
	for i := start; i < len(lines); i++ {
		line := stripComment(strings.TrimSpace(lines[i]))
		if line == ")" {
			return block, i
		}
		if line == "" {
			continue
		}
		block = append(block, line)
	}
	return block, len(lines) - 1
}

func parseRequireBlock(block []string) []models.Require {
	var reqs []models.Require
	for _, line := range block {
		if req, ok := parseRequireLine(line); ok {
			reqs = append(reqs, req)
		}
	}
	return reqs
}

func parseReplaceBlock(block []string) []models.Replace {
	var reps []models.Replace
	for _, line := range block {
		if rep, ok := parseReplaceLine(line); ok {
			reps = append(reps, rep)
		}
	}
	return reps
}

// parseRequireLine parses a require line of the form: path version [// indirect].
func parseRequireLine(line string) (models.Require, bool) {
	indirect := false
	if idx := strings.Index(line, "//"); idx >= 0 {
		indirect = strings.TrimSpace(line[idx+2:]) == "indirect"
		line = strings.TrimSpace(line[:idx])
	}
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return models.Require{}, false
	}
	return models.Require{Path: fields[0], Version: fields[1], Indirect: indirect}, true
}

// parseReplaceLine parses a replace line of the form: old [version] => new [version].
func parseReplaceLine(line string) (models.Replace, bool) {
	left, right, ok := strings.Cut(stripComment(line), "=>")
	if !ok {
		return models.Replace{}, false
	}
	leftFields := strings.Fields(left)
	rightFields := strings.Fields(right)
	rep := models.Replace{}
	if len(leftFields) > 0 {
		rep.OldPath = leftFields[0]
	}
	if len(leftFields) > 1 {
		rep.OldVersion = leftFields[1]
	}
	if len(rightFields) > 0 {
		rep.NewPath = rightFields[0]
	}
	if len(rightFields) > 1 {
		rep.NewVersion = rightFields[1]
	}
	return rep, true
}
