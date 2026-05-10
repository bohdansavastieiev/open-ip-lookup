package update

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/source"
)

var errUnknownTimestampSource = errors.New("unknown timestamp source")

type sourceTimestampSpec struct {
	skipLines int
	prefix    string
	parse     func(string) (time.Time, error)
}

var timestampSpecs = map[source.ID]sourceTimestampSpec{
	source.CymruFullBogonsIPv4: {
		skipLines: 0,
		prefix:    "# last updated ",
		parse:     parseCymruTimestamp,
	},
	source.CymruFullBogonsIPv6: {
		skipLines: 0,
		prefix:    "# last updated ",
		parse:     parseCymruTimestamp,
	},
	source.Az0VPNIP:       {skipLines: 0, prefix: "# ", parse: parseAz0Timestamp},
	source.Az0VPNHostname: {skipLines: 1, prefix: "# Updated ", parse: parseAz0Timestamp},
	source.AvastelBotIPsLists1Day: {
		skipLines: 1,
		prefix:    "# Last update: ",
		parse:     parseAvastelTimestampRFC3339,
	},
	source.AvastelBotIPsLists5Day: {
		skipLines: 1,
		prefix:    "# Last update: ",
		parse:     parseAvastelTimestampCustom,
	},
	source.AvastelBotIPsLists8Day: {
		skipLines: 1,
		prefix:    "# Last update: ",
		parse:     parseAvastelTimestampCustom,
	},
}

func extractUpdatedAt(sourceID source.ID, path string) (time.Time, error) {
	spec, ok := timestampSpecs[sourceID]
	if !ok {
		return time.Time{}, fmt.Errorf("%w: %q", errUnknownTimestampSource, sourceID)
	}

	f, err := os.Open(path)
	if err != nil {
		return time.Time{}, fmt.Errorf("open file %q: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	br := bufio.NewReader(f)
	for i := range spec.skipLines {
		if _, err := br.ReadString('\n'); err != nil {
			return time.Time{}, fmt.Errorf("skip line %d in %q: %w", i+1, path, err)
		}
	}

	line, err := br.ReadString('\n')
	if err != nil {
		return time.Time{}, fmt.Errorf("read line in %q: %w", path, err)
	}
	line = strings.TrimSpace(line)

	rest, ok := strings.CutPrefix(line, spec.prefix)
	if !ok {
		return time.Time{}, fmt.Errorf(
			"invalid timestamp header in %q: expected prefix %q, got %q",
			path,
			spec.prefix,
			line,
		)
	}

	t, err := spec.parse(rest)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse timestamp for %q: %w", sourceID, err)
	}
	return t, nil
}

func parseCymruTimestamp(line string) (time.Time, error) {
	unixPart, _, _ := strings.Cut(line, " ")
	unixSeconds, err := strconv.ParseInt(unixPart, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse unix timestamp %q: %w", line, err)
	}
	return time.Unix(unixSeconds, 0).UTC(), nil
}

func parseAz0Timestamp(line string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, line)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse RFC3339 timestamp %q: %w", line, err)
	}
	return t.UTC(), nil
}

func parseAvastelTimestampRFC3339(line string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, line)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse RFC3339Nano timestamp %q: %w", line, err)
	}
	return t.UTC(), nil
}

func parseAvastelTimestampCustom(line string) (time.Time, error) {
	const layout = "2006-01-02 15:04:05.999999"
	t, err := time.ParseInLocation(layout, line, time.UTC)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse custom timestamp %q: %w", line, err)
	}
	return t.UTC(), nil
}
