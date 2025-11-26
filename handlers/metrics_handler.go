package handlers

import (
	"github.com/alexander-bruun/magi/models"
	"github.com/gofiber/adaptor/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	fiber "github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
)

// Prometheus metrics
var (
	totalMedias = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "magi_total_medias",
		Help: "Total number of media items",
	})

	totalChapters = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "magi_total_chapters",
		Help: "Total number of chapters",
	})

	totalChaptersRead = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "magi_total_chapters_read",
		Help: "Total number of chapters read",
	})

	totalUsers = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "magi_total_users",
		Help: "Total number of users",
	})

	totalLibraries = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "magi_total_libraries",
		Help: "Total number of libraries",
	})
)

func init() {
	// Register metrics
	prometheus.MustRegister(totalMedias)
	prometheus.MustRegister(totalChapters)
	prometheus.MustRegister(totalChaptersRead)
	prometheus.MustRegister(totalUsers)
	prometheus.MustRegister(totalLibraries)
}

// updateMetrics updates all Prometheus metrics with current database values
func updateMetrics() {
	// Update media count
	if count, err := models.GetTotalMedias(); err == nil {
		totalMedias.Set(float64(count))
	} else {
		log.Warnf("Failed to get total medias for metrics: %v", err)
	}

	// Update chapter count
	if count, err := models.GetTotalChapters(); err == nil {
		totalChapters.Set(float64(count))
	} else {
		log.Warnf("Failed to get total chapters for metrics: %v", err)
	}

	// Update chapters read count
	if count, err := models.GetTotalChaptersRead(); err == nil {
		totalChaptersRead.Set(float64(count))
	} else {
		log.Warnf("Failed to get total chapters read for metrics: %v", err)
	}

	// Update user count
	if count, err := models.CountUsers(); err == nil {
		totalUsers.Set(float64(count))
	} else {
		log.Warnf("Failed to get total users for metrics: %v", err)
	}

	// Update library count
	if libraries, err := models.GetLibraries(); err == nil {
		totalLibraries.Set(float64(len(libraries)))
	} else {
		log.Warnf("Failed to get total libraries for metrics: %v", err)
	}
}

// HandleMetrics serves Prometheus metrics
func HandleMetrics(c *fiber.Ctx) error {
	// Update metrics before serving
	updateMetrics()

	// Use Fiber adaptor to convert HTTP handler to Fiber handler
	return adaptor.HTTPHandler(promhttp.Handler())(c)
}

// HandleReady serves the readiness endpoint
func HandleReady(c *fiber.Ctx) error {
	// Check if database is accessible - basic connectivity check
	if err := models.PingDB(); err != nil {
		log.Errorf("Database not ready: %v", err)
		return c.Status(fiber.StatusServiceUnavailable).SendString("NOT READY")
	}

	return c.SendString("OK")
}

// HandleHealth serves the health endpoint
func HandleHealth(c *fiber.Ctx) error {
	// Check database connectivity
	if err := models.PingDB(); err != nil {
		log.Errorf("Database health check failed: %v", err)
		return c.Status(fiber.StatusServiceUnavailable).SendString("UNHEALTHY")
	}

	// Additional health check: try to perform a simple database query
	if _, err := models.GetTotalMedias(); err != nil {
		log.Errorf("Database query health check failed: %v", err)
		return c.Status(fiber.StatusServiceUnavailable).SendString("UNHEALTHY")
	}

	return c.SendString("OK")
}