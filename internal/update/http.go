package update

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/source"
)

var errUnexpectedResponse = errors.New("request source unexpected status")
var errMissingFreshnessMetadata = errors.New("source response missing freshness metadata")

type sourceUpdateResult struct {
	state          sourceState
	success        bool
	changed        bool
	retryable      bool
	shortRetryable bool
	tempPath       string
}

func refreshHTTPSource(
	ctx context.Context,
	client *http.Client,
	definition source.Definition,
	savedState sourceState,
	tempDir string,
) (sourceUpdateResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, definition.URL, nil)
	if err != nil {
		return sourceUpdateResult{}, fmt.Errorf("build request for %q: %w", definition.ID, err)
	}
	if err := setHTTPRequestAuth(req, definition); err != nil {
		return handleConfigError(savedState, time.Now().UTC(), err), nil
	}
	if savedState.HasLocalArtifact {
		setNotModifiedPreconditionHeader(req.Header, savedState.ETag, savedState.LastModified)
	}

	checksum, err := fetchExpectedChecksum(ctx, client, definition)
	if err != nil {
		return handleChecksumError(savedState, time.Now().UTC(), err), nil
	}

	resp, err := client.Do(req)
	if err != nil {
		return handleRequestError(savedState, time.Now().UTC(), err), nil
	}
	defer func() { _ = resp.Body.Close() }()

	completedAt := time.Now().UTC()
	switch resp.StatusCode {
	case http.StatusNotModified:
		return handleNotModifiedResponse(savedState, completedAt), nil
	case http.StatusOK:
		return handleOKResponse(savedState, completedAt, resp, tempDir, definition, checksum)
	default:
		return handleUnexpectedResponse(savedState, completedAt, resp.StatusCode), nil
	}
}

func handleRequestError(
	savedState sourceState,
	completedAt time.Time,
	err error,
) sourceUpdateResult {
	return sourceUpdateResult{
		state:          newUnsuccessfulSourceState(savedState, completedAt, errorKindNetwork, true, err),
		success:        false,
		changed:        false,
		retryable:      true,
		shortRetryable: true,
	}
}

func handleNotModifiedResponse(
	savedState sourceState,
	completedAt time.Time,
) sourceUpdateResult {
	return sourceUpdateResult{
		state:     newNotModifiedSourceState(savedState, completedAt),
		success:   true,
		changed:   false,
		retryable: false,
	}
}

func handleOKResponse(
	savedState sourceState,
	completedAt time.Time,
	resp *http.Response,
	tempDir string,
	definition source.Definition,
	checksum []byte,
) (sourceUpdateResult, error) {
	var tempPath string
	var err error
	if definition.AuthKind == source.AuthKindMaxMind {
		tempPath, err = writeTempMaxMindArtifact(resp.Body, tempDir, definition, checksum)
	} else {
		tempPath, err = writeTempHTTPArtifact(resp.Body, tempDir, definition)
	}
	if err != nil {
		if errors.Is(err, errInvalidArchive) ||
			errors.Is(err, errMaxMindChecksumMismatch) ||
			errors.Is(err, errArtifactTooLarge) {
			return handleContentError(savedState, completedAt, err), nil
		}
		if isDownloadError(err) {
			return handleRequestError(savedState, completedAt, err), nil
		}
		return sourceUpdateResult{}, err
	}
	if err := validateHTTPArtifact(definition, tempPath); err != nil {
		_ = os.RemoveAll(tempPath)
		return handleContentError(savedState, completedAt, err), nil
	}

	state, err := newOKSourceState(savedState, resp.Header, completedAt)
	if err != nil {
		_ = os.RemoveAll(tempPath)
		return handleFreshnessError(savedState, completedAt, err), nil
	}
	if err := validateFreshnessMetadata(definition, state); err != nil {
		_ = os.RemoveAll(tempPath)
		return handleFreshnessError(savedState, completedAt, err), nil
	}

	return sourceUpdateResult{
		state:     state,
		changed:   true,
		success:   true,
		retryable: false,
		tempPath:  tempPath,
	}, nil
}

