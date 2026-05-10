package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeValidAppConfig() Config {
	return Config{
		Server:  makeValidServerConfig(),
		Sources: makeValidSourcesConfig(),
	}
}

func writeConfigFile(t *testing.T, content []byte) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(path, content, 0o600))

	return path
}

func writeJSONConfigFile(t *testing.T, cfg Config) string {
	t.Helper()

	data, err := json.Marshal(cfg)
	require.NoError(t, err)

	return writeConfigFile(t, data)
}

func TestLoad_ReturnsConfigForValidFile(t *testing.T) {
	expected := makeValidAppConfig()
	path := writeJSONConfigFile(t, expected)

	actual, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestLoad_ReturnsErrorForMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "missing-config.json"))
	require.Error(t, err)

	var pathErr *os.PathError
	require.ErrorAs(t, err, &pathErr)
}

func TestLoad_ReturnsErrorForInvalidJSON(t *testing.T) {
	path := writeConfigFile(t, []byte("{"))

	_, err := Load(path)
	require.Error(t, err)
}

func TestLoad_ReturnsErrorForUnknownJSONField(t *testing.T) {
	data, err := json.Marshal(makeValidAppConfig())
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(data, &payload))

	serverPayload, ok := payload["server"].(map[string]any)
	require.True(t, ok)
	serverPayload["extra"] = true

	data, err = json.Marshal(payload)
	require.NoError(t, err)

	path := writeConfigFile(t, data)

	_, err = Load(path)
	require.Error(t, err)
}

func TestLoad_ReturnsErrorForTrailingJSONValue(t *testing.T) {
	data, err := json.Marshal(makeValidAppConfig())
	require.NoError(t, err)

	data = append(data, []byte("\n{\"extra\":true}")...)
	path := writeConfigFile(t, data)

	_, err = Load(path)
	require.Error(t, err)
}

func TestLoad_ReturnsFieldErrorForInvalidServerConfiguration(t *testing.T) {
	cfg := makeValidAppConfig()
	cfg.Server.ReadTimeoutSeconds = minTimeoutSeconds - 1
	path := writeJSONConfigFile(t, cfg)

	_, err := Load(path)
	require.Error(t, err)

	var fieldErr *FieldError
	require.ErrorAs(t, err, &fieldErr)
	assert.Equal(t, "server.read_timeout_seconds", fieldErr.Field)
	assert.Equal(t, CodeRange, fieldErr.Code)
	assert.NotEmpty(t, fieldErr.Detail)
}

func TestLoad_ReturnsFieldErrorForInvalidSourcesConfiguration(t *testing.T) {
	cfg := makeValidAppConfig()
	cfg.Sources.Enabled[0] = "unsupported_id"
	path := writeJSONConfigFile(t, cfg)

	_, err := Load(path)
	require.Error(t, err)

	var fieldErr *FieldError
	require.ErrorAs(t, err, &fieldErr)
	assert.Equal(t, "sources.enabled[0]", fieldErr.Field)
	assert.Equal(t, CodeUnsupported, fieldErr.Code)
	assert.NotEmpty(t, fieldErr.Detail)
}

func TestLoad_AllowsMissingSourcesEnabled(t *testing.T) {
	data, err := json.Marshal(makeValidAppConfig())
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(data, &payload))

	sourcesPayload, ok := payload["sources"].(map[string]any)
	require.True(t, ok)

	delete(sourcesPayload, "enabled")

	data, err = json.Marshal(payload)
	require.NoError(t, err)

	path := writeConfigFile(t, data)

	actual, err := Load(path)
	require.NoError(t, err)
	assert.Nil(t, actual.Sources.Enabled)
}
