package runtime

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

// LoadConfig builds T from environment variables and CLI flags.
//
// The result is cached per type T: subsequent calls return the same value
// without re-reading the environment or args. Use [ResetConfigCache] to
// invalidate, or call [LoadConfigWithOpts] directly to bypass the cache.
//
// See [LoadConfigWithOpts] for tag semantics and precedence.
//
// Struct fields are configured via the `arg` tag:
//
//	`arg:"name"`                              required position-less flag/env
//	`arg:"name,env=ENV_VAR"`                  custom env var name
//	`arg:"name,default=value"`                fallback value
//	`arg:"name,env=ENV_VAR,default=value"`    combined
//	`arg:"name,required"`                     must be supplied
//	`arg:"-"`                                 explicitly skipped
//
// Lookup precedence (later wins): default -> env -> flag.
// Name matching is case-insensitive. When `env=` is omitted, the env var
// name equals the field's `name`.
func LoadConfig[T any]() (T, error) {
	var zero T
	key := reflect.TypeFor[T]()

	c, _ := configCache.LoadOrStore(key, &cachedConfig{})
	entry := c.(*cachedConfig)
	entry.once.Do(func() {
		entry.val, entry.err = LoadConfigWithOpts[T]()
	})

	if entry.err != nil {
		return zero, entry.err
	}

	return entry.val.(T), nil
}

// ResetConfigCache clears all cached configs loaded via [LoadConfig].
func ResetConfigCache() {
	configCache = sync.Map{}
}

type cachedConfig struct {
	once sync.Once
	val  any
	err  error
}

var configCache sync.Map // map[reflect.Type]*cachedConfig

// LoadConfigWithOpts builds T from defaults, environment variables and CLI flags.
//
// Struct fields are configured via the `arg` tag:
//
//	`arg:"name"`                              required position-less flag/env
//	`arg:"name,env=ENV_VAR"`                  custom env var name
//	`arg:"name,default=value"`                fallback value
//	`arg:"name,env=ENV_VAR,default=value"`    combined
//	`arg:"name,required"`                     must be supplied
//	`arg:"-"`                                 explicitly skipped
//
// Lookup precedence (later wins): default -> env -> flag.
// Name matching is case-insensitive. When `env=` is omitted, the env var
// name equals the field's `name`.
func LoadConfigWithOpts[T any](opts ...Option) (T, error) {
	o := loadOptions{
		env:  defaultEnv(),
		args: defaultArgs(),
	}
	for _, apply := range opts {
		apply(&o)
	}

	var cfg T

	v := reflect.ValueOf(&cfg).Elem()
	t := v.Type()
	if t.Kind() != reflect.Struct {
		return cfg, fmt.Errorf("LoadConfig: T must be a struct, got %s", t.Kind())
	}

	env := lowerKeys(o.env)
	flags := parseFlags(o.args)

	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		fv := v.Field(i)

		if !sf.IsExported() || !fv.CanSet() {
			continue
		}

		raw, ok := sf.Tag.Lookup("arg")
		if !ok || raw == "-" {
			continue
		}

		tag, err := parseArgTag(raw)
		if err != nil {
			return cfg, fmt.Errorf("field %q: %w", sf.Name, err)
		}
		if tag.name == "" {
			return cfg, fmt.Errorf("field %q: arg tag missing name", sf.Name)
		}

		envKey := tag.envName
		if envKey == "" {
			envKey = tag.name
		}

		set := false
		if tag.hasDefault {
			if err := setValueFromString(fv, sf.Type, tag.defaultVal); err != nil {
				return cfg, fmt.Errorf("field %q (default): %w", sf.Name, err)
			}
			set = true
		}
		if val, ok := env[strings.ToLower(envKey)]; ok {
			if err := setValueFromString(fv, sf.Type, val); err != nil {
				return cfg, fmt.Errorf("field %q (env %s): %w", sf.Name, envKey, err)
			}
			set = true
		}
		if val, ok := flags[strings.ToLower(tag.name)]; ok {
			if err := setValueFromString(fv, sf.Type, val); err != nil {
				return cfg, fmt.Errorf("field %q (flag --%s): %w", sf.Name, tag.name, err)
			}
			set = true
		}

		if tag.required && !set {
			return cfg, fmt.Errorf("field %q: required value not provided", sf.Name)
		}
	}

	return cfg, nil
}

// Option customises a LoadConfig call.
type Option func(*loadOptions)

type loadOptions struct {
	env  map[string]string
	args []string
}

