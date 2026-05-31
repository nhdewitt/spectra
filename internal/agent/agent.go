package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/nhdewitt/spectra/internal/collector/disk"
	"github.com/nhdewitt/spectra/internal/logging"
	"github.com/nhdewitt/spectra/internal/platform"
	"github.com/nhdewitt/spectra/internal/protocol"
	"github.com/nhdewitt/spectra/internal/version"
)

// Config holds the runtime configuration
type Config struct {
	BaseURL           string
	Hostname          string
	MetricsPath       string
	CommandPath       string
	PollInterval      time.Duration
	RegistrationToken string
	IdentityPath      string
	AgentID           string // set after registration or loaded from config
	Secret            string // set after registration or loaded from config
	ConfigPath        string
	LogFile           string
	LogLevel          string
	CACert            string
	TLSSkipVerify     bool
}

// Agent is the main application controller
type Agent struct {
	Config     Config
	Logger     *logging.Logger
	Client     *http.Client
	DriveCache *disk.DriveCache

	metricsCh chan protocol.Envelope
	batch     []protocol.Envelope
	wg        sync.WaitGroup
	cancel    context.CancelFunc
	done      chan struct{}

	cache *metricsCache

	gzipMu  sync.Mutex
	gzipBuf bytes.Buffer
	gzipW   *gzip.Writer

	commonHeaders map[string]string

	RetryConfig  RetryConfig
	backoffUntil time.Time
	backoffStep  int

	Platform platform.Info
	Identity Identity

	BinaryHash string
}

type RetryConfig struct {
	MaxAttempts  int // Only used for registration, agents will retry forever
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
}

func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
	}
}

func (rc RetryConfig) Delay(attempt int) time.Duration {
	if attempt <= 0 {
		return rc.InitialDelay
	}

	delay := float64(rc.InitialDelay)
	for range attempt {
		delay *= rc.Multiplier
	}

	if time.Duration(delay) > rc.MaxDelay {
		return rc.MaxDelay
	}
	return time.Duration(delay)
}

// New creates a configured Agent instance
func New(cfg Config) *Agent {
	if cfg.IdentityPath == "" {
		cfg.IdentityPath = identityPath()
	}

	logCfg := logging.DefaultAgentConfig()
	if cfg.LogFile != "" {
		logCfg.FilePath = cfg.LogFile
	}
	if cfg.LogLevel != "" {
		logCfg.ConsoleLevel = logging.ParseLevel(cfg.LogLevel)
	}

	logger := logging.New(logCfg)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfigFromAgentConfig(cfg, logger)

	client := &http.Client{
		Timeout:   45 * time.Second,
		Transport: transport,
	}

	id, err := loadIdentity(cfg.IdentityPath)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warn("failed to load identity", "error", err)
		}
	}

	return &Agent{
		Config:     cfg,
		Logger:     logger,
		Client:     client,
		DriveCache: disk.NewDriveCache(),
		metricsCh:  make(chan protocol.Envelope, 500),
		batch:      make([]protocol.Envelope, 0, 50),
		cancel:     nil,
		done:       make(chan struct{}),
		cache:      newMetricsCache(defaultMaxCacheSize),
		gzipW:      gzip.NewWriter(io.Discard),
		commonHeaders: map[string]string{
			"Content-Type":     "application/json",
			"Content-Encoding": "gzip",
			"User-Agent":       version.UserAgent("Agent"),
			"X-Agent-Version":  version.Version,
			"X-Agent-Commit":   version.Commit,
		},
		RetryConfig: DefaultRetryConfig(),
		Platform:    platform.Detect(),
		Identity:    id,
	}
}

// Start initializes all subsystems and blocks until Shutdown is called
func (a *Agent) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel

	a.Logger.Info("agent starting",
		"hostname", a.Config.Hostname,
		"server", a.Config.BaseURL,
		"version", version.Full(),
	)

	if err := a.computeBinaryHash(); err != nil {
		a.Logger.Warn("failed to compute binary hash", "error", err)
	}
	if a.BinaryHash != "" {
		a.commonHeaders["X-Agent-Binary-Hash"] = a.BinaryHash
	}

	if a.Identity.ID == "" {
		if err := a.Register(ctx); err != nil {
			return fmt.Errorf("registration failed: %w", err)
		}
		a.Logger.Info("agent registered", "agent_id", a.Identity.ID)
	}

	// Mount Manager (Windows disk mapping)
	go disk.RunMountManager(ctx, a.DriveCache, 30*time.Second)

	// Metric Sender
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.runMetricSender(ctx)
	}()

	// Start Command Loop
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.runCommandLoop(ctx)
	}()

	// Start Config Poller
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.runConfigPoller(ctx)
	}()

	// Align to minute boundary
	if err := waitForNextMinute(ctx); err != nil {
		return fmt.Errorf("clock alignment cancelled: %w", err)
	}

	// Start Collectors
	a.startCollectors(ctx)

	// Block until shutdown called
	<-ctx.Done()
	return nil
}

// Shutdown gracefully stops all background tasks
func (a *Agent) Shutdown() {
	a.Logger.Info("agent shutting down")
	a.cancel()
	a.wg.Wait()
	a.Logger.Info("agent stopped")
	a.Logger.Close()
}

// setHeaders sets common headers for an http.Request
func (a *Agent) setHeaders(req *http.Request) {
	for k, v := range a.commonHeaders {
		req.Header.Set(k, v)
	}
	if a.Identity.ID != "" {
		req.Header.Set("X-Agent-ID", a.Identity.ID)
		req.Header.Set("X-Agent-Secret", a.Identity.Secret)
	}
}

func (a *Agent) computeBinaryHash() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("resolve symlinks: %w", err)
	}
	f, err := os.Open(exe)
	if err != nil {
		return fmt.Errorf("open executable: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("hash executable: %w", err)
	}
	a.BinaryHash = hex.EncodeToString(h.Sum(nil))
	a.Logger.Info("binary hash computed", "sha256", a.BinaryHash)
	return nil
}

func tlsConfigFromAgentConfig(cfg Config, logger *logging.Logger) *tls.Config {
	if cfg.TLSSkipVerify {
		logger.Warn("TLS verification disabled")
		return &tls.Config{
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionTLS12,
		}
	}

	if cfg.CACert == "" {
		return &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	caCert, err := os.ReadFile(cfg.CACert)
	if err != nil {
		logger.Error("failed to read CA cert, agent cannot connect to TLS server", "path", cfg.CACert, "error", err)
		os.Exit(1)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caCert) {
		logger.Error("CA cert does not contain valid PEM certificates", "path", cfg.CACert)
		os.Exit(1)
	}

	logger.Info("TLS CA loaded", "path", cfg.CACert)
	return &tls.Config{
		RootCAs:    pool,
		MinVersion: tls.VersionTLS12,
	}
}
