package clickhousex

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ZoneCNH/clickhousex/internal/sanitize"
	"github.com/ZoneCNH/clickhousex/internal/validation"
)

const (
	DefaultPort            = 9000
	DefaultMaxOpenConns    = 10
	DefaultMaxIdleConns    = 5
	DefaultConnMaxLifetime = time.Hour
	DefaultTimeout         = 30 * time.Second
	MaxAllowedOpenConns    = 100
)

// Config holds the ClickHouse connection configuration.
type Config struct {
	Name            string
	DSN             string
	Host            string
	Port            int
	Database        string
	Username        string
	Password        string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	Timeout         time.Duration
}

// SanitizedConfig is the safe-to-log version of Config with secrets masked.
type SanitizedConfig struct {
	Name            string
	DSN             string
	Host            string
	Port            int
	Database        string
	Username        string
	Password        string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	Timeout         time.Duration
}

// Validate checks the configuration for required fields and valid values.
func (c Config) Validate() error {
	cfg, err := c.withDefaults()
	if err != nil {
		return err
	}
	if err := validation.RequireNonEmpty("name", c.Name); err != nil {
		return validationError("Config.Validate", err.Error(), err)
	}
	if cfg.DSN == "" {
		if err := validation.RequireNonEmpty("host", cfg.Host); err != nil {
			return validationError("Config.Validate", err.Error(), err)
		}
	}
	if cfg.Port < 0 {
		err := errors.New("port must not be negative")
		return validationError("Config.Validate", err.Error(), err)
	}
	if cfg.MaxOpenConns < 0 {
		err := errors.New("max_open_conns must not be negative")
		return validationError("Config.Validate", err.Error(), err)
	}
	if cfg.MaxOpenConns > MaxAllowedOpenConns {
		err := errors.New("max_open_conns must not exceed 100")
		return validationError("Config.Validate", err.Error(), err)
	}
	if cfg.MaxIdleConns < 0 {
		err := errors.New("max_idle_conns must not be negative")
		return validationError("Config.Validate", err.Error(), err)
	}
	if cfg.MaxIdleConns > cfg.MaxOpenConns {
		err := errors.New("max_idle_conns must not exceed max_open_conns")
		return validationError("Config.Validate", err.Error(), err)
	}
	if cfg.ConnMaxLifetime < 0 {
		err := errors.New("conn_max_lifetime must not be negative")
		return validationError("Config.Validate", err.Error(), err)
	}
	if cfg.Timeout < 0 {
		err := errors.New("timeout must not be negative")
		return validationError("Config.Validate", err.Error(), err)
	}
	return nil
}

// Sanitize returns a copy of the config with secrets masked.
func (c Config) Sanitize() SanitizedConfig {
	return SanitizedConfig{
		Name:            c.Name,
		DSN:             sanitizeDSN(c.DSN),
		Host:            c.Host,
		Port:            c.Port,
		Database:        c.Database,
		Username:        c.Username,
		Password:        sanitize.Secret(c.Password),
		MaxOpenConns:    c.MaxOpenConns,
		MaxIdleConns:    c.MaxIdleConns,
		ConnMaxLifetime: c.ConnMaxLifetime,
		Timeout:         c.Timeout,
	}
}

func (c Config) withDefaults() (Config, error) {
	if c.Port == 0 {
		c.Port = DefaultPort
	}
	if c.MaxOpenConns == 0 {
		c.MaxOpenConns = DefaultMaxOpenConns
	}
	if c.MaxIdleConns == 0 {
		c.MaxIdleConns = DefaultMaxIdleConns
	}
	if c.ConnMaxLifetime == 0 {
		c.ConnMaxLifetime = DefaultConnMaxLifetime
	}
	if c.Timeout == 0 {
		c.Timeout = DefaultTimeout
	}
	if c.DSN != "" {
		if _, err := clickhouse.ParseDSN(c.DSN); err != nil {
			return Config{}, validationError("Config.Validate", "dsn is invalid", err)
		}
	}
	return c, nil
}

func (c Config) clickhouseOptions() (*clickhouse.Options, error) {
	if c.DSN != "" {
		opt, err := clickhouse.ParseDSN(c.DSN)
		if err != nil {
			return nil, validationError("Config.clickhouseOptions", "dsn is invalid", err)
		}
		opt.MaxOpenConns = c.MaxOpenConns
		opt.MaxIdleConns = c.MaxIdleConns
		opt.ConnMaxLifetime = c.ConnMaxLifetime
		opt.DialTimeout = c.Timeout
		opt.ReadTimeout = c.Timeout
		return opt, nil
	}

	return &clickhouse.Options{
		Addr: []string{net.JoinHostPort(c.Host, strconv.Itoa(c.Port))},
		Auth: clickhouse.Auth{
			Database: c.Database,
			Username: c.Username,
			Password: c.Password,
		},
		MaxOpenConns:    c.MaxOpenConns,
		MaxIdleConns:    c.MaxIdleConns,
		ConnMaxLifetime: c.ConnMaxLifetime,
		DialTimeout:     c.Timeout,
		ReadTimeout:     c.Timeout,
	}, nil
}

func sanitizeDSN(raw string) string {
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "***"
	}
	if parsed.User != nil {
		username := parsed.User.Username()
		if _, ok := parsed.User.Password(); ok {
			parsed.User = url.UserPassword(username, "***")
		} else {
			parsed.User = url.User(username)
		}
	}
	query := parsed.Query()
	for _, key := range []string{"password", "pass", "secret", "token"} {
		if query.Has(key) {
			query.Set(key, "***")
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
