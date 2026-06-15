package runner

import (
	"bufio"
	"os"
	"strings"
)

// parseDotEnv reads the file at path and returns key=value pairs for each
// non-blank, non-comment line. The supported syntax is:
//
//   - KEY=VALUE
//   - export KEY=VALUE   (leading "export" is stripped)
//   - # comment          (ignored)
//   - "quoted" or 'quoted' values  (outer quotes are stripped)
func parseDotEnv(path string) ([]string, error) {
	f, err := os.Open(path) //nolint:gosec // path is operator-supplied, not user input
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var pairs []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		line = strings.TrimPrefix(line, "export ")

		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)

		// Strip matching surrounding quotes.
		if len(v) >= 2 {
			if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
				v = v[1 : len(v)-1]
			}
		}

		if k == "" {
			continue
		}
		pairs = append(pairs, k+"="+v)
	}
	return pairs, sc.Err()
}

// injectDotEnv parses dotenvPath and appends any variables not already present
// in env to the returned slice. Variables already set in env take precedence,
// so .env acts as a set of defaults. Returns (env, nil) if dotenvPath is empty.
func injectDotEnv(env []string, dotenvPath string) ([]string, error) {
	if dotenvPath == "" {
		return env, nil
	}

	pairs, err := parseDotEnv(dotenvPath)
	if err != nil {
		return env, err
	}
	if len(pairs) == 0 {
		return env, nil
	}

	// Build a set of keys already present so we don't override them.
	existing := make(map[string]struct{}, len(env))
	for _, kv := range env {
		k, _, _ := strings.Cut(kv, "=")
		existing[k] = struct{}{}
	}

	result := make([]string, 0, len(env)+len(pairs))
	result = append(result, env...)
	for _, kv := range pairs {
		k, _, _ := strings.Cut(kv, "=")
		if _, ok := existing[k]; !ok {
			result = append(result, kv)
		}
	}
	return result, nil
}
