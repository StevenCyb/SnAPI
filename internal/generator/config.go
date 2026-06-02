package generator

// Config tunes the generator. All fields are optional.
type Config struct {
	// Addr is the bind address of the generated server. Defaults to ":8080".
	Addr string
	// Swagger, when non-nil, enables Swagger UI + spec generation.
	Swagger *SwaggerConfig
}

// SwaggerConfig configures the embedded Swagger UI. Project metadata
// (title, description, version) is sourced from package-level @SnAPI.* annotations.
type SwaggerConfig struct {
	// Path is the URL prefix where Swagger UI is served. Defaults to "/swagger".
	Path string
}

func (c Config) addr() string {
	if c.Addr == "" {
		return ":8080"
	}
	return c.Addr
}

func (s *SwaggerConfig) path() string {
	if s.Path == "" {
		return "/swagger"
	}
	return s.Path
}
