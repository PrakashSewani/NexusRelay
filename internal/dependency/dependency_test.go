package dependency

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PrakashSewani/NexusRelay/internal/config"
	redis "github.com/redis/go-redis/v9"
)

func TestDefaultReadinessPolicy(t *testing.T) {
	policy := DefaultReadinessPolicy()
	if policy.StartupTimeout != 60*time.Second || policy.ProbeTimeout != 3*time.Second || policy.ProbeInterval != 5*time.Second || policy.RetryMinimum != 100*time.Millisecond || policy.RetryMaximum != 2*time.Second {
		t.Fatalf("unexpected defaults: %+v", policy)
	}
}

func TestRedisOptionsSupportPingOnlyACL(t *testing.T) {
	options, err := redisOptions(config.Redis{URL: readSecret(t, "redis://:password@redis:6379/0"), MaxConnections: 7}, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if options.Protocol != 2 || !options.DisableIdentity || options.DB != 0 || options.PoolSize != 7 || options.MaxActiveConns != 7 || !options.ContextTimeoutEnabled {
		t.Fatalf("unexpected Redis options: %+v", options)
	}
	if options.MaintNotificationsConfig == nil || options.MaintNotificationsConfig.Mode != "disabled" {
		t.Fatalf("maintenance notifications not disabled: %+v", options.MaintNotificationsConfig)
	}
}

func TestRedisOptionsRejectNonzeroDatabaseWithoutDisclosingURL(t *testing.T) {
	const sentinel = "do-not-disclose"
	_, err := redisOptions(config.Redis{URL: readSecret(t, "redis://:"+sentinel+"@redis:6379/1"), MaxConnections: 1}, time.Second)
	if err == nil || strings.Contains(err.Error(), sentinel) {
		t.Fatalf("error = %v", err)
	}
}

func TestPostgresPoolConfigUsesTypedConnectionSettings(t *testing.T) {
	database := config.Database{
		Host: "2001:db8::1", Port: 5432, Name: "nexusrelay", User: "nexusrelay_gateway",
		Password: readSecret(t, "password with reserved:/? characters"), SSLMode: "require",
		MinConnections: 2, MaxConnections: 9,
	}
	poolConfig, err := postgresPoolConfig(database, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if poolConfig.ConnConfig.Host != database.Host || poolConfig.ConnConfig.Port != uint16(database.Port) || poolConfig.ConnConfig.Database != database.Name || poolConfig.ConnConfig.User != database.User || poolConfig.ConnConfig.Password != database.Password.Reveal() {
		t.Fatalf("unexpected PostgreSQL connection config")
	}
	if poolConfig.MinConns != 2 || poolConfig.MaxConns != 9 || poolConfig.ConnConfig.ConnectTimeout != 3*time.Second {
		t.Fatalf("unexpected PostgreSQL pool config: min=%d max=%d timeout=%s", poolConfig.MinConns, poolConfig.MaxConns, poolConfig.ConnConfig.ConnectTimeout)
	}
}

func TestRunRetriesStartupAndTracksContinuousReadiness(t *testing.T) {
	policy := ReadinessPolicy{StartupTimeout: time.Second, ProbeTimeout: 50 * time.Millisecond, ProbeInterval: 10 * time.Millisecond, RetryMinimum: time.Millisecond, RetryMaximum: 4 * time.Millisecond}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mutex sync.Mutex
	ready := false
	transitions := make(chan bool, 20)
	setReady := func(value bool) {
		mutex.Lock()
		ready = value
		mutex.Unlock()
		transitions <- value
	}
	var attempts int
	probeHealthy := true
	probe := Probe{Name: "postgresql", Ping: func(context.Context) error {
		mutex.Lock()
		defer mutex.Unlock()
		attempts++
		if attempts < 3 || !probeHealthy {
			return errors.New("unavailable")
		}
		return nil
	}}
	serveStarted := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		result <- Run(ctx, policy, []Probe{probe}, setReady, func(ctx context.Context) error {
			close(serveStarted)
			<-ctx.Done()
			return nil
		})
	}()
	<-serveStarted
	waitTransition(t, transitions, true)

	mutex.Lock()
	probeHealthy = false
	mutex.Unlock()
	waitTransition(t, transitions, false)
	mutex.Lock()
	probeHealthy = true
	mutex.Unlock()
	waitTransition(t, transitions, true)

	cancel()
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	mutex.Lock()
	defer mutex.Unlock()
	if ready {
		t.Fatal("readiness remained true after shutdown")
	}
}

func TestRunStartupTimeoutIsSanitizedAndStopsServer(t *testing.T) {
	const sentinel = "secret-driver-error"
	policy := ReadinessPolicy{StartupTimeout: 25 * time.Millisecond, ProbeTimeout: 5 * time.Millisecond, ProbeInterval: time.Second, RetryMinimum: time.Millisecond, RetryMaximum: 2 * time.Millisecond}
	serverStopped := make(chan struct{})
	err := Run(context.Background(), policy, []Probe{{Name: "redis", Ping: func(context.Context) error { return errors.New(sentinel) }}}, func(bool) {}, func(ctx context.Context) error {
		<-ctx.Done()
		close(serverStopped)
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "redis unavailable") || strings.Contains(err.Error(), sentinel) {
		t.Fatalf("error = %v", err)
	}
	select {
	case <-serverStopped:
	case <-time.After(time.Second):
		t.Fatal("server was not stopped")
	}
}

func TestRealRedisPingOnlyACL(t *testing.T) {
	redisURL := os.Getenv("NEXUSRELAY_TEST_REDIS_URL")
	if redisURL == "" {
		t.Skip("NEXUSRELAY_TEST_REDIS_URL is not set")
	}
	clientOptions, err := redisOptions(config.Redis{URL: readSecret(t, redisURL), MaxConnections: 2}, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(clientOptions)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close Redis client: %v", err)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("authenticated PING failed: %v", err)
	}
}

func TestRunRejectsInvalidPolicyAndProbe(t *testing.T) {
	if err := Run(context.Background(), ReadinessPolicy{}, []Probe{{Name: "x", Ping: func(context.Context) error { return nil }}}, func(bool) {}, func(context.Context) error { return nil }); err == nil {
		t.Fatal("invalid policy accepted")
	}
	if err := Run(context.Background(), DefaultReadinessPolicy(), []Probe{{Name: "x"}}, func(bool) {}, func(context.Context) error { return nil }); err == nil {
		t.Fatal("invalid probe accepted")
	}
}

func waitTransition(t *testing.T, transitions <-chan bool, want bool) {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		select {
		case got := <-transitions:
			if got == want {
				return
			}
		case <-deadline:
			t.Fatalf("readiness did not transition to %t", want)
		}
	}
}

func readSecret(t *testing.T, value string) config.Secret {
	t.Helper()
	path := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
	secret, err := config.ReadSimpleSecret("TEST_SECRET_FILE", path, 4096)
	if err != nil {
		t.Fatal(err)
	}
	return secret
}
