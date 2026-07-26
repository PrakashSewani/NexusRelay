package config

import (
	"fmt"
	"net"
	"net/mail"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	clusterAdminUser = "nexusrelay_cluster_admin"
	migrationUser    = "nexusrelay_migration"
	gatewayUser      = "nexusrelay_gateway"
	controlPlaneUser = "nexusrelay_control_plane"
	workerUser       = "nexusrelay_worker"
)

type Process string

const (
	ProcessGateway      Process = "gateway"
	ProcessControlPlane Process = "control-plane"
	ProcessWorker       Process = "worker"
	ProcessBootstrap    Process = "bootstrap"
	ProcessDeployment   Process = "deployment"
)

type SettingClass struct {
	Secret    bool
	Consumers []Process
}

var inventory = map[string]SettingClass{
	"NEXUSRELAY_ENV": {Consumers: []Process{ProcessGateway, ProcessControlPlane, ProcessWorker, ProcessBootstrap, ProcessDeployment}}, "NEXUSRELAY_VERSION": {Consumers: []Process{ProcessGateway, ProcessControlPlane, ProcessWorker, ProcessBootstrap, ProcessDeployment}},
	"PUBLIC_API_BASE_URL": {Consumers: []Process{ProcessGateway, ProcessControlPlane, ProcessDeployment}}, "ADMIN_BASE_URL": {Consumers: []Process{ProcessControlPlane, ProcessDeployment}}, "PUBLIC_API_HOST": {Consumers: []Process{ProcessDeployment}}, "ADMIN_HOST": {Consumers: []Process{ProcessDeployment}}, "ADMIN_EXPOSURE_MODE": {Consumers: []Process{ProcessDeployment}}, "ADMIN_ORIGINS": {Consumers: []Process{ProcessControlPlane, ProcessDeployment}}, "TRUSTED_PROXY_CIDRS": {Consumers: []Process{ProcessGateway, ProcessControlPlane, ProcessDeployment}},
	"HTTP_BIND_ADDRESS": {Consumers: []Process{ProcessGateway, ProcessControlPlane, ProcessWorker, ProcessDeployment}}, "HTTP_PORT": {Consumers: []Process{ProcessGateway, ProcessControlPlane, ProcessWorker, ProcessDeployment}}, "HTTPS_PORT": {Consumers: []Process{ProcessDeployment}}, "TLS_MODE": {Consumers: []Process{ProcessDeployment}}, "TLS_CERT_FILE": {Consumers: []Process{ProcessDeployment}}, "TLS_KEY_FILE": {Secret: true, Consumers: []Process{ProcessDeployment}}, "ACME_EMAIL": {Consumers: []Process{ProcessDeployment}}, "ACME_DNS_PROVIDER": {Consumers: []Process{ProcessDeployment}}, "ACME_DNS_API_TOKEN_FILE": {Secret: true, Consumers: []Process{ProcessDeployment}},
	"POSTGRES_DB": {Consumers: []Process{ProcessDeployment}}, "POSTGRES_USER": {Consumers: []Process{ProcessDeployment}}, "POSTGRES_PASSWORD_FILE": {Secret: true, Consumers: []Process{ProcessDeployment}}, "DATABASE_HOST": {Consumers: []Process{ProcessGateway, ProcessControlPlane, ProcessWorker, ProcessBootstrap}}, "DATABASE_PORT": {Consumers: []Process{ProcessGateway, ProcessControlPlane, ProcessWorker, ProcessBootstrap}}, "DATABASE_NAME": {Consumers: []Process{ProcessGateway, ProcessControlPlane, ProcessWorker, ProcessBootstrap, ProcessDeployment}}, "DATABASE_MIGRATION_USER": {Consumers: []Process{ProcessDeployment}}, "DATABASE_MIGRATION_PASSWORD_FILE": {Secret: true, Consumers: []Process{ProcessDeployment}}, "DATABASE_GATEWAY_USER": {Consumers: []Process{ProcessGateway, ProcessDeployment}}, "DATABASE_GATEWAY_PASSWORD_FILE": {Secret: true, Consumers: []Process{ProcessGateway, ProcessDeployment}}, "DATABASE_CONTROL_PLANE_USER": {Consumers: []Process{ProcessControlPlane, ProcessBootstrap, ProcessDeployment}}, "DATABASE_CONTROL_PLANE_PASSWORD_FILE": {Secret: true, Consumers: []Process{ProcessControlPlane, ProcessBootstrap, ProcessDeployment}}, "DATABASE_WORKER_USER": {Consumers: []Process{ProcessWorker, ProcessDeployment}}, "DATABASE_WORKER_PASSWORD_FILE": {Secret: true, Consumers: []Process{ProcessWorker, ProcessDeployment}}, "DATABASE_SSLMODE": {Consumers: []Process{ProcessGateway, ProcessControlPlane, ProcessWorker, ProcessBootstrap, ProcessDeployment}}, "DATABASE_MIN_CONNECTIONS_GATEWAY": {Consumers: []Process{ProcessGateway}}, "DATABASE_MAX_CONNECTIONS_GATEWAY": {Consumers: []Process{ProcessGateway}}, "DATABASE_MIN_CONNECTIONS_CONTROL_PLANE": {Consumers: []Process{ProcessControlPlane, ProcessBootstrap}}, "DATABASE_MAX_CONNECTIONS_CONTROL_PLANE": {Consumers: []Process{ProcessControlPlane, ProcessBootstrap}}, "DATABASE_MIN_CONNECTIONS_WORKER": {Consumers: []Process{ProcessWorker}}, "DATABASE_MAX_CONNECTIONS_WORKER": {Consumers: []Process{ProcessWorker}}, "DATABASE_STATEMENT_TIMEOUT": {Consumers: []Process{ProcessGateway, ProcessControlPlane, ProcessWorker, ProcessBootstrap}}, "DATABASE_TRANSACTION_TIMEOUT": {Consumers: []Process{ProcessGateway, ProcessControlPlane, ProcessWorker, ProcessBootstrap}},
	"REDIS_URL_FILE": {Secret: true, Consumers: []Process{ProcessGateway, ProcessControlPlane, ProcessWorker, ProcessDeployment}}, "REDIS_PASSWORD_FILE": {Secret: true, Consumers: []Process{ProcessDeployment}}, "REDIS_MAX_CONNECTIONS_GATEWAY": {Consumers: []Process{ProcessGateway}}, "REDIS_MAX_CONNECTIONS_CONTROL_PLANE": {Consumers: []Process{ProcessControlPlane}}, "REDIS_MAX_CONNECTIONS_WORKER": {Consumers: []Process{ProcessWorker}},
	"DEPENDENCY_STARTUP_TIMEOUT": {Consumers: []Process{ProcessGateway, ProcessControlPlane, ProcessWorker}}, "DEPENDENCY_PROBE_TIMEOUT": {Consumers: []Process{ProcessGateway, ProcessControlPlane, ProcessWorker}}, "DEPENDENCY_PROBE_INTERVAL": {Consumers: []Process{ProcessGateway, ProcessControlPlane, ProcessWorker}}, "DEPENDENCY_RETRY_MIN_DELAY": {Consumers: []Process{ProcessGateway, ProcessControlPlane, ProcessWorker}}, "DEPENDENCY_RETRY_MAX_DELAY": {Consumers: []Process{ProcessGateway, ProcessControlPlane, ProcessWorker}},
	"MASTER_KEYRING_FILE": {Secret: true, Consumers: []Process{ProcessGateway, ProcessControlPlane, ProcessWorker}}, "API_KEY_PEPPER_RING_FILE": {Secret: true, Consumers: []Process{ProcessGateway, ProcessControlPlane}}, "SESSION_SECRET_FILE": {Secret: true, Consumers: []Process{ProcessControlPlane}}, "CSRF_SECRET_RING_FILE": {Secret: true, Consumers: []Process{ProcessControlPlane}},
	"SESSION_COOKIE_NAME": {Consumers: []Process{ProcessControlPlane}}, "SESSION_IDLE_TTL": {Consumers: []Process{ProcessControlPlane}}, "SESSION_ABSOLUTE_TTL": {Consumers: []Process{ProcessControlPlane}}, "SESSION_LAST_SEEN_WRITE_INTERVAL": {Consumers: []Process{ProcessControlPlane}}, "PASSWORD_ARGON2_MEMORY_KIB": {Consumers: []Process{ProcessControlPlane, ProcessBootstrap}}, "PASSWORD_ARGON2_ITERATIONS": {Consumers: []Process{ProcessControlPlane, ProcessBootstrap}}, "PASSWORD_ARGON2_PARALLELISM": {Consumers: []Process{ProcessControlPlane, ProcessBootstrap}},
	"REQUEST_BODY_MAX_BYTES": {Consumers: []Process{ProcessGateway}}, "UPSTREAM_CONNECT_TIMEOUT": {Consumers: []Process{ProcessGateway}}, "UPSTREAM_FIRST_BYTE_TIMEOUT": {Consumers: []Process{ProcessGateway}}, "UPSTREAM_IDLE_TIMEOUT": {Consumers: []Process{ProcessGateway}}, "UPSTREAM_TOTAL_TIMEOUT": {Consumers: []Process{ProcessGateway, ProcessWorker}}, "SHUTDOWN_GRACE_PERIOD": {Consumers: []Process{ProcessGateway, ProcessControlPlane, ProcessWorker, ProcessDeployment}}, "MAX_UPSTREAM_ATTEMPTS": {Consumers: []Process{ProcessGateway}},
	"LOG_LEVEL": {Consumers: []Process{ProcessGateway, ProcessControlPlane, ProcessWorker, ProcessBootstrap}}, "LOG_FORMAT": {Consumers: []Process{ProcessGateway, ProcessControlPlane, ProcessWorker, ProcessBootstrap}}, "METRICS_ENABLED": {Consumers: []Process{ProcessGateway, ProcessControlPlane, ProcessWorker}}, "OTEL_ENABLED": {Consumers: []Process{ProcessGateway, ProcessControlPlane, ProcessWorker}}, "OTEL_EXPORTER_OTLP_ENDPOINT": {Consumers: []Process{ProcessGateway, ProcessControlPlane, ProcessWorker}},
	"REQUEST_RETENTION_DAYS": {Consumers: []Process{ProcessWorker}}, "AUDIT_RETENTION_DAYS": {Consumers: []Process{ProcessWorker}}, "HEALTH_OBSERVATION_RETENTION_DAYS": {Consumers: []Process{ProcessWorker}}, "PROCESSED_OUTBOX_RETENTION_DAYS": {Consumers: []Process{ProcessWorker}}, "ANALYTICS_TARGET_LAG": {Consumers: []Process{ProcessWorker}}, "STALE_REQUEST_RECONCILIATION_INTERVAL": {Consumers: []Process{ProcessWorker}}, "WORKER_CONCURRENCY": {Consumers: []Process{ProcessWorker}}, "OUTBOX_LEASE_DURATION": {Consumers: []Process{ProcessWorker}}, "OUTBOX_RETRY_BASE_DELAY": {Consumers: []Process{ProcessWorker}}, "OUTBOX_RETRY_MAX_DELAY": {Consumers: []Process{ProcessWorker}}, "HEALTH_PROBE_INTERVAL": {Consumers: []Process{ProcessWorker}}, "HEALTH_FAST_WINDOW": {Consumers: []Process{ProcessWorker}}, "HEALTH_STABILIZING_WINDOW": {Consumers: []Process{ProcessWorker}}, "HEALTH_MIN_SAMPLES": {Consumers: []Process{ProcessWorker}}, "HEALTH_POLICY_VERSION": {Consumers: []Process{ProcessWorker}}, "RETENTION_BATCH_SIZE": {Consumers: []Process{ProcessWorker}},
	"HEALTH_UNAVAILABLE_ENTRY_THRESHOLD_BPS": {Consumers: []Process{ProcessWorker}}, "HEALTH_UNAVAILABLE_EXIT_THRESHOLD_BPS": {Consumers: []Process{ProcessWorker}}, "HEALTH_CONSECUTIVE_FAILURES": {Consumers: []Process{ProcessWorker}}, "HEALTH_CONSECUTIVE_SUCCESSES": {Consumers: []Process{ProcessWorker}},
	"AGENT_API_KEY_ENV": {Consumers: []Process{ProcessControlPlane}}, "OPENCODE_PROVIDER_ID": {Consumers: []Process{ProcessControlPlane}}, "OPENCODE_PROVIDER_NAME": {Consumers: []Process{ProcessControlPlane}},
	"ENABLE_CLOUDFLARE_TUNNEL": {Consumers: []Process{ProcessDeployment}}, "CLOUDFLARE_TUNNEL_ID": {Consumers: []Process{ProcessDeployment}}, "CLOUDFLARE_TUNNEL_CREDENTIALS_FILE": {Secret: true, Consumers: []Process{ProcessDeployment}},
	"ENABLE_TAILSCALE_PRIVATE_ADMIN": {Consumers: []Process{ProcessDeployment}}, "TAILSCALE_AUTH_KEY_FILE": {Secret: true}, "TAILSCALE_STATE_DIR": {}, "TAILSCALE_HOSTNAME": {}, "PRIVATE_INGRESS_SUBNET": {}, "PRIVATE_TRAEFIK_IP": {}, "PRIVATE_DNS_IP": {}, "TAILSCALE_ADVERTISE_ROUTES": {},
	"BOOTSTRAP_OWNER_EMAIL": {Consumers: []Process{ProcessBootstrap}}, "BOOTSTRAP_OWNER_NAME": {Consumers: []Process{ProcessBootstrap}}, "BOOTSTRAP_ORGANIZATION_NAME": {Consumers: []Process{ProcessBootstrap}}, "BOOTSTRAP_ORGANIZATION_SLUG": {Consumers: []Process{ProcessBootstrap}}, "BOOTSTRAP_PASSWORD_FILE": {Secret: true, Consumers: []Process{ProcessBootstrap}},
}

