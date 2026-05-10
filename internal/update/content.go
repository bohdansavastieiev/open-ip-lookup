package update

import (
	"bufio"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strings"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/source"
)

var errInvalidDanIPList = errors.New("dan response is not an IP list")

func validateHTTPArtifact(definition source.Definition, path string) error {
	switch definition.ID {
	case source.DanTorExit, source.DanTorFull:
		return validateDanIPList(path)
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
