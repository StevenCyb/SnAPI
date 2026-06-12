package utils

import (
	"regexp"
	"strings"
)

var reAnnotation = regexp.MustCompile(
	`(?mi)^\s*@SnAPI\.(?P<name>[a-z]+)(?P<args>$|\(\)$|(\([a-z0-9./"+:,\s_\-{}\[\]]+\)$))`,
)

// Annotation represents a single annotation with its name and arguments.
type Annotation struct {
	Name string
	Args []string
}

// ExtractAnnotation extracts annotations from the given
// lines of comment and returns a slice of Annotation structs.
func ExtractAnnotation(annotation ...string) []Annotation {
	result := []Annotation{}

	nameIdx := reAnnotation.SubexpIndex("name")
	argsIdx := reAnnotation.SubexpIndex("args")

	for _, block := range annotation {
		for rawLine := range strings.SplitSeq(block, "\n") {
			line := strings.TrimSpace(rawLine)
			if line == "" {
				continue
			}

			matches := reAnnotation.FindStringSubmatch(line)
			if matches == nil {
				continue
			}

			ann := Annotation{Name: matches[nameIdx]}
			argsRaw := strings.TrimSpace(matches[argsIdx])
			argsRaw = strings.TrimPrefix(argsRaw, "(")
			argsRaw = strings.TrimSuffix(argsRaw, ")")
			argsRaw = strings.TrimSpace(argsRaw)

			if argsRaw != "" {
				for part := range strings.SplitSeq(argsRaw, ",") {
					arg := strings.TrimSpace(part)
					arg = strings.Trim(arg, `"`)
					if arg != "" {
						ann.Args = append(ann.Args, arg)
					}
				}
			}

			result = append(result, ann)
		}
	}

	return result
}