var unmistakableOwnedPrefixes = []string{"NEXUSRELAY_", "BOOTSTRAP_"}

var obsoleteSettings = map[string]struct{}{
	"CSRF_SECRET_FILE":              {},
	"DATABASE_USER":                 {},
	"DATABASE_PASSWORD_FILE":        {},
	"SHARED_DATABASE_PASSWORD_FILE": {},
}

type SharedDeployment struct {
	Environment string
	Version     string
}

type Endpoints struct {
	PublicAPIBaseURL *url.URL
	AdminBaseURL     *url.URL
	PublicAPIHost    string
	AdminHost        string
	AdminExposure    string
	AdminOrigins     []*url.URL
	TrustedProxies   []*net.IPNet
}

type HTTP struct {
	Address       string
	Port          int
	ShutdownGrace time.Duration
}

type TLS struct {
	Mode            string
	CertificateFile string
	KeyFile         string
	ACMEEmail       string
	ACMEDNSProvider string
	ACMEDNSAPIToken Secret
}

type Database struct {
	Host               string
	Port               int
	Name               string
	User               string
	Password           Secret
	SSLMode            string
	MinConnections     int
	MaxConnections     int
	StatementTimeout   time.Duration
	TransactionTimeout time.Duration
}

type databaseProcessSettings struct {
	userSetting     string
	passwordSetting string
	minSetting      string
	maxSetting      string
	user            string
	defaultMax      int
}

var (
	gatewayDatabaseSettings = databaseProcessSettings{
		userSetting: "DATABASE_GATEWAY_USER", passwordSetting: "DATABASE_GATEWAY_PASSWORD_FILE",
		minSetting: "DATABASE_MIN_CONNECTIONS_GATEWAY", maxSetting: "DATABASE_MAX_CONNECTIONS_GATEWAY",
		user: gatewayUser, defaultMax: 30,
	}
	controlPlaneDatabaseSettings = databaseProcessSettings{
		userSetting: "DATABASE_CONTROL_PLANE_USER", passwordSetting: "DATABASE_CONTROL_PLANE_PASSWORD_FILE",
		minSetting: "DATABASE_MIN_CONNECTIONS_CONTROL_PLANE", maxSetting: "DATABASE_MAX_CONNECTIONS_CONTROL_PLANE",
		user: controlPlaneUser, defaultMax: 20,
	}
	workerDatabaseSettings = databaseProcessSettings{
		userSetting: "DATABASE_WORKER_USER", passwordSetting: "DATABASE_WORKER_PASSWORD_FILE",
		minSetting: "DATABASE_MIN_CONNECTIONS_WORKER", maxSetting: "DATABASE_MAX_CONNECTIONS_WORKER",
		user: workerUser, defaultMax: 10,
	}
)

type Redis struct {
	URL            Secret
	MaxConnections int
}

type DependencyReadiness struct {
	StartupTimeout time.Duration
	ProbeTimeout   time.Duration
	ProbeInterval  time.Duration
	RetryMinimum   time.Duration
	RetryMaximum   time.Duration
}

type Observability struct {
	LogLevel     string
	LogFormat    string
	Metrics      bool
	OTel         bool
	OTLPEndpoint *url.URL
}