func writeTempHTTPArtifact(
	body io.Reader,
	tempDir string,
	definition source.Definition,
) (string, error) {
	switch definition.ArtifactKind {
	case source.ArtifactKindDirectFile:
		return writeTempSource(body, tempDir, definition.LocalBaseName)
	case source.ArtifactKindTarGzDir:
		return writeTempTarGzDir(body, tempDir, definition.LocalBaseName)
	default:
		return "", fmt.Errorf("unsupported source artifact kind: %q", definition.ID)
	}
}

func handleChecksumError(
	savedState sourceState,
	completedAt time.Time,
	err error,
) sourceUpdateResult {
	if errors.Is(err, errMissingMaxMindCredentials) {
		return handleConfigError(savedState, completedAt, err)
	}
	if errors.Is(err, errInvalidMaxMindChecksum) {
		return handleContentError(savedState, completedAt, err)
	}
	return handleRequestError(savedState, completedAt, err)
}

func isDownloadError(err error) bool {
	if errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}

	var netErr net.Error
	return errors.As(err, &netErr)
}

func handleConfigError(
	savedState sourceState,
	completedAt time.Time,
	err error,
) sourceUpdateResult {
	return sourceUpdateResult{
		state: newUnsuccessfulSourceState(
			savedState,
			completedAt,
			errorKindConfig,
			false,
			err,
		),
		success:   false,
		changed:   false,
		retryable: false,
	}
}

func handleFreshnessError(
	savedState sourceState,
	completedAt time.Time,
	err error,
) sourceUpdateResult {
	return sourceUpdateResult{
		state: newUnsuccessfulSourceState(
			savedState,
			completedAt,
			errorKindFreshness,
			false,
			err,
		),
		success:   false,
		changed:   false,
		retryable: false,
	}
}

func handleContentError(
	savedState sourceState,
	completedAt time.Time,
	err error,
) sourceUpdateResult {
	return sourceUpdateResult{
		state: newUnsuccessfulSourceState(
			savedState,
			completedAt,
			errorKindContent,
			true,
			err,
		),
		success:        false,
		changed:        false,
		retryable:      true,
		shortRetryable: false,
	}
}

func validateFreshnessMetadata(definition source.Definition, state sourceState) error {
	switch definition.FreshnessKind {
	case source.FreshnessKindETag:
		if state.ETag != "" {
			return nil
		}
	case source.FreshnessKindLastModified:
		if !state.LastModified.IsZero() {
			return nil
		}
	}

	return fmt.Errorf("%w: %q", errMissingFreshnessMetadata, definition.ID)
}

func handleUnexpectedResponse(
	savedState sourceState,
	completedAt time.Time,
	statusCode int,
) sourceUpdateResult {
	err := fmt.Errorf("%w: status %d", errUnexpectedResponse, statusCode)
	return sourceUpdateResult{
		state: newUnsuccessfulSourceState(
			savedState,
			completedAt,
			errorKindHTTPStatus,
			isRetryableHTTPStatus(statusCode),
			err,
		),
		success:        false,
		changed:        false,
		retryable:      isRetryableHTTPStatus(statusCode),
		shortRetryable: isRetryableHTTPStatus(statusCode),
	}
}

func isRetryableHTTPStatus(statusCode int) bool {
	return statusCode == http.StatusRequestTimeout ||
		statusCode == http.StatusTooManyRequests ||
		statusCode >= http.StatusInternalServerError
}

func setNotModifiedPreconditionHeader(headers http.Header, etag string, lastModified time.Time) {
	if etag != "" {
		headers.Set("If-None-Match", etag)
		return
	}
	if !lastModified.IsZero() {
		headers.Set("If-Modified-Since", lastModified.UTC().Format(http.TimeFormat))
	}
}
