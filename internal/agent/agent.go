package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/nhdewitt/spectra/internal/collector/disk"
	"github.com/nhdewitt/spectra/internal/platform"
	"github.com/nhdewitt/spectra/internal/protocol"
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
}

// Agent is the main application controller
type Agent struct {
	Config     Config
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

	RetryConfig RetryConfig

	Platform platform.Info
	Identity Identity
}

type RetryConfig struct {
	MaxAttempts  int
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
	client := &http.Client{
		Timeout: 45 * time.Second,
	}

	if cfg.IdentityPath == "" {
		cfg.IdentityPath = identityPath()
	}

	id, err := loadIdentity(cfg.IdentityPath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("warning: failed to load identity: %v", err)
		}
	}

	return &Agent{
		Config:     cfg,
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
			"User-Agent":       "Spectra-Agent/1.0",
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

	log.Printf("Spectra Agent starting on %s...\n", a.Config.Hostname)
	log.Printf("Server: %s\n", a.Config.BaseURL)

	if a.Identity.ID == "" {
		if err := a.Register(ctx); err != nil {
			return fmt.Errorf("registration failed: %w", err)
		}
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
	log.Println("Agent shutting down...")
	a.cancel()
	a.wg.Wait()
	log.Println("Agent stopped.")
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