type SessionPolicy struct {
	CookieName            string
	IdleTTL               time.Duration
	AbsoluteTTL           time.Duration
	LastSeenWriteInterval time.Duration
	Secret                Secret
	CSRFKeys              KeyRing
	Argon2MemoryKiB       uint32
	Argon2Iterations      uint32
	Argon2Parallelism     uint8
}

type PasswordPolicy struct {
	Argon2MemoryKiB   uint32
	Argon2Iterations  uint32
	Argon2Parallelism uint8
}

type GatewayPolicy struct {
	RequestBodyMaxBytes      int64
	UpstreamConnectTimeout   time.Duration
	UpstreamFirstByteTimeout time.Duration
	UpstreamIdleTimeout      time.Duration
	UpstreamTotalTimeout     time.Duration
	MaxUpstreamAttempts      int
}

type WorkerPolicy struct {
	RequestRetentionDays               int
	AuditRetentionDays                 int
	HealthObservationRetentionDays     int
	ProcessedOutboxRetentionDays       int
	AnalyticsTargetLag                 time.Duration
	StaleRequestReconciliationInterval time.Duration
	Concurrency                        int
	OutboxLeaseDuration                time.Duration
	OutboxRetryBaseDelay               time.Duration
	OutboxRetryMaxDelay                time.Duration
	HealthProbeInterval                time.Duration
	HealthFastWindow                   time.Duration
	HealthStabilizingWindow            time.Duration
	HealthMinSamples                   int
	HealthPolicyVersion                int
	HealthUnavailableEntryThresholdBPS int
	HealthUnavailableExitThresholdBPS  int
	HealthConsecutiveFailures          int
	HealthConsecutiveSuccesses         int
	RetentionBatchSize                 int
	MaximumRequestDuration             time.Duration
}

type AgentExport struct {
	APIKeyEnvironment    string
	OpenCodeProviderID   string
	OpenCodeProviderName string
}

type OptionalIngress struct {
	CloudflareEnabled bool
	TailscaleEnabled  bool
}

type Cloudflare struct {
	Enabled         bool
	TunnelID        string
	CredentialsFile string
}

type Tailscale struct {
	Enabled bool
}

type DatabaseInventory struct {
	Name          string
	ClusterAdmin  string
	MigrationUser string
	GatewayUser   string
	ControlUser   string
	WorkerUser    string
	PasswordFiles map[string]string
	SSLMode       string
}

type Deployment struct {
	Shared        SharedDeployment
	Endpoints     Endpoints
	HTTP          HTTP
	HTTPSPort     int
	TLS           TLS
	Database      DatabaseInventory
	RedisPassword Secret
	Cloudflare    Cloudflare
	Tailscale     Tailscale
}

type Gateway struct {
	Shared        SharedDeployment
	Endpoints     Endpoints
	HTTP          HTTP
	Database      Database
	Redis         Redis
	Readiness     DependencyReadiness
	Observability Observability
	Policy        GatewayPolicy
	ProviderKeys  ProviderKeyRing
	APIKeyPeppers KeyRing
	Ingress       OptionalIngress
}

type ControlPlane struct {
	Shared        SharedDeployment
	Endpoints     Endpoints
	HTTP          HTTP
	Database      Database
	Redis         Redis
	Readiness     DependencyReadiness
	Observability Observability
	Sessions      SessionPolicy
	ProviderKeys  ProviderKeyRing
	APIKeyPeppers KeyRing
	AgentExport   AgentExport
	Ingress       OptionalIngress
}

type Worker struct {
	Shared        SharedDeployment
	Endpoints     Endpoints
	HTTP          HTTP
	Database      Database
	Redis         Redis
	Readiness     DependencyReadiness
	Observability Observability
	Policy        WorkerPolicy
	ProviderKeys  ProviderKeyRing
	Ingress       OptionalIngress
}

type Bootstrap struct {
	Shared           SharedDeployment
	Database         Database
	Observability    Observability
	PasswordPolicy   PasswordPolicy
	OwnerEmail       string
	OwnerName        string
	OrganizationName string
	OrganizationSlug string
	Password         Secret
}

func Inventory() map[string]SettingClass {
	copy := make(map[string]SettingClass, len(inventory))
	for name, class := range inventory {
		class.Consumers = append([]Process(nil), class.Consumers...)
		copy[name] = class
	}
	return copy
}

func LoadGateway() (Gateway, error)           { return ParseGateway(environment()) }
func LoadControlPlane() (ControlPlane, error) { return ParseControlPlane(environment()) }
func LoadWorker() (Worker, error)             { return ParseWorker(environment()) }
func LoadBootstrap() (Bootstrap, error)       { return ParseBootstrap(environment()) }
func LoadDeployment() (Deployment, error)     { return ParseDeployment(environment()) }

func ParseGateway(v map[string]string) (Gateway, error) {
	if err := preflight(v, ProcessGateway); err != nil {
		return Gateway{}, err
	}
	shared, http, obs, err := parseServiceCommon(v)
	if err != nil {
		return Gateway{}, err
	}
	endpoints, err := parseGatewayEndpoints(v, shared.Environment)
	if err != nil {
		return Gateway{}, err
	}
	db, err := parseDatabase(v, gatewayDatabaseSettings)
	if err != nil {
		return Gateway{}, err
	}
	redis, err := parseRedis(v, "REDIS_MAX_CONNECTIONS_GATEWAY", 50)
	if err != nil {
		return Gateway{}, err
	}
	readiness, err := parseDependencyReadiness(v)
	if err != nil {
		return Gateway{}, err
	}
	provider, err := ReadProviderKeyRing(requiredPath(v, "MASTER_KEYRING_FILE", "/run/secrets/provider_master_keyring"))
	if err != nil {
		return Gateway{}, err
	}
	peppers, err := ReadKeyRing("API_KEY_PEPPER_RING_FILE", requiredPath(v, "API_KEY_PEPPER_RING_FILE", "/run/secrets/api_key_pepper_ring"))
	if err != nil {
		return Gateway{}, err
	}
	policy, err := parseGatewayPolicy(v)
	if err != nil {
		return Gateway{}, err
	}
	return Gateway{shared, endpoints, http, db, redis, readiness, obs, policy, provider, peppers, OptionalIngress{}}, nil
}

func ParseControlPlane(v map[string]string) (ControlPlane, error) {
	if err := preflight(v, ProcessControlPlane); err != nil {
		return ControlPlane{}, err
	}
	shared, http, obs, err := parseServiceCommon(v)
	if err != nil {
		return ControlPlane{}, err
	}
	endpoints, err := parseControlPlaneEndpoints(v, shared.Environment)
	if err != nil {
		return ControlPlane{}, err
	}
	db, err := parseDatabase(v, controlPlaneDatabaseSettings)
	if err != nil {
		return ControlPlane{}, err
	}
	redis, err := parseRedis(v, "REDIS_MAX_CONNECTIONS_CONTROL_PLANE", 20)
	if err != nil {
		return ControlPlane{}, err
	}
	readiness, err := parseDependencyReadiness(v)
	if err != nil {
		return ControlPlane{}, err
	}
	provider, err := ReadProviderKeyRing(requiredPath(v, "MASTER_KEYRING_FILE", "/run/secrets/provider_master_keyring"))
	if err != nil {
		return ControlPlane{}, err
	}
	peppers, err := ReadKeyRing("API_KEY_PEPPER_RING_FILE", requiredPath(v, "API_KEY_PEPPER_RING_FILE", "/run/secrets/api_key_pepper_ring"))
	if err != nil {
		return ControlPlane{}, err
	}
	sessions, err := parseSessionPolicy(v)
	if err != nil {
		return ControlPlane{}, err
	}
	if isProductionGrade(shared.Environment) && !strings.HasPrefix(sessions.CookieName, "__Host-") {
		return ControlPlane{}, fmt.Errorf("SESSION_COOKIE_NAME must use the __Host- prefix in production")
	}
	agent, err := parseAgent(v)
	if err != nil {
		return ControlPlane{}, err
	}
	return ControlPlane{shared, endpoints, http, db, redis, readiness, obs, sessions, provider, peppers, agent, OptionalIngress{}}, nil
}

func ParseWorker(v map[string]string) (Worker, error) {
	if err := preflight(v, ProcessWorker); err != nil {
		return Worker{}, err
	}
	shared, http, obs, err := parseServiceCommon(v)
	if err != nil {
		return Worker{}, err
	}
	db, err := parseDatabase(v, workerDatabaseSettings)
	if err != nil {
		return Worker{}, err
	}
	redis, err := parseRedis(v, "REDIS_MAX_CONNECTIONS_WORKER", 20)
	if err != nil {
		return Worker{}, err
	}
	readiness, err := parseDependencyReadiness(v)
	if err != nil {
		return Worker{}, err
	}
	provider, err := ReadProviderKeyRing(requiredPath(v, "MASTER_KEYRING_FILE", "/run/secrets/provider_master_keyring"))
	if err != nil {
		return Worker{}, err
	}
	policy, err := parseWorkerPolicy(v)
	if err != nil {
		return Worker{}, err
	}
	return Worker{shared, Endpoints{}, http, db, redis, readiness, obs, policy, provider, OptionalIngress{}}, nil
}

