package common

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestDuration_UnmarshalText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{
			name:     "nanoseconds",
			input:    "100ns",
			expected: 100 * time.Nanosecond,
			wantErr:  false,
		},
		{
			name:     "microseconds",
			input:    "500us",
			expected: 500 * time.Microsecond,
			wantErr:  false,
		},
		{
			name:     "milliseconds",
			input:    "250ms",
			expected: 250 * time.Millisecond,
			wantErr:  false,
		},
		{
			name:     "seconds",
			input:    "30s",
			expected: 30 * time.Second,
			wantErr:  false,
		},
		{
			name:     "minutes",
			input:    "5m",
			expected: 5 * time.Minute,
			wantErr:  false,
		},
		{
			name:     "hours",
			input:    "2h",
			expected: 2 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "complex duration",
			input:    "1h30m45s",
			expected: 1*time.Hour + 30*time.Minute + 45*time.Second,
			wantErr:  false,
		},
		{
			name:     "zero duration",
			input:    "0s",
			expected: 0,
			wantErr:  false,
		},
		{
			name:     "invalid format - no unit",
			input:    "100",
			expected: 0,
			wantErr:  true,
		},
		{
			name:     "invalid format - invalid unit",
			input:    "100x",
			expected: 0,
			wantErr:  true,
		},
		{
			name:     "invalid format - empty string",
			input:    "",
			expected: 0,
			wantErr:  true,
		},
		{
			name:     "invalid format - non-numeric",
			input:    "abcs",
			expected: 0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d Duration
			err := d.UnmarshalText([]byte(tt.input))

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, d.Duration)
			}
		})
	}
}

func TestNewDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
	}{
		{
			name:     "zero duration",
			duration: 0,
		},
		{
			name:     "1 second",
			duration: time.Second,
		},
		{
			name:     "5 minutes",
			duration: 5 * time.Minute,
		},
		{
			name:     "1 hour",
			duration: time.Hour,
		},
		{
			name:     "complex duration",
			duration: 1*time.Hour + 30*time.Minute + 45*time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDuration(tt.duration)
			assert.Equal(t, tt.duration, d.Duration)
		})
	}
}

func TestDuration_JSONUnmarshal(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		expected time.Duration
		wantErr  bool
	}{
		{
			name:     "valid duration in JSON",
			json:     `{"timeout":"30s"}`,
			expected: 30 * time.Second,
			wantErr:  false,
		},
		{
			name:     "complex duration in JSON",
			json:     `{"timeout":"1h30m"}`,
			expected: 90 * time.Minute,
			wantErr:  false,
		},
		{
			name:     "milliseconds in JSON",
			json:     `{"timeout":"500ms"}`,
			expected: 500 * time.Millisecond,
			wantErr:  false,
		},
		{
			name:     "invalid duration in JSON",
			json:     `{"timeout":"invalid"}`,
			expected: 0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config struct {
				Timeout Duration `json:"timeout"`
			}

			err := json.Unmarshal([]byte(tt.json), &config)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, config.Timeout.Duration)
			}
		})
	}
}

func TestDuration_YAMLUnmarshal(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected time.Duration
		wantErr  bool
	}{
		{
			name:     "valid duration in YAML",
			yaml:     "timeout: 30s\n",
			expected: 30 * time.Second,
			wantErr:  false,
		},
		{
			name:     "complex duration in YAML",
			yaml:     "timeout: 1h30m45s\n",
			expected: 1*time.Hour + 30*time.Minute + 45*time.Second,
			wantErr:  false,
		},
		{
			name:     "milliseconds in YAML",
			yaml:     "timeout: 250ms\n",
			expected: 250 * time.Millisecond,
			wantErr:  false,
		},
		{
			name:     "invalid duration in YAML",
			yaml:     "timeout: invalid\n",
			expected: 0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config struct {
				Timeout Duration `yaml:"timeout"`
			}

			err := yaml.Unmarshal([]byte(tt.yaml), &config)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, config.Timeout.Duration)
			}
		})
	}
}

func TestDuration_JSONSchema(t *testing.T) {
	d := Duration{}
	schema := d.JSONSchema()

	require.NotNil(t, schema)
	assert.Equal(t, "string", schema.Type)
	assert.Equal(t, "Duration", schema.Title)
	assert.Contains(t, schema.Description, "Duration expressed in units")
	assert.NotEmpty(t, schema.Examples)
	assert.Contains(t, schema.Examples, "1m")
	assert.Contains(t, schema.Examples, "300ms")
}

func TestDuration_ZeroValue(t *testing.T) {
	var d Duration
	assert.Equal(t, time.Duration(0), d.Duration)
}

func TestDuration_Roundtrip(t *testing.T) {
	// Test JSON roundtrip
	t.Run("JSON roundtrip", func(t *testing.T) {
		original := struct {
			Timeout Duration `json:"timeout"`
		}{
			Timeout: NewDuration(5 * time.Minute),
		}

		data, err := json.Marshal(original)
		require.NoError(t, err)

		var decoded struct {
			Timeout Duration `json:"timeout"`
		}
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.Equal(t, original.Timeout.Duration, decoded.Timeout.Duration)
	})

	// Test YAML roundtrip
	t.Run("YAML roundtrip", func(t *testing.T) {
		original := struct {
			Timeout Duration `yaml:"timeout"`
		}{
			Timeout: NewDuration(10 * time.Second),
		}

		data, err := yaml.Marshal(original)
		require.NoError(t, err)

		var decoded struct {
			Timeout Duration `yaml:"timeout"`
		}
		err = yaml.Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.Equal(t, original.Timeout.Duration, decoded.Timeout.Duration)
	})
}
