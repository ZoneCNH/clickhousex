package clickhousex

import (
	"errors"
	"time"

	"github.com/ZoneCNH/clickhousex/internal/sanitize"
	"github.com/ZoneCNH/clickhousex/internal/validation"
)

const (
	DefaultPort         = 9000
	DefaultMaxOpenConns = 10
	DefaultMaxIdleConns = 5
	DefaultTimeout      = 30 * time.Second
)

// Config holds the ClickHouse connection configuration.
type Config struct {
	Name           string
	Host           string
	Port           int
	Database       string
	Username       string
	Password       string
	MaxOpenConns   int
	MaxIdleConns   int
	ConnMaxLifetime time.Duration
	Timeout        time.Duration
}

// SanitizedConfig is the safe-to-log version of Config with secrets masked.
type SanitizedConfig struct {
	Name           string
	Host           string
	Port           int
	Database       string
	Username       string
	Password       string
	MaxOpenConns   int
	MaxIdleConns   int
	ConnMaxLifetime time.Duration
	Timeout        time.Duration
}

// Validate checks the configuration for required fields and valid values.
func (c Config) Validate() error {
	if err := validation.RequireNonEmpty("name", c.Name); err != nil {
		return validationError("Config.Validate", err.Error(), err)
	}
	if err := validation.RequireNonEmpty("host", c.Host); err != nil {
		return validationError("Config.Validate", err.Error(), err)
	}
	if c.Port < 0 {
		err := errors.New("port must not be negative")
		return validationError("Config.Validate", err.Error(), err)
	}
	if c.MaxOpenConns < 0 {
		err := errors.New("max_open_conns must not be negative")
		return validationError("Config.Validate", err.Error(), err)
	}
	if c.MaxIdleConns < 0 {
		err := errors.New("max_idle_conns must not be negative")
		return validationError("Config.Validate", err.Error(), err)
	}
	if c.ConnMaxLifetime < 0 {
		err := errors.New("conn_max_lifetime must not be negative")
		return validationError("Config.Validate", err.Error(), err)
	}
	if c.Timeout < 0 {
		err := errors.New("timeout must not be negative")
		return validationError("Config.Validate", err.Error(), err)
	}
	return nil
}

// Sanitize returns a copy of the config with secrets masked.
func (c Config) Sanitize() SanitizedConfig {
	return SanitizedConfig{
		Name:           c.Name,
		Host:           c.Host,
		Port:           c.Port,
		Database:       c.Database,
		Username:       c.Username,
		Password:       sanitize.Secret(c.Password),
		MaxOpenConns:   c.MaxOpenConns,
		MaxIdleConns:   c.MaxIdleConns,
		ConnMaxLifetime: c.ConnMaxLifetime,
		Timeout:        c.Timeout,
	}
}