func parseDependencyReadiness(v map[string]string) (DependencyReadiness, error) {
	startup, err := duration(v, "DEPENDENCY_STARTUP_TIMEOUT", 60*time.Second, time.Second, 10*time.Minute)
	if err != nil {
		return DependencyReadiness{}, err
	}
	probe, err := duration(v, "DEPENDENCY_PROBE_TIMEOUT", 3*time.Second, 100*time.Millisecond, time.Minute)
	if err != nil {
		return DependencyReadiness{}, err
	}
	interval, err := duration(v, "DEPENDENCY_PROBE_INTERVAL", 5*time.Second, time.Second, time.Minute)
	if err != nil {
		return DependencyReadiness{}, err
	}
	minimum, err := duration(v, "DEPENDENCY_RETRY_MIN_DELAY", 100*time.Millisecond, 10*time.Millisecond, time.Minute)
	if err != nil {
		return DependencyReadiness{}, err
	}
	maximum, err := duration(v, "DEPENDENCY_RETRY_MAX_DELAY", 2*time.Second, 10*time.Millisecond, time.Minute)
	if err != nil {
		return DependencyReadiness{}, err
	}
	if probe >= startup {
		return DependencyReadiness{}, fmt.Errorf("DEPENDENCY_PROBE_TIMEOUT must be less than DEPENDENCY_STARTUP_TIMEOUT")
	}
	if minimum > maximum {
		return DependencyReadiness{}, fmt.Errorf("DEPENDENCY_RETRY_MIN_DELAY must not exceed DEPENDENCY_RETRY_MAX_DELAY")
	}
	return DependencyReadiness{startup, probe, interval, minimum, maximum}, nil
}

func ParseBootstrap(v map[string]string) (Bootstrap, error) {
	if err := preflight(v, ProcessBootstrap); err != nil {
		return Bootstrap{}, err
	}
	shared := SharedDeployment{value(v, "NEXUSRELAY_ENV", "development"), value(v, "NEXUSRELAY_VERSION", "dev")}
	if err := validateEnvironment(shared.Environment); err != nil {
		return Bootstrap{}, err
	}
	if err := cleanNonempty("NEXUSRELAY_VERSION", shared.Version); err != nil {
		return Bootstrap{}, err
	}
	obs, err := parseLogging(v)
	if err != nil {
		return Bootstrap{}, err
	}
	db, err := parseDatabase(v, controlPlaneDatabaseSettings)
	if err != nil {
		return Bootstrap{}, err
	}
	passwordPolicy, err := parsePasswordPolicy(v)
	if err != nil {
		return Bootstrap{}, err
	}
	email, err := requiredText(v, "BOOTSTRAP_OWNER_EMAIL")
	if err != nil {
		return Bootstrap{}, err
	}
	if address, parseErr := mail.ParseAddress(email); parseErr != nil || address.Address != email {
		return Bootstrap{}, fmt.Errorf("BOOTSTRAP_OWNER_EMAIL must be one valid email address")
	}
	name, err := requiredText(v, "BOOTSTRAP_OWNER_NAME")
	if err != nil {
		return Bootstrap{}, err
	}
	org, err := requiredText(v, "BOOTSTRAP_ORGANIZATION_NAME")
	if err != nil {
		return Bootstrap{}, err
	}
	slug, err := requiredText(v, "BOOTSTRAP_ORGANIZATION_SLUG")
	if err != nil {
		return Bootstrap{}, err
	}
	if ok, _ := regexp.MatchString(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`, slug); !ok {
		return Bootstrap{}, fmt.Errorf("BOOTSTRAP_ORGANIZATION_SLUG has invalid format")
	}
	password, err := ReadSimpleSecret("BOOTSTRAP_PASSWORD_FILE", requiredPath(v, "BOOTSTRAP_PASSWORD_FILE", "/run/secrets/bootstrap_password"), 4*1024)
	if err != nil {
		return Bootstrap{}, err
	}
	return Bootstrap{shared, db, obs, passwordPolicy, email, name, org, slug, password}, nil
}

func ParseDeployment(v map[string]string) (Deployment, error) {
	if err := preflight(v, ProcessDeployment); err != nil {
		return Deployment{}, err
	}
	shared := SharedDeployment{value(v, "NEXUSRELAY_ENV", "development"), value(v, "NEXUSRELAY_VERSION", "dev")}
	if err := validateEnvironment(shared.Environment); err != nil {
		return Deployment{}, err
	}
	if err := cleanNonempty("NEXUSRELAY_VERSION", shared.Version); err != nil {
		return Deployment{}, err
	}
	endpoints, err := parseEndpoints(v, shared.Environment)
	if err != nil {
		return Deployment{}, err
	}
	http, err := parseHTTP(v)
	if err != nil {
		return Deployment{}, err
	}
	httpsPort, err := integer(v, "HTTPS_PORT", 8443, 1, 65535)
	if err != nil {
		return Deployment{}, err
	}
	tls, err := parseTLS(v, shared.Environment, endpoints)
	if err != nil {
		return Deployment{}, err
	}
	database, err := parseDatabaseInventory(v, shared.Environment)
	if err != nil {
		return Deployment{}, err
	}
	redisPassword, err := parseRedisDeployment(v)
	if err != nil {
		return Deployment{}, err
	}
	cloudflare, err := parseCloudflare(v, endpoints, tls)
	if err != nil {
		return Deployment{}, err
	}
	tailscale, err := parseTailscale(v)
	if err != nil {
		return Deployment{}, err
	}
	return Deployment{shared, endpoints, http, httpsPort, tls, database, redisPassword, cloudflare, tailscale}, nil
}

func parseRedisDeployment(v map[string]string) (Secret, error) {
	password, err := ReadSimpleSecret("REDIS_PASSWORD_FILE", requiredPath(v, "REDIS_PASSWORD_FILE", "/run/secrets/redis_password"), 4*1024)
	if err != nil {
		return Secret{}, err
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_-]{32,}$`).MatchString(password.Reveal()) {
		return Secret{}, fmt.Errorf("REDIS_PASSWORD_FILE must contain at least 32 base64url characters")
	}
	_, redisURL, err := ReadURISecret("REDIS_URL_FILE", requiredPath(v, "REDIS_URL_FILE", "/run/secrets/redis_url"), 4*1024, "redis", "rediss")
	if err != nil {
		return Secret{}, err
	}
	if redisURL.User == nil {
		return Secret{}, fmt.Errorf("REDIS_URL_FILE must contain Redis authentication credentials")
	}
	if username := redisURL.User.Username(); username != "" && username != "default" {
		return Secret{}, fmt.Errorf("REDIS_URL_FILE username must be empty or default")
	}
	urlPassword, ok := redisURL.User.Password()
	if !ok || urlPassword != password.Reveal() {
		return Secret{}, fmt.Errorf("REDIS_PASSWORD_FILE must match the password in REDIS_URL_FILE")
	}
	return password, nil
}

func preflight(v map[string]string, process Process) error {
	for name := range v {
		if _, ok := inventory[name]; ok {
			continue
		}
		if _, obsolete := obsoleteSettings[name]; obsolete {
			return fmt.Errorf("%s is not a documented NexusRelay setting", name)
		}
		for _, prefix := range unmistakableOwnedPrefixes {
			if strings.HasPrefix(name, prefix) {
				return fmt.Errorf("%s is not a documented NexusRelay setting", name)
			}
		}
	}
	if process == ProcessGateway || process == ProcessControlPlane || process == ProcessWorker {
		for _, name := range []string{"BOOTSTRAP_OWNER_EMAIL", "BOOTSTRAP_OWNER_NAME", "BOOTSTRAP_ORGANIZATION_NAME", "BOOTSTRAP_ORGANIZATION_SLUG", "BOOTSTRAP_PASSWORD_FILE"} {
			if _, ok := v[name]; ok {
				return fmt.Errorf("%s is command-only and is not accepted by long-running services", name)
			}
		}
	}
	return nil
}

