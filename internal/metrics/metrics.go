package metrics

import (
	"database/sql"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTP request metrics
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status_code"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint", "status_code"},
	)

	// Database connection pool metrics
	DBMaxOpenConns = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "db_max_open_connections",
			Help: "Maximum number of open database connections",
		},
		[]string{"database"},
	)

	DBOpenConns = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "db_open_connections",
			Help: "Number of open database connections",
		},
		[]string{"database"},
	)

	DBIdleConns = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "db_idle_connections",
			Help: "Number of idle database connections",
		},
		[]string{"database"},
	)

	DBInUseConns = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "db_in_use_connections",
			Help: "Number of database connections in use",
		},
		[]string{"database"},
	)

	DBWaitCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_wait_count_total",
			Help: "Total number of connections waited for",
		},
		[]string{"database"},
	)

	DBWaitDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_wait_duration_seconds",
			Help:    "Total time blocked waiting for a new connection",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"database"},
	)

	DBMaxIdleClosed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_max_idle_closed_total",
			Help: "Total number of connections closed due to SetMaxIdleConns",
		},
		[]string{"database"},
	)

	DBMaxLifetimeClosed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_max_lifetime_closed_total",
			Help: "Total number of connections closed due to SetConnMaxLifetime",
		},
		[]string{"database"},
	)

	// Business metrics
	HostsIngestedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hosts_ingested_total",
			Help: "Total number of hosts ingested",
		},
		[]string{"org_id"},
	)

	APIKeysCreatedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_keys_created_total",
			Help: "Total number of API keys created",
		},
		[]string{"org_id"},
	)

	APIKeysUsedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_keys_used_total",
			Help: "Total number of API key authentications",
		},
		[]string{"org_id"},
	)
)

// RegisterDBMetrics registers database connection pool metrics
func RegisterDBMetrics(db *sql.DB, dbName string) {
	go func() {
		stats := db.Stats()
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			stats = db.Stats()

			DBMaxOpenConns.WithLabelValues(dbName).Set(float64(stats.MaxOpenConnections))
			DBOpenConns.WithLabelValues(dbName).Set(float64(stats.OpenConnections))
			DBIdleConns.WithLabelValues(dbName).Set(float64(stats.Idle))
			DBInUseConns.WithLabelValues(dbName).Set(float64(stats.InUse))
			DBWaitCount.WithLabelValues(dbName).Add(float64(stats.WaitCount))
			DBWaitDuration.WithLabelValues(dbName).Observe(stats.WaitDuration.Seconds())
			DBMaxIdleClosed.WithLabelValues(dbName).Add(float64(stats.MaxIdleClosed))
			DBMaxLifetimeClosed.WithLabelValues(dbName).Add(float64(stats.MaxLifetimeClosed))
		}
	}()
}
