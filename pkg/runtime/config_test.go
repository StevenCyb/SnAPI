package runtime

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_NonStruct(t *testing.T) {
	t.Parallel()

	_, err := LoadConfigWithOpts[int]()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be a struct")
}

func TestLoadConfig_DefaultsApplied(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Host string `arg:"host,default=localhost"`
		Port int    `arg:"port,default=8080"`
	}

	got, err := LoadConfigWithOpts[cfg](WithEnv(nil), WithArgs(nil))
	require.NoError(t, err)
	assert.Equal(t, "localhost", got.Host)
	assert.Equal(t, 8080, got.Port)
}

func TestLoadConfig_EnvOverridesDefault(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Host string `arg:"host,env=APP_HOST,default=localhost"`
	}

	got, err := LoadConfigWithOpts[cfg](
		WithEnv(map[string]string{"APP_HOST": "example.com"}),
		WithArgs(nil),
	)
	require.NoError(t, err)
	assert.Equal(t, "example.com", got.Host)
}

func TestLoadConfig_EnvFallsBackToName(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Host string `arg:"host"`
	}

	got, err := LoadConfigWithOpts[cfg](
		WithEnv(map[string]string{"HOST": "from-env"}),
		WithArgs(nil),
	)
	require.NoError(t, err)
	assert.Equal(t, "from-env", got.Host)
}

func TestLoadConfig_FlagOverridesEnvAndDefault(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Host string `arg:"host,env=APP_HOST,default=localhost"`
	}

	got, err := LoadConfigWithOpts[cfg](
		WithEnv(map[string]string{"APP_HOST": "from-env"}),
		WithArgs([]string{"--host=from-flag"}),
	)
	require.NoError(t, err)
	assert.Equal(t, "from-flag", got.Host)
}

func TestLoadConfig_CaseInsensitiveLookups(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Host string `arg:"Host,env=App_Host"`
		Port int    `arg:"PORT"`
	}

	got, err := LoadConfigWithOpts[cfg](
		WithEnv(map[string]string{"app_host": "h", "port": "0"}),
		WithArgs([]string{"--PORT", "9090"}),
	)
	require.NoError(t, err)
	assert.Equal(t, "h", got.Host)
	assert.Equal(t, 9090, got.Port)
}

func TestLoadConfig_RequiredMissing(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Token string `arg:"token,required"`
	}

	_, err := LoadConfigWithOpts[cfg](WithEnv(nil), WithArgs(nil))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

func TestLoadConfig_RequiredProvided(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Token string `arg:"token,required"`
	}

	got, err := LoadConfigWithOpts[cfg](
		WithEnv(map[string]string{"TOKEN": "abc"}),
		WithArgs(nil),
	)
	require.NoError(t, err)
	assert.Equal(t, "abc", got.Token)
}

func TestLoadConfig_SkipUntaggedAndDash(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Tagged   string `arg:"tagged,default=t"`
		Skipped  string `arg:"-"`
		Untagged string
	}

	got, err := LoadConfigWithOpts[cfg](
		WithEnv(map[string]string{"SKIPPED": "x", "UNTAGGED": "y"}),
		WithArgs(nil),
	)
	require.NoError(t, err)
	assert.Equal(t, "t", got.Tagged)
	assert.Empty(t, got.Skipped)
	assert.Empty(t, got.Untagged)
}

func TestLoadConfig_UnexportedSkipped(t *testing.T) {
	t.Parallel()

	type cfg struct {
		secret string `arg:"secret,default=nope"` //nolint:unused
		Name   string `arg:"name,default=ok"`
	}

	got, err := LoadConfigWithOpts[cfg]()
	require.NoError(t, err)
	assert.Equal(t, "ok", got.Name)
	assert.Empty(t, got.secret)
}

func TestLoadConfig_AllScalarTypes(t *testing.T) {
	t.Parallel()

	type cfg struct {
		S   string        `arg:"s"`
		B   bool          `arg:"b"`
		I   int           `arg:"i"`
		I8  int8          `arg:"i8"`
		I64 int64         `arg:"i64"`
		U   uint          `arg:"u"`
		U16 uint16        `arg:"u16"`
		F32 float32       `arg:"f32"`
		F64 float64       `arg:"f64"`
		D   time.Duration `arg:"d"`
		PS  *string       `arg:"ps"`
		Sl  []int         `arg:"sl"`
	}

	got, err := LoadConfigWithOpts[cfg](WithEnv(map[string]string{
		"S": "hello", "B": "true",
		"I": "-1", "I8": "-8", "I64": "64",
		"U": "1", "U16": "16",
		"F32": "1.5", "F64": "2.5",
		"D":  "750ms",
		"PS": "ptr",
		"SL": "1, 2 ,3",
	}))
	require.NoError(t, err)
	assert.Equal(t, "hello", got.S)
	assert.True(t, got.B)
	assert.Equal(t, -1, got.I)
	assert.Equal(t, int8(-8), got.I8)
	assert.Equal(t, int64(64), got.I64)
	assert.Equal(t, uint(1), got.U)
	assert.Equal(t, uint16(16), got.U16)
	assert.InDelta(t, 1.5, got.F32, 0.0001)
	assert.InDelta(t, 2.5, got.F64, 0.0001)
	assert.Equal(t, 750*time.Millisecond, got.D)
	require.NotNil(t, got.PS)
	assert.Equal(t, "ptr", *got.PS)
	assert.Equal(t, []int{1, 2, 3}, got.Sl)
}

func TestLoadConfig_EmptySliceValue(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Sl []string `arg:"sl,default="`
	}

	got, err := LoadConfigWithOpts[cfg]()
	require.NoError(t, err)
	assert.Equal(t, []string{}, got.Sl)
}