func parseServiceCommon(v map[string]string) (SharedDeployment, HTTP, Observability, error) {
	shared := SharedDeployment{value(v, "NEXUSRELAY_ENV", "development"), value(v, "NEXUSRELAY_VERSION", "dev")}
	if err := validateEnvironment(shared.Environment); err != nil {
		return SharedDeployment{}, HTTP{}, Observability{}, err
	}
	if err := cleanNonempty("NEXUSRELAY_VERSION", shared.Version); err != nil {
		return SharedDeployment{}, HTTP{}, Observability{}, err
	}
	http, err := parseHTTP(v)
	if err != nil {
		return SharedDeployment{}, HTTP{}, Observability{}, err
	}
	obs, err := parseObservability(v)
	if err != nil {
		return SharedDeployment{}, HTTP{}, Observability{}, err
	}
	return shared, http, obs, nil
}

func parseGatewayEndpoints(v map[string]string, environment string) (Endpoints, error) {
	public, err := parsePublicAPIURL(v, environment)
	if err != nil {
		return Endpoints{}, err
	}
	proxies, err := parseCIDRs(value(v, "TRUSTED_PROXY_CIDRS", ""))
	if err != nil {
		return Endpoints{}, err
	}
	return Endpoints{PublicAPIBaseURL: public, TrustedProxies: proxies}, nil
}

func parseControlPlaneEndpoints(v map[string]string, environment string) (Endpoints, error) {
	public, err := parsePublicAPIURL(v, environment)
	if err != nil {
		return Endpoints{}, err
	}
	admin, err := originURL("ADMIN_BASE_URL", value(v, "ADMIN_BASE_URL", "http://localhost:8080"))
	if err != nil {
		return Endpoints{}, err
	}
	if isProductionGrade(environment) && admin.Scheme != "https" {
		return Endpoints{}, fmt.Errorf("production ADMIN_BASE_URL must use https")
	}
	origins, err := parseOrigins(value(v, "ADMIN_ORIGINS", "http://localhost:8080"))
	if err != nil {
		return Endpoints{}, err
	}
	if err := validateAdminOrigins(admin, origins, environment); err != nil {
		return Endpoints{}, err
	}
	proxies, err := parseCIDRs(value(v, "TRUSTED_PROXY_CIDRS", ""))
	if err != nil {
		return Endpoints{}, err
	}
	return Endpoints{PublicAPIBaseURL: public, AdminBaseURL: admin, AdminOrigins: origins, TrustedProxies: proxies}, nil
}

func parsePublicAPIURL(v map[string]string, environment string) (*url.URL, error) {
	public, err := absoluteURL("PUBLIC_API_BASE_URL", value(v, "PUBLIC_API_BASE_URL", "http://localhost:8080/v1"))
	if err != nil {
		return nil, err
	}
	if public.Path != "/v1" || public.RawQuery != "" || public.Fragment != "" || public.User != nil {
		return nil, fmt.Errorf("PUBLIC_API_BASE_URL must be an absolute HTTP(S) URL ending exactly in /v1")
	}
	if isProductionGrade(environment) && public.Scheme != "https" {
		return nil, fmt.Errorf("production PUBLIC_API_BASE_URL must use https")
	}
	return public, nil
}

func parseEndpoints(v map[string]string, environment string) (Endpoints, error) {
	public, err := absoluteURL("PUBLIC_API_BASE_URL", value(v, "PUBLIC_API_BASE_URL", "http://localhost:8080/v1"))
	if err != nil {
		return Endpoints{}, err
	}
	if public.Path != "/v1" || public.RawQuery != "" || public.Fragment != "" || public.User != nil {
		return Endpoints{}, fmt.Errorf("PUBLIC_API_BASE_URL must be an absolute HTTP(S) URL ending exactly in /v1")
	}
	admin, err := originURL("ADMIN_BASE_URL", value(v, "ADMIN_BASE_URL", "http://localhost:8080"))
	if err != nil {
		return Endpoints{}, err
	}
	publicHost := value(v, "PUBLIC_API_HOST", "localhost")
	adminHost := value(v, "ADMIN_HOST", "localhost")
	if !strings.EqualFold(public.Hostname(), publicHost) {
		return Endpoints{}, fmt.Errorf("PUBLIC_API_HOST must match PUBLIC_API_BASE_URL hostname")
	}
	if !strings.EqualFold(admin.Hostname(), adminHost) {
		return Endpoints{}, fmt.Errorf("ADMIN_HOST must match ADMIN_BASE_URL hostname")
	}
	exposure := value(v, "ADMIN_EXPOSURE_MODE", "local")
	if !oneOf(exposure, "local", "private", "public") {
		return Endpoints{}, fmt.Errorf("ADMIN_EXPOSURE_MODE must be local, private, or public")
	}
	if isProductionGrade(environment) && (public.Scheme != "https" || admin.Scheme != "https") {
		return Endpoints{}, fmt.Errorf("production PUBLIC_API_BASE_URL and ADMIN_BASE_URL must use https")
	}
	origins, err := parseOrigins(value(v, "ADMIN_ORIGINS", "http://localhost:8080"))
	if err != nil {
		return Endpoints{}, err
	}
	if err := validateAdminOrigins(admin, origins, environment); err != nil {
		return Endpoints{}, err
	}
	proxies, err := parseCIDRs(value(v, "TRUSTED_PROXY_CIDRS", ""))
	if err != nil {
		return Endpoints{}, err
	}
	return Endpoints{public, admin, publicHost, adminHost, exposure, origins, proxies}, nil
}

func validateAdminOrigins(admin *url.URL, origins []*url.URL, environment string) error {
	adminAllowed := false
	for _, origin := range origins {
		adminAllowed = adminAllowed || origin.String() == admin.String()
		if isProductionGrade(environment) && origin.Scheme != "https" {
			return fmt.Errorf("production ADMIN_ORIGINS entries must use https")
		}
	}
	if !adminAllowed {
		return fmt.Errorf("ADMIN_ORIGINS must include ADMIN_BASE_URL")
	}
	return nil
}

func parseHTTP(v map[string]string) (HTTP, error) {
	bind := value(v, "HTTP_BIND_ADDRESS", "0.0.0.0")
	if net.ParseIP(bind) == nil {
		return HTTP{}, fmt.Errorf("HTTP_BIND_ADDRESS must be an IP address")
	}
	port, err := integer(v, "HTTP_PORT", 8080, 1, 65535)
	if err != nil {
		return HTTP{}, err
	}
	grace, err := duration(v, "SHUTDOWN_GRACE_PERIOD", 30*time.Second, time.Second, 5*time.Minute)
	if err != nil {
		return HTTP{}, err
	}
	return HTTP{net.JoinHostPort(bind, strconv.Itoa(port)), port, grace}, nil
}

func parseTLS(v map[string]string, environment string, endpoints Endpoints) (TLS, error) {
	tls := TLS{Mode: value(v, "TLS_MODE", "disabled"), CertificateFile: value(v, "TLS_CERT_FILE", ""), KeyFile: value(v, "TLS_KEY_FILE", ""), ACMEEmail: value(v, "ACME_EMAIL", ""), ACMEDNSProvider: value(v, "ACME_DNS_PROVIDER", "")}
	if !oneOf(tls.Mode, "disabled", "files", "acme") {
		return TLS{}, fmt.Errorf("TLS_MODE must be disabled, files, or acme")
	}
	if tls.Mode == "files" {
		if err := readableRegular("TLS_CERT_FILE", tls.CertificateFile); err != nil {
			return TLS{}, err
		}
		if err := validateProtectedFile("TLS_KEY_FILE", tls.KeyFile); err != nil {
			return TLS{}, err
		}
	}
	if tls.Mode == "acme" {
		if _, err := mail.ParseAddress(tls.ACMEEmail); err != nil {
			return TLS{}, fmt.Errorf("ACME_EMAIL must be a valid email address in acme mode")
		}
		if err := cleanNonempty("ACME_DNS_PROVIDER", tls.ACMEDNSProvider); err != nil {
			return TLS{}, err
		}
		var err error
		tls.ACMEDNSAPIToken, err = ReadSimpleSecret("ACME_DNS_API_TOKEN_FILE", requiredPath(v, "ACME_DNS_API_TOKEN_FILE", "/run/secrets/acme_dns_api_token"), 8*1024)
		if err != nil {
			return TLS{}, err
		}
	}
	if isProductionGrade(environment) && (endpoints.PublicAPIBaseURL.Scheme != "https" || endpoints.AdminBaseURL.Scheme != "https") {
		return TLS{}, fmt.Errorf("production endpoints require https")
	}
	return tls, nil
}