// WithEnv overrides the environment used by LoadConfig.
func WithEnv(env map[string]string) Option {
	return func(o *loadOptions) { o.env = env }
}

// WithArgs overrides the CLI args used by LoadConfig (without the program name).
func WithArgs(args []string) Option {
	return func(o *loadOptions) { o.args = args }
}

type argTag struct {
	name       string
	envName    string
	defaultVal string
	hasDefault bool
	required   bool
}

func parseArgTag(raw string) (argTag, error) {
	var tag argTag
	parts := strings.Split(raw, ",")
	tag.name = strings.TrimSpace(parts[0])

	for _, p := range parts[1:] {
		p = strings.TrimSpace(p)
		switch {
		case p == "":
			continue
		case p == "required":
			tag.required = true
		case strings.HasPrefix(p, "env="):
			tag.envName = strings.TrimSpace(strings.TrimPrefix(p, "env="))
		case strings.HasPrefix(p, "default="):
			tag.defaultVal = strings.TrimPrefix(p, "default=")
			tag.hasDefault = true
		default:
			return tag, fmt.Errorf("unknown arg option %q", p)
		}
	}
	return tag, nil
}

func parseFlags(args []string) map[string]string {
	out := map[string]string{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") || a == "-" || a == "--" {
			continue
		}
		a = strings.TrimLeft(a, "-")
		name, val, hasEq := strings.Cut(a, "=")
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" {
			continue
		}
		switch {
		case hasEq:
			out[name] = val
		case i+1 < len(args) && !strings.HasPrefix(args[i+1], "-"):
			out[name] = args[i+1]
			i++
		default:
			out[name] = "true"
		}
	}
	return out
}

func lowerKeys(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[strings.ToLower(k)] = v
	}
	return out
}

func defaultEnv() map[string]string {
	entries := os.Environ()
	out := make(map[string]string, len(entries))
	for _, e := range entries {
		if k, v, ok := strings.Cut(e, "="); ok {
			out[k] = v
		}
	}
	return out
}

func defaultArgs() []string {
	if len(os.Args) <= 1 {
		return nil
	}
	return os.Args[1:]
}

func setValueFromString(v reflect.Value, t reflect.Type, raw string) error {
	if t.Kind() == reflect.Pointer {
		elem := reflect.New(t.Elem())
		if err := setValueFromString(elem.Elem(), t.Elem(), raw); err != nil {
			return err
		}
		v.Set(elem)
		return nil
	}

	if t == reflect.TypeFor[time.Duration]() {
		d, err := time.ParseDuration(strings.TrimSpace(raw))
		if err != nil {
			return fmt.Errorf("invalid duration %q: %w", raw, err)
		}
		v.SetInt(int64(d))
		return nil
	}

	switch t.Kind() {
	case reflect.String:
		v.SetString(raw)
		return nil
	case reflect.Bool:
		parsed, err := strconv.ParseBool(strings.TrimSpace(raw))
		if err != nil {
			return fmt.Errorf("invalid bool %q: %w", raw, err)
		}
		v.SetBool(parsed)
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		parsed, err := strconv.ParseInt(strings.TrimSpace(raw), 10, t.Bits())
		if err != nil {
			return fmt.Errorf("invalid int %q: %w", raw, err)
		}
		v.SetInt(parsed)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		parsed, err := strconv.ParseUint(strings.TrimSpace(raw), 10, t.Bits())
		if err != nil {
			return fmt.Errorf("invalid uint %q: %w", raw, err)
		}
		v.SetUint(parsed)
		return nil
	case reflect.Float32, reflect.Float64:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(raw), t.Bits())
		if err != nil {
			return fmt.Errorf("invalid float %q: %w", raw, err)
		}
		v.SetFloat(parsed)
		return nil
	case reflect.Slice:
		raw = strings.TrimSpace(raw)
		if raw == "" {
			v.Set(reflect.MakeSlice(t, 0, 0))
			return nil
		}
		parts := strings.Split(raw, ",")
		s := reflect.MakeSlice(t, 0, len(parts))
		for _, part := range parts {
			elem := reflect.New(t.Elem()).Elem()
			if err := setValueFromString(elem, t.Elem(), strings.TrimSpace(part)); err != nil {
				return fmt.Errorf("invalid slice element %q: %w", part, err)
			}
			s = reflect.Append(s, elem)
		}
		v.Set(s)
		return nil
	default:
		return fmt.Errorf("unsupported type %s", t.String())
	}
}
