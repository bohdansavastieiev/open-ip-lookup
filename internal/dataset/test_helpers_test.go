package dataset

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func datasetTestdataDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)

	return filepath.Join(filepath.Dir(file), "testdata")
}

func testdataPath(t *testing.T, localPath string) string {
	t.Helper()

	return filepath.Join(datasetTestdataDir(t), filepath.FromSlash(localPath))
}
