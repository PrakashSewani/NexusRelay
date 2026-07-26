package dependency

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/PrakashSewani/NexusRelay/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
	redis "github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
)

type Clients struct {
	PostgreSQL *pgxpool.Pool
	Redis      *redis.Client
}

func Open(ctx context.Context, database config.Database, redisConfig config.Redis, probeTimeout time.Duration) (*Clients, error) {
	postgresConfig, err := postgresPoolConfig(database, probeTimeout)
	if err != nil {
		return nil, errors.New("configure PostgreSQL client")
	}
	postgres, err := pgxpool.NewWithConfig(ctx, postgresConfig)
	if err != nil {
		return nil, errors.New("configure PostgreSQL client")
	}

	redisOptions, err := redisOptions(redisConfig, probeTimeout)
	if err != nil {
		postgres.Close()
		return nil, errors.New("configure Redis client")
	}

	return &Clients{PostgreSQL: postgres, Redis: redis.NewClient(redisOptions)}, nil
}

func (c *Clients) Close() {
	if c == nil {
		return
	}
	if c.Redis != nil {
		_ = c.Redis.Close()
	}
	if c.PostgreSQL != nil {
		c.PostgreSQL.Close()
	}
}

func (c *Clients) Probes(observe func(string, time.Duration, error)) []Probe {
	return []Probe{
		{Name: "postgresql", Ping: c.PostgreSQL.Ping, Observe: observe},
		{Name: "redis", Ping: func(ctx context.Context) error { return c.Redis.Ping(ctx).Err() }, Observe: observe},
	}
}

func postgresPoolConfig(database config.Database, probeTimeout time.Duration) (*pgxpool.Config, error) {
	connectionURL := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(database.User, database.Password.Reveal()),
		Host:   net.JoinHostPort(database.Host, strconv.Itoa(database.Port)),
		Path:   database.Name,
	}
	query := connectionURL.Query()
	query.Set("sslmode", database.SSLMode)
	connectionURL.RawQuery = query.Encode()

	poolConfig, err := pgxpool.ParseConfig(connectionURL.String())
	if err != nil {
		return nil, err
	}
	poolConfig.MinConns = int32(database.MinConnections)
	poolConfig.MaxConns = int32(database.MaxConnections)
	poolConfig.ConnConfig.ConnectTimeout = probeTimeout
	return poolConfig, nil
}

func redisOptions(redisConfig config.Redis, probeTimeout time.Duration) (*redis.Options, error) {
	options, err := redis.ParseURL(redisConfig.URL.Reveal())
	if err != nil {
		return nil, err
	}
	if options.DB != 0 {
		return nil, fmt.Errorf("redis database must be zero")
	}

	// Phase 2 Redis allows only AUTH and PING. RESP2 and these disabled client
	// features prevent HELLO, CLIENT SETINFO, and maintenance commands.
	options.Protocol = 2
	options.DisableIdentity = true
	options.MaintNotificationsConfig = &maintnotifications.Config{Mode: maintnotifications.ModeDisabled}
	options.MaxRetries = -1
	options.DialerRetries = 1
	options.DialTimeout = probeTimeout
	options.ReadTimeout = probeTimeout
	options.WriteTimeout = probeTimeout
	options.ContextTimeoutEnabled = true
	options.PoolSize = redisConfig.MaxConnections
	options.MaxActiveConns = redisConfig.MaxConnections
	return options, nil
}
