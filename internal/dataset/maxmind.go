package dataset

import (
	"errors"
	"fmt"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/source"
	"github.com/oschwald/geoip2-golang/v2"
)

type maxmindDBType string

const (
	maxmindDBTypeCity maxmindDBType = "GeoLite2-City"
	maxmindDBTypeASN  maxmindDBType = "GeoLite2-ASN"
)

var (
	errMaxMindDBTypeMismatch = errors.New("unexpected MaxMind database type")
	errMaxMindReaderType     = errors.New("unexpected MaxMind reader type")
)

func loadMaxMindGeoReader(path string, dbType maxmindDBType) (*geoip2.Reader, error) {
	reader, err := geoip2.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open MMDB source %q: %w", path, err)
	}

	if reader.Metadata().DatabaseType != string(dbType) {
		_ = reader.Close()
		return nil, fmt.Errorf(
			"%w: expected %s database, got %q",
			errMaxMindDBTypeMismatch,
			dbType,
			reader.Metadata().DatabaseType,
		)
	}

	return reader, nil
}

func extractReader(snap snapshot, sourceID source.ID) (*geoip2.Reader, error) {
	data, ok := snap[sourceID]
	if !ok {
		return nil, nil
	}
	reader, ok := data.(*geoip2.Reader)
	if !ok {
		return nil, fmt.Errorf(
			"%w: expected source %q data to be *geoip2.Reader, got %T",
			errMaxMindReaderType,
			sourceID,
			data,
		)
	}
	return reader, nil
}
