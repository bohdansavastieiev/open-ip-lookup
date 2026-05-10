package config

import (
	"errors"
	"fmt"
	"net"
	"strings"
)

type ServerConfig struct {
	Addr                     string `json:"addr"`
	ReadHeaderTimeoutSeconds int    `json:"read_header_timeout_seconds"`
	ReadTimeoutSeconds       int    `json:"read_timeout_seconds"`
	WriteTimeoutSeconds      int    `json:"write_timeout_seconds"`
	IdleTimeoutSeconds       int    `json:"idle_timeout_seconds"`
	ShutdownTimeoutSeconds   int    `json:"shutdown_timeout_seconds"`
}

const (
	minTimeoutSeconds = 1
	maxTimeoutSeconds = 300
)

func (c ServerConfig) validate() error {
	var errs []error

	if err := validateAddr(c.Addr); err != nil {
		errs = append(errs, err)
	}

	if err := validateTimeoutRange(
		"server.read_header_timeout_seconds",
		c.ReadHeaderTimeoutSeconds); err != nil {
		errs = append(errs, err)
	}

	if err := validateTimeoutRange(
		"server.read_timeout_seconds",
		c.ReadTimeoutSeconds); err != nil {
		errs = append(errs, err)
	}

	if err := validateTimeoutRange(
		"server.write_timeout_seconds",
		c.WriteTimeoutSeconds); err != nil {
		errs = append(errs, err)
	}

	if err := validateTimeoutRange(
		"server.idle_timeout_seconds",
		c.IdleTimeoutSeconds); err != nil {
		errs = append(errs, err)
	}

	if err := validateTimeoutRange(
		"server.shutdown_timeout_seconds",
		c.ShutdownTimeoutSeconds); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

func validateAddr(addr string) error {
	if strings.TrimSpace(addr) != addr {
		return New("server.addr", CodeWhitespace, "must not have leading or trailing whitespace", addr)
	}

	if addr == "" {
		return New("server.addr", CodeRequired, "must be set", addr)
	}

	if _, _, err := net.SplitHostPort(addr); err != nil {
		return New("server.addr", CodeInvalid, "must be a valid host:port", addr)
	}

	return nil
}

func validateTimeoutRange(field string, timeoutSeconds int) error {
	if timeoutSeconds < minTimeoutSeconds || timeoutSeconds > maxTimeoutSeconds {
		detail := fmt.Sprintf("must be between %d and %d seconds", minTimeoutSeconds, maxTimeoutSeconds)
		return New(field, CodeRange, detail, timeoutSeconds)
	}

	return nil
}