func TestLoadConfig_ParseErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		run  func() error
		msg  string
	}{
		{
			name: "bad bool",
			run: func() error {
				type c struct {
					B bool `arg:"b"`
				}
				_, err := LoadConfigWithOpts[c](WithEnv(map[string]string{"B": "nope"}))
				return err
			},
			msg: "invalid bool",
		},
		{
			name: "bad int",
			run: func() error {
				type c struct {
					I int `arg:"i"`
				}
				_, err := LoadConfigWithOpts[c](WithEnv(map[string]string{"I": "x"}))
				return err
			},
			msg: "invalid int",
		},
		{
			name: "bad uint",
			run: func() error {
				type c struct {
					U uint `arg:"u"`
				}
				_, err := LoadConfigWithOpts[c](WithEnv(map[string]string{"U": "-1"}))
				return err
			},
			msg: "invalid uint",
		},
		{
			name: "bad float",
			run: func() error {
				type c struct {
					F float64 `arg:"f"`
				}
				_, err := LoadConfigWithOpts[c](WithEnv(map[string]string{"F": "x"}))
				return err
			},
			msg: "invalid float",
		},
		{
			name: "bad duration",
			run: func() error {
				type c struct {
					D time.Duration `arg:"d"`
				}
				_, err := LoadConfigWithOpts[c](WithEnv(map[string]string{"D": "x"}))
				return err
			},
			msg: "invalid duration",
		},
		{
			name: "bad slice element",
			run: func() error {
				type c struct {
					S []int `arg:"s"`
				}
				_, err := LoadConfigWithOpts[c](WithEnv(map[string]string{"S": "1,bad"}))
				return err
			},
			msg: "invalid slice element",
		},
		{
			name: "unsupported type",
			run: func() error {
				type c struct {
					M map[string]string `arg:"m"`
				}
				_, err := LoadConfigWithOpts[c](WithEnv(map[string]string{"M": "x"}))
				return err
			},
			msg: "unsupported type",
		},
		{
			name: "bad default",
			run: func() error {
				type c struct {
					I int `arg:"i,default=bad"`
				}
				_, err := LoadConfigWithOpts[c]()
				return err
			},
			msg: "default",
		},
		{
			name: "bad flag",
			run: func() error {
				type c struct {
					I int `arg:"i"`
				}
				_, err := LoadConfigWithOpts[c](WithArgs([]string{"--i=bad"}))
				return err
			},
			msg: "flag --i",
		},
		{
			name: "unknown option",
			run: func() error {
				type c struct {
					S string `arg:"s,weird=1"`
				}
				_, err := LoadConfigWithOpts[c]()
				return err
			},
			msg: "unknown arg option",
		},
		{
			name: "missing name",
			run: func() error {
				type c struct {
					S string `arg:",default=x"`
				}
				_, err := LoadConfigWithOpts[c]()
				return err
			},
			msg: "missing name",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.run()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.msg)
		})
	}
}

func TestParseFlags(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		want map[string]string
	}{
		{
			name: "equals form",
			args: []string{"--host=example.com", "-port=80"},
			want: map[string]string{"host": "example.com", "port": "80"},
		},
		{
			name: "space form",
			args: []string{"--host", "example.com", "--port", "80"},
			want: map[string]string{"host": "example.com", "port": "80"},
		},
		{
			name: "bool style",
			args: []string{"--debug", "--verbose"},
			want: map[string]string{"debug": "true", "verbose": "true"},
		},
		{
			name: "case insensitive",
			args: []string{"--HOST=x"},
			want: map[string]string{"host": "x"},
		},
		{
			name: "ignores positional and dashes",
			args: []string{"positional", "-", "--", "--name=v"},
			want: map[string]string{"name": "v"},
		},
		{
			name: "empty name skipped",
			args: []string{"--=value"},
			want: map[string]string{},
		},
		{
			name: "nil args",
			args: nil,
			want: map[string]string{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, parseFlags(tc.args))
		})
	}
}

func TestParseArgTag(t *testing.T) {
	t.Parallel()

	tag, err := parseArgTag("name,env=ENV,default=val,required")
	require.NoError(t, err)
	assert.Equal(t, "name", tag.name)
	assert.Equal(t, "ENV", tag.envName)
	assert.Equal(t, "val", tag.defaultVal)
	assert.True(t, tag.hasDefault)
	assert.True(t, tag.required)

	tag, err = parseArgTag("name,,required")
	require.NoError(t, err)
	assert.True(t, tag.required)

	_, err = parseArgTag("name,bogus")
	require.Error(t, err)
}

func TestLowerKeys(t *testing.T) {
	t.Parallel()

	got := lowerKeys(map[string]string{"FOO": "1", "Bar": "2"})
	assert.Equal(t, map[string]string{"foo": "1", "bar": "2"}, got)
}

func TestDefaultEnvAndArgs(t *testing.T) {
	t.Parallel()

	// Just ensure they don't panic and return non-nil/expected shape.
	env := defaultEnv()
	assert.NotNil(t, env)

	args := defaultArgs()
	_ = args // may be nil depending on test runner; just ensuring callable
}

func TestDefaultArgs_BothBranches(t *testing.T) {
	// not parallel: mutates os.Args
	orig := os.Args
	t.Cleanup(func() { os.Args = orig })

	os.Args = []string{"prog"}
	assert.Nil(t, defaultArgs())

	os.Args = []string{"prog", "--x=1"}
	assert.Equal(t, []string{"--x=1"}, defaultArgs())
}

func TestLoadConfig_PointerInnerError(t *testing.T) {
	t.Parallel()

	type cfg struct {
		P *int `arg:"p"`
	}

	_, err := LoadConfigWithOpts[cfg](WithEnv(map[string]string{"P": "bad"}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid int")
}
