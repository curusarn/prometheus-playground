package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/pelletier/go-toml/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Configuration structure matching the TOML file
type Config struct {
	UpServices   []string `toml:"up_services"`
	DownServices []string `toml:"down_services"`
}

var (
	requestsProcessed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "service_monitor_requests_total",
		Help: "The total number of processed requests",
	})

	requestDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "service_monitor_request_duration_seconds",
		Help:    "Request duration distribution",
		Buckets: prometheus.LinearBuckets(0.01, 0.05, 10),
	})

	activeRequests = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "service_monitor_active_requests",
		Help: "Number of active requests",
	})

	errorRate = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "service_monitor_error_rate",
		Help: "Current error rate",
	})

	// Define service status gauge vector
	serviceStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "service_monitor_up",
			Help: "Status of monitored services (1=up, 0=down)",
		},
		[]string{"service"},
	)

	// Configuration file path (default, can be overridden by environment variable)
	configPath = "/app/config/config.toml"

	// Last modification time
	lastModTime time.Time

	// Mutex for thread-safe operations
	configMutex sync.RWMutex
)

func init() {
	prometheus.MustRegister(requestsProcessed)
	prometheus.MustRegister(requestDuration)
	prometheus.MustRegister(activeRequests)
	prometheus.MustRegister(errorRate)
	prometheus.MustRegister(serviceStatus)

	// Seed the random number generator
	rand.Seed(time.Now().UnixNano())
}

// loadConfig reads the configuration file and returns the Config
// It opens and closes the file for each read to ensure we get the latest content
func loadConfig() (*Config, error) {
	// Open the file explicitly so it's closed after reading
	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("error opening config file: %w", err)
	}
	defer file.Close()

	// Read the file content
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	var config Config
	if err := toml.Unmarshal(configData, &config); err != nil {
		return nil, fmt.Errorf("error parsing config file: %w", err)
	}

	return &config, nil
}

// updateServiceMetrics updates the Prometheus metrics based on service status
func updateServiceMetrics(config *Config) {
	// Reset existing metrics
	serviceStatus.Reset()

	// Set up services as 1
	for _, service := range config.UpServices {
		serviceStatus.WithLabelValues(service).Set(1)
	}

	// Set down services as 0
	for _, service := range config.DownServices {
		serviceStatus.WithLabelValues(service).Set(0)
	}
}

// watchConfig monitors the config file for changes and reloads it
// The file is opened and closed on each check to ensure we detect changes
func watchConfig() {
	log.Printf("Starting config watcher for file: %s", configPath)
	checkInterval := 3 * time.Second // Check more frequently (3 seconds)
	
	for {
		// Check if file has been modified
		fileInfo, err := os.Stat(configPath)
		if err != nil {
			log.Printf("Error checking config file: %v", err)
			time.Sleep(checkInterval)
			continue
		}

		modTime := fileInfo.ModTime()
		if modTime != lastModTime {
			log.Println("Config file changed, reloading...")
			
			config, err := loadConfig()
			if err != nil {
				log.Printf("Error loading config: %v", err)
			} else {
				configMutex.Lock()
				updateServiceMetrics(config)
				lastModTime = modTime
				configMutex.Unlock()
				log.Printf("Reloaded config: %d up services and %d down services", 
					len(config.UpServices), len(config.DownServices))
			}
		}
		
		// Short sleep to be more responsive to changes
		time.Sleep(checkInterval)
	}
}

func main() {
	// Check for CONFIG_PATH environment variable
	if envPath := os.Getenv("CONFIG_PATH"); envPath != "" {
		configPath = envPath
		log.Printf("Using config path from environment: %s", configPath)
	}

	// Ensure config directory exists
	lastSlash := strings.LastIndex(configPath, "/")
	if lastSlash > 0 {
		configDir := configPath[:lastSlash]
		if _, err := os.Stat(configDir); os.IsNotExist(err) {
			log.Printf("Config directory %s does not exist, creating it", configDir)
			if err := os.MkdirAll(configDir, 0755); err != nil {
				log.Printf("Error creating config directory: %v", err)
			}
		}
	}

	// Check if config file exists, create default if not
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Printf("Config file %s does not exist, creating default", configPath)
		defaultConfig := `# Service Monitor Configuration

# Services that are currently up
up_services = [
  "api-gateway",
  "auth-service",
  "user-service",
  "payment-service"
]

# Services that are currently down
down_services = [
  "notification-service",
  "recommendation-engine"
]`
		if err := os.WriteFile(configPath, []byte(defaultConfig), 0644); err != nil {
			log.Printf("Error creating default config file: %v", err)
		}
	}

	// Initial config load
	config, err := loadConfig()
	if err != nil {
		log.Printf("Error loading initial config: %v", err)
		config = &Config{
			UpServices:   []string{"default-service"},
			DownServices: []string{},
		}
	} else {
		log.Printf("Loaded initial config with %d up services and %d down services", 
			len(config.UpServices), len(config.DownServices))
	}
	
	// Set initial last modified time
	fileInfo, err := os.Stat(configPath)
	if err == nil {
		lastModTime = fileInfo.ModTime()
	}
	
	// Initialize metrics with config
	updateServiceMetrics(config)
	
	// Start config watcher in background
	go watchConfig()

	// Health check endpoint
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		activeRequests.Inc()
		defer activeRequests.Dec()

		start := time.Now()
		defer func() {
			duration := time.Since(start).Seconds()
			requestDuration.Observe(duration)
			requestsProcessed.Inc()
		}()

		// Simulate some processing time
		processingTime := rand.Float64() * 0.5
		time.Sleep(time.Duration(processingTime * float64(time.Second)))

		// Randomly generate errors (10% of the time)
		if rand.Float64() < 0.1 {
			errorRate.Set(0.1)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error"))
			return
		}

		errorRate.Set(0.0)
		w.Write([]byte("Service Monitor is running!"))
	})
	
	// Config update endpoint
	http.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
		configMutex.RLock()
		defer configMutex.RUnlock()
		
		config, err := loadConfig()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Error loading config: %v", err)
			return
		}
		
		fmt.Fprintf(w, "UP SERVICES (%d):\n", len(config.UpServices))
		for _, svc := range config.UpServices {
			fmt.Fprintf(w, "- %s\n", svc)
		}
		
		fmt.Fprintf(w, "\nDOWN SERVICES (%d):\n", len(config.DownServices))
		for _, svc := range config.DownServices {
			fmt.Fprintf(w, "- %s\n", svc)
		}
	})

	// Metrics endpoint for Prometheus
	http.Handle("/metrics", promhttp.Handler())

	// Start a background routine to update general metrics
	go func() {
		for {
			// Simulate fluctuating load
			load := rand.Float64() * 10
			activeRequests.Set(load)
			time.Sleep(5 * time.Second)
		}
	}()

	log.Println("Starting Service Monitor on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}