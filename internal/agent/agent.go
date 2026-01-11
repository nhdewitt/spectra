package agent

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/nhdewitt/spectra/internal/collector"
	"github.com/nhdewitt/spectra/internal/protocol"
)

// Config holds the runtime configuration
type Config struct {
	BaseURL      string
	Hostname     string
	MetricsPath  string
	CommandPath  string
	PollInterval time.Duration
}

// Agent is the main application controller
type Agent struct {
	Config     Config
	Client     *http.Client
	DriveCache *collector.DriveCache

	metricsCh chan protocol.Envelope
	wg        sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
}

// New creates a configured Agent instance
func New(cfg Config) *Agent {
	client := &http.Client{
		Timeout: 45 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Agent{
		Config:     cfg,
		Client:     client,
		DriveCache: collector.NewDriveCache(),
		metricsCh:  make(chan protocol.Envelope, 500),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start initializes all subsystems and blocks until Shutdown is called
func (a *Agent) Start() error {
	fmt.Printf("Spectra Agent starting on %s...\n", a.Config.Hostname)
	fmt.Printf("Server: %s\n", a.Config.BaseURL)

	// Mount Manager (Windows disk mapping)
	go collector.RunMountManager(a.ctx, a.DriveCache, 30*time.Second)

	// Metric Sender
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.runMetricSender()
	}()

	// Start Collectors
	a.startCollectors()

	// Start Command Loop
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.runCommandLoop()
	}()

	// Register Identity
	if err := a.Register(); err != nil {
		fmt.Printf("Initial registration failed: %v", err)
	}

	// Block until shutdown called
	<-a.ctx.Done()
	return nil
}

// Shutdown gracefully stops all background tasks
func (a *Agent) Shutdown() {
	fmt.Println("Agent shutting down...")
	a.cancel()
	a.wg.Wait()
	fmt.Println("Agent stopped.")
}