func parseDatabaseInventory(v map[string]string, environment string) (DatabaseInventory, error) {
	postgresDB := value(v, "POSTGRES_DB", "nexusrelay")
	if postgresDB != value(v, "DATABASE_NAME", "nexusrelay") {
		return DatabaseInventory{}, fmt.Errorf("POSTGRES_DB must equal DATABASE_NAME")
	}
	fixed := map[string]string{"POSTGRES_USER": clusterAdminUser, "DATABASE_MIGRATION_USER": migrationUser, "DATABASE_GATEWAY_USER": gatewayUser, "DATABASE_CONTROL_PLANE_USER": controlPlaneUser, "DATABASE_WORKER_USER": workerUser}
	for name, expected := range fixed {
		if value(v, name, expected) != expected {
			return DatabaseInventory{}, fmt.Errorf("%s must equal %s", name, expected)
		}
	}
	paths := map[string]string{
		"POSTGRES_PASSWORD_FILE":               value(v, "POSTGRES_PASSWORD_FILE", "/run/secrets/postgres_cluster_admin_password"),
		"DATABASE_MIGRATION_PASSWORD_FILE":     value(v, "DATABASE_MIGRATION_PASSWORD_FILE", "/run/secrets/postgres_migration_password"),
		"DATABASE_GATEWAY_PASSWORD_FILE":       value(v, "DATABASE_GATEWAY_PASSWORD_FILE", "/run/secrets/postgres_gateway_password"),
		"DATABASE_CONTROL_PLANE_PASSWORD_FILE": value(v, "DATABASE_CONTROL_PLANE_PASSWORD_FILE", "/run/secrets/postgres_control_plane_password"),
		"DATABASE_WORKER_PASSWORD_FILE":        value(v, "DATABASE_WORKER_PASSWORD_FILE", "/run/secrets/postgres_worker_password"),
	}
	seen := map[string]string{}
	seenValues := map[string]string{}
	for name, path := range paths {
		if path == "" {
			return DatabaseInventory{}, fmt.Errorf("%s must name a secret file", name)
		}
		cleaned := filepath.Clean(path)
		if previous, ok := seen[cleaned]; ok {
			return DatabaseInventory{}, fmt.Errorf("%s and %s must name distinct files", previous, name)
		}
		seen[cleaned] = name
		secret, err := ReadSimpleSecret(name, path, 4*1024)
		if err != nil {
			return DatabaseInventory{}, err
		}
		if previous, ok := seenValues[secret.Reveal()]; ok {
			return DatabaseInventory{}, fmt.Errorf("%s and %s must contain distinct passwords", previous, name)
		}
		seenValues[secret.Reveal()] = name
	}
	sslmode, err := parseDatabaseSSLMode(v, environment)
	if err != nil {
		return DatabaseInventory{}, err
	}
	return DatabaseInventory{postgresDB, clusterAdminUser, migrationUser, gatewayUser, controlPlaneUser, workerUser, paths, sslmode}, nil
}

func parseCloudflare(v map[string]string, endpoints Endpoints, tls TLS) (Cloudflare, error) {
	enabled, err := boolean(v, "ENABLE_CLOUDFLARE_TUNNEL", false)
	if err != nil {
		return Cloudflare{}, err
	}
	config := Cloudflare{enabled, value(v, "CLOUDFLARE_TUNNEL_ID", ""), value(v, "CLOUDFLARE_TUNNEL_CREDENTIALS_FILE", "/run/secrets/cloudflare_tunnel_credentials.json")}
	if !enabled {
		return config, nil
	}
	if endpoints.PublicAPIBaseURL.Scheme != "https" {
		return Cloudflare{}, fmt.Errorf("ENABLE_CLOUDFLARE_TUNNEL requires PUBLIC_API_BASE_URL to use https")
	}
	if tls.Mode != "acme" || tls.ACMEDNSProvider != "cloudflare" {
		return Cloudflare{}, fmt.Errorf("ENABLE_CLOUDFLARE_TUNNEL requires TLS_MODE=acme and ACME_DNS_PROVIDER=cloudflare")
	}
	if err := validatePublicTunnelHosts(endpoints); err != nil {
		return Cloudflare{}, err
	}
	if err := cleanNonempty("CLOUDFLARE_TUNNEL_ID", config.TunnelID); err != nil {
		return Cloudflare{}, err
	}
	credentials, err := ReadCloudflareCredentials(config.CredentialsFile)
	if err != nil {
		return Cloudflare{}, err
	}
	if credentials.TunnelID != config.TunnelID {
		return Cloudflare{}, fmt.Errorf("CLOUDFLARE_TUNNEL_CREDENTIALS_FILE must match CLOUDFLARE_TUNNEL_ID")
	}
	return config, nil
}

func parseTailscale(v map[string]string) (Tailscale, error) {
	enabled, err := boolean(v, "ENABLE_TAILSCALE_PRIVATE_ADMIN", false)
	if err != nil {
		return Tailscale{}, err
	}
	if !enabled {
		return Tailscale{}, nil
	}
	return Tailscale{}, fmt.Errorf("ENABLE_TAILSCALE_PRIVATE_ADMIN=true is unsupported while ADR 0002 is Proposed")
}

func parseDatabase(v map[string]string, settings databaseProcessSettings) (Database, error) {
	databaseName := value(v, "DATABASE_NAME", "nexusrelay")
	if err := cleanNonempty("DATABASE_NAME", databaseName); err != nil {
		return Database{}, err
	}
	host := value(v, "DATABASE_HOST", "postgres")
	if err := cleanNonempty("DATABASE_HOST", host); err != nil {
		return Database{}, err
	}
	port, err := integer(v, "DATABASE_PORT", 5432, 1, 65535)
	if err != nil {
		return Database{}, err
	}
	sslmode, err := parseDatabaseSSLMode(v, value(v, "NEXUSRELAY_ENV", "development"))
	if err != nil {
		return Database{}, err
	}
	statement, err := duration(v, "DATABASE_STATEMENT_TIMEOUT", 30*time.Second, time.Millisecond, 30*time.Minute)
	if err != nil {
		return Database{}, err
	}
	transaction, err := duration(v, "DATABASE_TRANSACTION_TIMEOUT", 30*time.Second, time.Millisecond, 30*time.Minute)
	if err != nil {
		return Database{}, err
	}
	if value(v, settings.userSetting, settings.user) != settings.user {
		return Database{}, fmt.Errorf("%s must equal %s", settings.userSetting, settings.user)
	}
	min, err := integer(v, settings.minSetting, 2, 0, 10000)
	if err != nil {
		return Database{}, err
	}
	max, err := integer(v, settings.maxSetting, settings.defaultMax, 1, 10000)
	if err != nil {
		return Database{}, err
	}
	if min > max {
		return Database{}, fmt.Errorf("%s must not exceed %s", settings.minSetting, settings.maxSetting)
	}
	password, err := ReadSimpleSecret(settings.passwordSetting, requiredPath(v, settings.passwordSetting, defaultPasswordPath(settings.passwordSetting)), 4*1024)
	if err != nil {
		return Database{}, err
	}
	return Database{host, port, databaseName, settings.user, password, sslmode, min, max, statement, transaction}, nil
}

func parseRedis(v map[string]string, maxSetting string, defaultMax int) (Redis, error) {
	secret, parsed, err := ReadURISecret("REDIS_URL_FILE", requiredPath(v, "REDIS_URL_FILE", "/run/secrets/redis_url"), 4*1024, "redis", "rediss")
	if err != nil {
		return Redis{}, err
	}
	if parsed.Host == "" {
		return Redis{}, fmt.Errorf("REDIS_URL_FILE must contain an absolute Redis URI")
	}
	max, err := integer(v, maxSetting, defaultMax, 1, 10000)
	if err != nil {
		return Redis{}, err
	}
	return Redis{secret, max}, nil
}

func parseSessionPolicy(v map[string]string) (SessionPolicy, error) {
	idle, err := duration(v, "SESSION_IDLE_TTL", 12*time.Hour, time.Minute, 30*24*time.Hour)
	if err != nil {
		return SessionPolicy{}, err
	}
	abs, err := duration(v, "SESSION_ABSOLUTE_TTL", 168*time.Hour, time.Minute, 365*24*time.Hour)
	if err != nil {
		return SessionPolicy{}, err
	}
	interval, err := duration(v, "SESSION_LAST_SEEN_WRITE_INTERVAL", 5*time.Minute, time.Second, idle)
	if err != nil {
		return SessionPolicy{}, err
	}
	if idle > abs {
		return SessionPolicy{}, fmt.Errorf("SESSION_IDLE_TTL must not exceed SESSION_ABSOLUTE_TTL")
	}
	cookie := value(v, "SESSION_COOKIE_NAME", "nexusrelay_session")
	if !regexp.MustCompile(`^[!#$%&'*+.^_` + "`" + `|~0-9A-Za-z-]+$`).MatchString(cookie) {
		return SessionPolicy{}, fmt.Errorf("SESSION_COOKIE_NAME is not a valid cookie name")
	}
	passwordPolicy, err := parsePasswordPolicy(v)
	if err != nil {
		return SessionPolicy{}, err
	}
	secret, err := ReadBase64Key("SESSION_SECRET_FILE", requiredPath(v, "SESSION_SECRET_FILE", "/run/secrets/session_secret"), 4*1024)
	if err != nil {
		return SessionPolicy{}, err
	}
	csrf, err := ReadKeyRing("CSRF_SECRET_RING_FILE", requiredPath(v, "CSRF_SECRET_RING_FILE", "/run/secrets/csrf_secret_ring"))
	if err != nil {
		return SessionPolicy{}, err
	}
	return SessionPolicy{cookie, idle, abs, interval, secret, csrf, passwordPolicy.Argon2MemoryKiB, passwordPolicy.Argon2Iterations, passwordPolicy.Argon2Parallelism}, nil
}

