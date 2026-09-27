package update

import (
	"bufio"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strings"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/dataset"
	"github.com/bohdansavastieiev/open-ip-lookup/internal/source"
)

var errInvalidDanIPList = errors.New("dan response is not an IP list")

// validateHTTPArtifact runs before a download replaces the current file. Required sources are loaded
// in full, so a broken file never reaches the disk and the dataset can always start. The others are
// cheap to skip at load time, and parsing their large files twice would cost memory.
func validateHTTPArtifact(definition source.Definition, path string) error {
	switch {
	case definition.ID == source.DanTorExit || definition.ID == source.DanTorFull:
		return validateDanIPList(path)
	case dataset.RequiresSource(definition.ID):
		return dataset.ValidateSource(definition.ID, path)
	default:
		return nil
	}
}

func validateDanIPList(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open Dan response %q: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			return fmt.Errorf("%w: empty line %d", errInvalidDanIPList, lineNumber)
		}
		if _, err := netip.ParseAddr(line); err != nil {
			return fmt.Errorf("%w: line %d", errInvalidDanIPList, lineNumber)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan Dan response %q: %w", path, err)
	}
	if lineNumber == 0 {
		return fmt.Errorf("%w: empty response", errInvalidDanIPList)
	}
	return nil
}
