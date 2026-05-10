// Package config handles config parsing and defines config errors
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
)

type Config struct {
	Server  ServerConfig  `json:"server"`
	Sources SourcesConfig `json:"sources"`
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, err
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return Config{}, errors.New("config must contain a single JSON value")
		}

		return Config{}, err
	}

	if err := errors.Join(cfg.Server.validate(), cfg.Sources.validate()); err != nil {
		return Config{}, err
	}

	return cfg, nil
}