func parsePasswordPolicy(v map[string]string) (PasswordPolicy, error) {
	memory, err := integer(v, "PASSWORD_ARGON2_MEMORY_KIB", 65536, 8, 4*1024*1024)
	if err != nil {
		return PasswordPolicy{}, err
	}
	iterations, err := integer(v, "PASSWORD_ARGON2_ITERATIONS", 3, 1, 100)
	if err != nil {
		return PasswordPolicy{}, err
	}
	parallelism, err := integer(v, "PASSWORD_ARGON2_PARALLELISM", 2, 1, 255)
	if err != nil {
		return PasswordPolicy{}, err
	}
	if memory < 8*parallelism {
		return PasswordPolicy{}, fmt.Errorf("PASSWORD_ARGON2_MEMORY_KIB must be at least 8 times PASSWORD_ARGON2_PARALLELISM")
	}
	return PasswordPolicy{uint32(memory), uint32(iterations), uint8(parallelism)}, nil
}

func parseGatewayPolicy(v map[string]string) (GatewayPolicy, error) {
	body, err := int64Value(v, "REQUEST_BODY_MAX_BYTES", 10485760, 1, 1024*1024*1024)
	if err != nil {
		return GatewayPolicy{}, err
	}
	connect, err := duration(v, "UPSTREAM_CONNECT_TIMEOUT", 10*time.Second, time.Millisecond, 10*time.Minute)
	if err != nil {
		return GatewayPolicy{}, err
	}
	first, err := duration(v, "UPSTREAM_FIRST_BYTE_TIMEOUT", 60*time.Second, time.Millisecond, 30*time.Minute)
	if err != nil {
		return GatewayPolicy{}, err
	}
	idle, err := duration(v, "UPSTREAM_IDLE_TIMEOUT", 60*time.Second, time.Millisecond, 30*time.Minute)
	if err != nil {
		return GatewayPolicy{}, err
	}
	total, err := duration(v, "UPSTREAM_TOTAL_TIMEOUT", 10*time.Minute, time.Millisecond, 24*time.Hour)
	if err != nil {
		return GatewayPolicy{}, err
	}
	if connect > total || first > total || idle > total {
		return GatewayPolicy{}, fmt.Errorf("each upstream stage timeout must not exceed UPSTREAM_TOTAL_TIMEOUT")
	}
	attempts, err := integer(v, "MAX_UPSTREAM_ATTEMPTS", 3, 1, 20)
	if err != nil {
		return GatewayPolicy{}, err
	}
	return GatewayPolicy{body, connect, first, idle, total, attempts}, nil
}

func parseWorkerPolicy(v map[string]string) (WorkerPolicy, error) {
	requestDays, err := integer(v, "REQUEST_RETENTION_DAYS", 90, 1, 36500)
	if err != nil {
		return WorkerPolicy{}, err
	}
	auditDays, err := integer(v, "AUDIT_RETENTION_DAYS", 365, 1, 36500)
	if err != nil {
		return WorkerPolicy{}, err
	}
	healthDays, err := integer(v, "HEALTH_OBSERVATION_RETENTION_DAYS", 14, 1, 36500)
	if err != nil {
		return WorkerPolicy{}, err
	}
	outboxDays, err := integer(v, "PROCESSED_OUTBOX_RETENTION_DAYS", 7, 1, 36500)
	if err != nil {
		return WorkerPolicy{}, err
	}
	lag, err := duration(v, "ANALYTICS_TARGET_LAG", 5*time.Minute, time.Second, 24*time.Hour)
	if err != nil {
		return WorkerPolicy{}, err
	}
	reconcile, err := duration(v, "STALE_REQUEST_RECONCILIATION_INTERVAL", time.Minute, time.Second, 24*time.Hour)
	if err != nil {
		return WorkerPolicy{}, err
	}
	concurrency, err := integer(v, "WORKER_CONCURRENCY", 4, 1, 1024)
	if err != nil {
		return WorkerPolicy{}, err
	}
	lease, err := duration(v, "OUTBOX_LEASE_DURATION", 30*time.Second, time.Second, time.Hour)
	if err != nil {
		return WorkerPolicy{}, err
	}
	base, err := duration(v, "OUTBOX_RETRY_BASE_DELAY", time.Second, time.Millisecond, time.Hour)
	if err != nil {
		return WorkerPolicy{}, err
	}
	max, err := duration(v, "OUTBOX_RETRY_MAX_DELAY", 5*time.Minute, time.Millisecond, 24*time.Hour)
	if err != nil {
		return WorkerPolicy{}, err
	}
	if base > max {
		return WorkerPolicy{}, fmt.Errorf("OUTBOX_RETRY_BASE_DELAY must not exceed OUTBOX_RETRY_MAX_DELAY")
	}
	probe, err := duration(v, "HEALTH_PROBE_INTERVAL", time.Minute, time.Second, 24*time.Hour)
	if err != nil {
		return WorkerPolicy{}, err
	}
	fast, err := duration(v, "HEALTH_FAST_WINDOW", 5*time.Minute, time.Second, 24*time.Hour)
	if err != nil {
		return WorkerPolicy{}, err
	}
	stable, err := duration(v, "HEALTH_STABILIZING_WINDOW", 30*time.Minute, time.Second, 7*24*time.Hour)
	if err != nil {
		return WorkerPolicy{}, err
	}
	if fast > stable {
		return WorkerPolicy{}, fmt.Errorf("HEALTH_FAST_WINDOW must not exceed HEALTH_STABILIZING_WINDOW")
	}
	samples, err := integer(v, "HEALTH_MIN_SAMPLES", 20, 1, 1000000)
	if err != nil {
		return WorkerPolicy{}, err
	}
	version, err := integer(v, "HEALTH_POLICY_VERSION", 1, 1, 2147483647)
	if err != nil {
		return WorkerPolicy{}, err
	}
	entryThreshold, err := integer(v, "HEALTH_UNAVAILABLE_ENTRY_THRESHOLD_BPS", 5000, 0, 10000)
	if err != nil {
		return WorkerPolicy{}, err
	}
	exitThreshold, err := integer(v, "HEALTH_UNAVAILABLE_EXIT_THRESHOLD_BPS", 9000, 0, 10000)
	if err != nil {
		return WorkerPolicy{}, err
	}
	if entryThreshold >= exitThreshold {
		return WorkerPolicy{}, fmt.Errorf("HEALTH_UNAVAILABLE_ENTRY_THRESHOLD_BPS must be less than HEALTH_UNAVAILABLE_EXIT_THRESHOLD_BPS")
	}
	consecutiveFailures, err := integer(v, "HEALTH_CONSECUTIVE_FAILURES", 3, 1, 1000)
	if err != nil {
		return WorkerPolicy{}, err
	}
	consecutiveSuccesses, err := integer(v, "HEALTH_CONSECUTIVE_SUCCESSES", 3, 1, 1000)
	if err != nil {
		return WorkerPolicy{}, err
	}
	batch, err := integer(v, "RETENTION_BATCH_SIZE", 1000, 1, 100000)
	if err != nil {
		return WorkerPolicy{}, err
	}
	maximumRequestDuration, err := duration(v, "UPSTREAM_TOTAL_TIMEOUT", 10*time.Minute, time.Millisecond, 24*time.Hour)
	if err != nil {
		return WorkerPolicy{}, err
	}
	return WorkerPolicy{requestDays, auditDays, healthDays, outboxDays, lag, reconcile, concurrency, lease, base, max, probe, fast, stable, samples, version, entryThreshold, exitThreshold, consecutiveFailures, consecutiveSuccesses, batch, maximumRequestDuration}, nil
}

