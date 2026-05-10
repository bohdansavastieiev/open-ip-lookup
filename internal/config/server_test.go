package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeValidServerConfig() ServerConfig {
	return ServerConfig{
		Addr:                     ":8080",
		ReadHeaderTimeoutSeconds: 5,
		ReadTimeoutSeconds:       15,
		WriteTimeoutSeconds:      15,
		IdleTimeoutSeconds:       60,
		ShutdownTimeoutSeconds:   5,
	}
}

func TestConfigValidate_ReturnsNilForValidConfigurations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ServerConfig)
	}{
		{
			name: "valid default values",
			mutate: func(*ServerConfig) {
			},
		},
		{
			name: "all timeout values at minimum",
			mutate: func(c *ServerConfig) {
				c.ReadHeaderTimeoutSeconds = minTimeoutSeconds
				c.ReadTimeoutSeconds = minTimeoutSeconds
				c.WriteTimeoutSeconds = minTimeoutSeconds
				c.IdleTimeoutSeconds = minTimeoutSeconds
				c.ShutdownTimeoutSeconds = minTimeoutSeconds
			},
		},
		{
			name: "all timeout values at maximum",
			mutate: func(c *ServerConfig) {
				c.ReadHeaderTimeoutSeconds = maxTimeoutSeconds
				c.ReadTimeoutSeconds = maxTimeoutSeconds
				c.WriteTimeoutSeconds = maxTimeoutSeconds
				c.IdleTimeoutSeconds = maxTimeoutSeconds
				c.ShutdownTimeoutSeconds = maxTimeoutSeconds
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := makeValidServerConfig()
			tt.mutate(&cfg)
			expected := cfg

			require.NoError(t, cfg.validate())
			assert.Equal(t, expected, cfg, "validate() mutated config")
		})
	}
}

func TestConfigValidate_ReturnsFieldErrorForInvalidConfigurations(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*ServerConfig)
		wantField string
		wantCode  Code
	}{
		{
			name: "empty addr",
			mutate: func(c *ServerConfig) {
				c.Addr = ""
			},
			wantField: "server.addr",
			wantCode:  CodeRequired,
		},
		{
			name: "addr has edge whitespace",
			mutate: func(c *ServerConfig) {
				c.Addr = " :8080"
			},
			wantField: "server.addr",
			wantCode:  CodeWhitespace,
		},
		{
			name: "invalid addr format",
			mutate: func(c *ServerConfig) {
				c.Addr = "localhost"
			},
			wantField: "server.addr",
			wantCode:  CodeInvalid,
		},
		{
			name: "read_header_timeout below minimum",
			mutate: func(c *ServerConfig) {
				c.ReadHeaderTimeoutSeconds = minTimeoutSeconds - 1
			},
			wantField: "server.read_header_timeout_seconds",
			wantCode:  CodeRange,
		},
		{
			name: "read_header_timeout above maximum",
			mutate: func(c *ServerConfig) {
				c.ReadHeaderTimeoutSeconds = maxTimeoutSeconds + 1
			},
			wantField: "server.read_header_timeout_seconds",
			wantCode:  CodeRange,
		},
		{
			name: "read_timeout below minimum",
			mutate: func(c *ServerConfig) {
				c.ReadTimeoutSeconds = minTimeoutSeconds - 1
			},
			wantField: "server.read_timeout_seconds",
			wantCode:  CodeRange,
		},
		{
			name: "read_timeout above maximum",
			mutate: func(c *ServerConfig) {
				c.ReadTimeoutSeconds = maxTimeoutSeconds + 1
			},
			wantField: "server.read_timeout_seconds",
			wantCode:  CodeRange,
		},
		{
			name: "write_timeout below minimum",
			mutate: func(c *ServerConfig) {
				c.WriteTimeoutSeconds = minTimeoutSeconds - 1
			},
			wantField: "server.write_timeout_seconds",
			wantCode:  CodeRange,
		},
		{
			name: "write_timeout above maximum",
			mutate: func(c *ServerConfig) {
				c.WriteTimeoutSeconds = maxTimeoutSeconds + 1
			},
			wantField: "server.write_timeout_seconds",
			wantCode:  CodeRange,
		},
		{
			name: "idle_timeout below minimum",
			mutate: func(c *ServerConfig) {
				c.IdleTimeoutSeconds = minTimeoutSeconds - 1
			},
			wantField: "server.idle_timeout_seconds",
			wantCode:  CodeRange,
		},
		{
			name: "idle_timeout above maximum",
			mutate: func(c *ServerConfig) {
				c.IdleTimeoutSeconds = maxTimeoutSeconds + 1
			},
			wantField: "server.idle_timeout_seconds",
			wantCode:  CodeRange,
		},
		{
			name: "shutdown_timeout below minimum",
			mutate: func(c *ServerConfig) {
				c.ShutdownTimeoutSeconds = minTimeoutSeconds - 1
			},
			wantField: "server.shutdown_timeout_seconds",
			wantCode:  CodeRange,
		},
		{
			name: "shutdown_timeout above maximum",
			mutate: func(c *ServerConfig) {
				c.ShutdownTimeoutSeconds = maxTimeoutSeconds + 1
			},
			wantField: "server.shutdown_timeout_seconds",
			wantCode:  CodeRange,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := makeValidServerConfig()
			tt.mutate(&cfg)
			expected := cfg

			err := cfg.validate()
			require.Error(t, err)

			var fieldErr *FieldError
			require.ErrorAs(t, err, &fieldErr)

			assert.Equal(t, tt.wantField, fieldErr.Field)
			assert.Equal(t, tt.wantCode, fieldErr.Code)
			assert.NotEmpty(t, fieldErr.Detail)
			assert.Equal(t, expected, cfg, "Validate() mutated config")
		})
	}
}
