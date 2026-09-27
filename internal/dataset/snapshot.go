// Package dataset provides dataset
package dataset

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/netip"
	"path/filepath"
	"reflect"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/source"
)

type snapshot map[source.ID]any

type loadFunc func(sourceID source.ID, path string) (any, error)

type loadSourceFunc[T any] func(path string) (T, error)
type loadSourceLoggerFunc[T any] func(path string, logger *slog.Logger) (T, error)

func loadSourceEntry[T any](
	loader loadSourceFunc[T],
) func(sourceID source.ID, path string) (any, error) {
	return func(sourceID source.ID, path string) (any, error) {
		data, err := loader(path)
		if err != nil {
			return nil, fmt.Errorf("load source %q (path: %q): %w", sourceID, path, err)
		}
		return data, nil
	}
}

func loadSourceEntryWithLogger[T any](
	loader loadSourceLoggerFunc[T],
	logger *slog.Logger,
) func(sourceID source.ID, path string) (any, error) {
	return func(sourceID source.ID, path string) (any, error) {
		sourceLogger := logger.With(slog.String("source", string(sourceID)))
		data, err := loader(path, sourceLogger)
		if err != nil {
			return nil, fmt.Errorf("load source %q (path: %q): %w", sourceID, path, err)
		}
		return data, nil
	}
}

func sourceLoaders(logger *slog.Logger) map[source.ID]loadFunc {
	return map[source.ID]loadFunc{
		source.MaxMindGeoLite2City: loadSourceEntry(func(path string) (any, error) {
			return loadMaxMindGeoReader(path, maxmindDBTypeCity)
		}),
		source.MaxMindGeoLite2ASN: loadSourceEntry(func(path string) (any, error) {
			return loadMaxMindGeoReader(path, maxmindDBTypeASN)
		}),
		source.IANASpecialIPv4:              loadSourceEntry(loadIANASpecialIPv4),
		source.IANASpecialIPv6:              loadSourceEntry(loadIANASpecialIPv6),
		source.CymruFullBogonsIPv4:          loadSourceEntry(loadCymruFullbogonsIPv4),
		source.CymruFullBogonsIPv6:          loadSourceEntry(loadCymruFullbogonsIPv6),
		source.DanTorExit:                   loadSourceEntry(loadDanAddrs),
		source.DanTorFull:                   loadSourceEntry(loadDanAddrs),
		source.X4bnetListsVPNVPNIPv4:        loadSourceEntry(loadX4bnetPrefixes),
		source.X4bnetListsVPNDatacenterIPv4: loadSourceEntry(loadX4bnetPrefixes),
		source.TobilgCloudProviderRanges:    loadSourceEntry(loadTobilgCloud),
		source.RezmossCloudProviders:        loadSourceEntry(loadRezmossAllProviders),
		source.Az0VPNIP:                     loadSourceEntry(loadAz0VPNIP),
		source.Az0VPNHostname:               loadSourceEntry(loadAz0VPNHostname),
		source.AvastelBotIPsLists1Day: loadSourceEntryWithLogger(
			loadAvastelInfoByAddr,
			logger,
		),
		source.AvastelBotIPsLists5Day: loadSourceEntryWithLogger(
			loadAvastelInfoByPrefix,
			logger,
		),
		source.AvastelBotIPsLists8Day: loadSourceEntryWithLogger(
			loadAvastelInfoByPrefix,
			logger,
		),
		source.BountyyfiBadASNListAll: loadSourceEntry(loadBountyyfiBadASN),
		source.UmkusIPIndexASNDCs:     loadSourceEntry(loadUmkusASNDCs),
		source.IPVerseASIPBlocksAll:   loadSourceEntry(loadIPVerseASBlocks),
		source.IPVerseASMetadataAll:   loadSourceEntry(loadIPVerseASMetadata),
	}
}

// loadSnapshot skips a source whose file fails to load and returns its error in skipped, so one
// broken upstream file does not stop the whole dataset from loading.
func loadSnapshot(
	dataDir string,
	sourceIDs []source.ID,
	logger *slog.Logger,
) (s snapshot, skipped map[source.ID]error, err error) {
	loaders := sourceLoaders(logger)
	s = make(snapshot)
	skipped = make(map[source.ID]error)
	for _, id := range sourceIDs {
		loader, ok := loaders[id]
		if !ok {
			_ = s.Close()
			return nil, nil, fmt.Errorf("unknown source: %s", id)
		}

		def, ok := source.Lookup(id)
		if !ok {
			_ = s.Close()
			return nil, nil, fmt.Errorf("unknown source definition: %s", id)
		}

		data, err := loader(id, filepath.Join(dataDir, def.LocalBaseName))
		if err != nil {
			skipped[id] = err
			continue
		}

		logLoadedSource(logger, id, data)

		s[id] = data
	}

	return s, skipped, nil
}

// ValidateSource loads one source file the same way Load does and discards the result.
func ValidateSource(id source.ID, path string) error {
	loader, ok := sourceLoaders(slog.New(slog.DiscardHandler))[id]
	if !ok {
		return fmt.Errorf("unknown source: %s", id)
	}
	data, err := loader(id, path)
	if err != nil {
		return err
	}
	if closer, ok := data.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func (s snapshot) Close() error {
	var errs []error
	for sourceID, data := range s {
		if closer, ok := data.(io.Closer); ok {
			if err := closer.Close(); err != nil {
				errs = append(errs, fmt.Errorf("close %q: %w", sourceID, err))
			}
		}
	}

	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("close snapshot: %w", err)
	}
	return nil
}

func logLoadedSource(logger *slog.Logger, sourceID source.ID, data any) {
	rv := reflect.ValueOf(data)
	var kind string
	if rv.Kind() == reflect.Map {
		kind = kindFromType(rv.Type().Key())
	} else if rv.Kind() == reflect.Slice {
		kind = kindFromType(rv.Type().Elem())
	}

	if kind != "" {
		logger.Debug("loaded source",
			slog.String("id", string(sourceID)),
			slog.String("kind", kind),
			slog.Int("count", rv.Len()))
	} else {
		logger.Debug("loaded source", slog.String("id", string(sourceID)))
	}
}

func kindFromType(t reflect.Type) string {
	switch t {
	case reflect.TypeFor[netip.Prefix]():
		return "prefixes"
	case reflect.TypeFor[netip.Addr]():
		return "addresses"
	case reflect.TypeFor[ASN]():
		return "asns"
	default:
		return ""
	}
}
