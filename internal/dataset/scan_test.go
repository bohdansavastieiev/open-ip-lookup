package dataset

import (
	"encoding/csv"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testJSONValue struct {
	Value int `json:"value"`
}

type testCSVValueRecord struct {
	Value string `csv:"value"`
}

type testCSVPairRecord struct {
	Value string `csv:"value"`
	Note  string `csv:"note"`
}

func writeTempScanFile(t *testing.T, name string, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestNewBOMStrippedReader_StripsUTF8BOM(t *testing.T) {
	br, err := newBOMStrippedReader(strings.NewReader("\uFEFFhello"))
	require.NoError(t, err)

	data, err := io.ReadAll(br)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(data))
}

func TestNewBOMStrippedReader_LeavesNonBOMUntouched(t *testing.T) {
	br, err := newBOMStrippedReader(strings.NewReader("hello"))
	require.NoError(t, err)

	data, err := io.ReadAll(br)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(data))
}

func TestScanTextSource_StripsUTF8BOM(t *testing.T) {
	path := writeTempScanFile(t, "source.txt", "\uFEFF# header\nvalue\n")

	var lines []string
	acceptedCount, err := scanTextSource(path, 1, func(line string) (bool, error) {
		lines = append(lines, line)
		return true, nil
	})

	require.NoError(t, err)
	assert.Equal(t, 1, acceptedCount)
	assert.Equal(t, []string{"value"}, lines)
}

func TestScanTextBody_ReturnsErrorForEmptyTrimmedLine(t *testing.T) {
	handledLines := 0

	acceptedCount, err := scanTextBody(
		strings.NewReader("first\n \nthird\n"),
		"source.txt",
		func(line string) (bool, error) {
			handledLines++
			return true, nil
		},
	)

	require.Error(t, err)
	assert.Zero(t, acceptedCount)
	assert.Equal(t, 1, handledLines)
}

func TestScanCSVSource_StripsUTF8BOM(t *testing.T) {
	path := writeTempScanFile(t, "source.csv", "\uFEFF# header\nvalue\nok\n")

	var values []string
	acceptedCount, err := scanCSVSource(path, 1, nil, func(record testCSVValueRecord) (bool, error) {
		values = append(values, record.Value)
		return true, nil
	})

	require.NoError(t, err)
	assert.Equal(t, 1, acceptedCount)
	assert.Equal(t, []string{"ok"}, values)
}

func TestScanCSVBody_UsesConfigure(t *testing.T) {
	var records []testCSVPairRecord
	acceptedCount, err := scanCSVBody(
		strings.NewReader("value;note\nfirst;ok\n"),
		"source.csv",
		func(r *csv.Reader) {
			r.Comma = ';'
		},
		func(record testCSVPairRecord) (bool, error) {
			records = append(records, record)
			return true, nil
		},
	)

	require.NoError(t, err)
	assert.Equal(t, 1, acceptedCount)
	assert.Equal(t, []testCSVPairRecord{{Value: "first", Note: "ok"}}, records)
}

func TestScanCSVBody_ReturnsErrorWhenHandlerFails(t *testing.T) {
	_, err := scanCSVBody(
		strings.NewReader("value\nfirst\nsecond\n"),
		"source.csv",
		nil,
		func(record testCSVValueRecord) (bool, error) {
			if record.Value == "second" {
				return false, io.ErrUnexpectedEOF
			}

			return true, nil
		},
	)

	require.Error(t, err)
}

func TestDecodeJSON_ReturnsErrorForTrailingToken(t *testing.T) {
	var dst testJSONValue

	err := decodeJSON(strings.NewReader(`{"value":1} true`), &dst)

	require.Error(t, err)
}

func TestDecodeJSON_StripsUTF8BOM(t *testing.T) {
	var dst testJSONValue

	err := decodeJSON(strings.NewReader("\uFEFF{\"value\":1}"), &dst)

	require.NoError(t, err)
	assert.Equal(t, testJSONValue{Value: 1}, dst)
}

func TestDecodeJSONArray_StripsUTF8BOM(t *testing.T) {
	var values []int
	count, err := decodeJSONArray(
		strings.NewReader("\uFEFF[{\"value\":1},{\"value\":2}]"),
		func(record testJSONValue) error {
			values = append(values, record.Value)
			return nil
		},
	)

	require.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.Equal(t, []int{1, 2}, values)
}

func TestDecodeJSONArray_ReturnsErrorWhenRootIsNotArray(t *testing.T) {
	count, err := decodeJSONArray(strings.NewReader(`{"value":1}`), func(record testJSONValue) error {
		return nil
	})

	require.Error(t, err)
	assert.Zero(t, count)
}

func TestDecodeJSONArray_ReturnsErrorForTrailingToken(t *testing.T) {
	handled := 0

	count, err := decodeJSONArray(
		strings.NewReader(`[{"value":1}] true`),
		func(record testJSONValue) error {
			handled++
			return nil
		},
	)

	require.Error(t, err)
	assert.Zero(t, count)
	assert.Equal(t, 1, handled)
}