func parseAgent(v map[string]string) (AgentExport, error) {
	env := value(v, "AGENT_API_KEY_ENV", "NEXUSRELAY_API_KEY")
	if !regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`).MatchString(env) {
		return AgentExport{}, fmt.Errorf("AGENT_API_KEY_ENV must be an environment variable name")
	}
	id := value(v, "OPENCODE_PROVIDER_ID", "nexusrelay")
	if !regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`).MatchString(id) {
		return AgentExport{}, fmt.Errorf("OPENCODE_PROVIDER_ID has invalid format")
	}
	name := value(v, "OPENCODE_PROVIDER_NAME", "NexusRelay")
	if err := cleanNonempty("OPENCODE_PROVIDER_NAME", name); err != nil {
		return AgentExport{}, err
	}
	return AgentExport{env, id, name}, nil
}

func parseObservability(v map[string]string) (Observability, error) {
	logging, err := parseLogging(v)
	if err != nil {
		return Observability{}, err
	}
	metrics, err := boolean(v, "METRICS_ENABLED", true)
	if err != nil {
		return Observability{}, err
	}
	otel, err := boolean(v, "OTEL_ENABLED", false)
	if err != nil {
		return Observability{}, err
	}
	var endpoint *url.URL
	if raw := value(v, "OTEL_EXPORTER_OTLP_ENDPOINT", ""); raw != "" {
		endpoint, err = absoluteURL("OTEL_EXPORTER_OTLP_ENDPOINT", raw)
		if err != nil {
			return Observability{}, err
		}
		if endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
			return Observability{}, fmt.Errorf("OTEL_EXPORTER_OTLP_ENDPOINT must not contain userinfo, query, or fragment")
		}
	}
	if otel && endpoint == nil {
		return Observability{}, fmt.Errorf("OTEL_EXPORTER_OTLP_ENDPOINT is required when OTEL_ENABLED=true")
	}
	logging.Metrics = metrics
	logging.OTel = otel
	logging.OTLPEndpoint = endpoint
	return logging, nil
}

func parseLogging(v map[string]string) (Observability, error) {
	level := value(v, "LOG_LEVEL", "info")
	if !oneOf(level, "debug", "info", "warn", "error") {
		return Observability{}, fmt.Errorf("LOG_LEVEL must be debug, info, warn, or error")
	}
	format := value(v, "LOG_FORMAT", "json")
	if !oneOf(format, "json", "text") {
		return Observability{}, fmt.Errorf("LOG_FORMAT must be json or text")
	}
	return Observability{LogLevel: level, LogFormat: format}, nil
}

func parseOrigins(raw string) ([]*url.URL, error) {
	if raw == "" {
		return nil, fmt.Errorf("ADMIN_ORIGINS must contain at least one explicit origin")
	}
	parts := strings.Split(raw, ",")
	result := make([]*url.URL, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		if strings.TrimSpace(part) != part || part == "" {
			return nil, fmt.Errorf("ADMIN_ORIGINS must be comma-separated origins without whitespace")
		}
		origin, err := originURL("ADMIN_ORIGINS", part)
		if err != nil {
			return nil, err
		}
		if seen[origin.String()] {
			return nil, fmt.Errorf("ADMIN_ORIGINS contains a duplicate origin")
		}
		seen[origin.String()] = true
		result = append(result, origin)
	}
	return result, nil
}

func parseCIDRs(raw string) ([]*net.IPNet, error) {
	return parseNamedCIDRs("TRUSTED_PROXY_CIDRS", raw)
}

func parseNamedCIDRs(name, raw string) ([]*net.IPNet, error) {
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	result := make([]*net.IPNet, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) != part || part == "" {
			return nil, fmt.Errorf("%s must be comma-separated CIDRs without whitespace", name)
		}
		_, network, err := net.ParseCIDR(part)
		if err != nil {
			return nil, fmt.Errorf("%s contains an invalid CIDR", name)
		}
		result = append(result, network)
	}
	return result, nil
}

func absoluteURL(name, raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() || u.Host == "" || !oneOf(u.Scheme, "http", "https") {
		return nil, fmt.Errorf("%s must be an absolute HTTP(S) URL", name)
	}
	return u, nil
}

func originURL(name, raw string) (*url.URL, error) {
	u, err := absoluteURL(name, raw)
	if err != nil {
		return nil, err
	}
	if u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("%s must be an origin without path, query, fragment, or userinfo", name)
	}
	u.Path = ""
	return u, nil
}

func duration(v map[string]string, name string, fallback, min, max time.Duration) (time.Duration, error) {
	parsed, err := time.ParseDuration(value(v, name, fallback.String()))
	if err != nil || parsed < min || parsed > max {
		return 0, fmt.Errorf("%s must be between %s and %s", name, min, max)
	}
	return parsed, nil
}

func integer(v map[string]string, name string, fallback, min, max int) (int, error) {
	n, err := strconv.Atoi(value(v, name, strconv.Itoa(fallback)))
	if err != nil || n < min || n > max {
		return 0, fmt.Errorf("%s must be an integer from %d through %d", name, min, max)
	}
	return n, nil
}

func int64Value(v map[string]string, name string, fallback, min, max int64) (int64, error) {
	n, err := strconv.ParseInt(value(v, name, strconv.FormatInt(fallback, 10)), 10, 64)
	if err != nil || n < min || n > max {
		return 0, fmt.Errorf("%s must be an integer from %d through %d", name, min, max)
	}
	return n, nil
}

func boolean(v map[string]string, name string, fallback bool) (bool, error) {
	raw := value(v, name, strconv.FormatBool(fallback))
	if raw != "true" && raw != "false" {
		return false, fmt.Errorf("%s must be true or false", name)
	}
	return raw == "true", nil
}

func requiredText(v map[string]string, name string) (string, error) {
	raw, ok := v[name]
	if !ok || raw == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	if err := cleanNonempty(name, raw); err != nil {
		return "", err
	}
	return raw, nil
}
func requiredPath(v map[string]string, name, fallback string) string { return value(v, name, fallback) }
func value(v map[string]string, name, fallback string) string {
	if raw, ok := v[name]; ok {
		return raw
	}
	return fallback
}
func cleanNonempty(name, raw string) error {
	if raw == "" || strings.TrimSpace(raw) != raw {
		return fmt.Errorf("%s must be non-empty and contain no surrounding whitespace", name)
	}
	return nil
}
func oneOf(value string, choices ...string) bool {
	for _, choice := range choices {
		if value == choice {
			return true
		}
	}
	return false
}
func defaultPasswordPath(setting string) string {
	return map[string]string{
		"DATABASE_GATEWAY_PASSWORD_FILE":       "/run/secrets/postgres_gateway_password",
		"DATABASE_CONTROL_PLANE_PASSWORD_FILE": "/run/secrets/postgres_control_plane_password",
		"DATABASE_WORKER_PASSWORD_FILE":        "/run/secrets/postgres_worker_password",
	}[setting]
}
func readableRegular(name, path string) error {
	if path == "" {
		return fmt.Errorf("%s is required", name)
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("%s must name a readable regular file", name)
	}
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("%s must name a readable regular file", name)
	}
	file.Close()
	return nil
}

func validateEnvironment(environment string) error {
	if !oneOf(environment, "development", "test", "production") {
		return fmt.Errorf("NEXUSRELAY_ENV must be development, test, or production")
	}
	return nil
}

func isProductionGrade(environment string) bool { return environment == "production" }

func parseDatabaseSSLMode(v map[string]string, environment string) (string, error) {
	sslmode := value(v, "DATABASE_SSLMODE", "disable")
	if !oneOf(sslmode, "disable", "require", "verify-ca", "verify-full") {
		return "", fmt.Errorf("DATABASE_SSLMODE must be disable, require, verify-ca, or verify-full")
	}
	if isProductionGrade(environment) && sslmode == "disable" {
		return "", fmt.Errorf("production DATABASE_SSLMODE must be require, verify-ca, or verify-full")
	}
	return sslmode, nil
}

func validatePublicTunnelHosts(endpoints Endpoints) error {
	if !strings.EqualFold(endpoints.PublicAPIHost, endpoints.AdminHost) {
		return nil
	}
	if endpoints.AdminExposure == "private" {
		return fmt.Errorf("ENABLE_CLOUDFLARE_TUNNEL cannot publish the private ADMIN_HOST")
	}
	return fmt.Errorf("ENABLE_CLOUDFLARE_TUNNEL requires PUBLIC_API_HOST and ADMIN_HOST to differ")
}

func environment() map[string]string {
	values := make(map[string]string)
	for _, entry := range os.Environ() {
		name, value, ok := strings.Cut(entry, "=")
		if ok {
			values[name] = value
		}
	}
	return values
}
