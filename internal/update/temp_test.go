package update

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTempFilePattern_PreservesExtensionWhenPresent(t *testing.T) {
	assert.Equal(t, "source-*.txt", tempFilePattern("source.txt"))
	assert.Equal(t, "source-without-ext-*", tempFilePattern("source-without-ext"))
}

func TestWriteTempSource_CreatesTempFileWithExpectedContent(t *testing.T) {
	dir := t.TempDir()
	path, err := writeTempSource(strings.NewReader("body-data"), dir, "source.txt")
	require.NoError(t, err)

	assert.Equal(t, dir, filepath.Dir(path))
	assert.Contains(t, filepath.Base(path), "source-")
	assert.True(t, strings.HasSuffix(path, ".txt"))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "body-data", string(data))
}
