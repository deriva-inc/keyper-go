package handlers

import (
	"net/http"

	"github.com/deriva-inc/keyper-go/config"
	"github.com/deriva-inc/keyper-go/db"
	"github.com/gin-gonic/gin"
)

// HealthCheck handles GET /api/v1/health
// Returns overall server status, database state, and runtime build metadata.
func HealthCheck(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		dbStatus := "UP"
		if database != nil && database.DB != nil {
			if err := database.Ping(); err != nil {
				dbStatus = "DOWN"
			}
		} else {
			dbStatus = "UNAVAILABLE"
		}

		c.JSON(http.StatusOK, gin.H{
			"status":    "UP",
			"version":   config.Version,
			"commit":    config.GitCommit,
			"buildDate": config.BuildDate,
			"database":  dbStatus,
		})
	}
}

// LivenessProbe handles GET /api/v1/health/live
// Used by Kubernetes to determine if the container process is alive and responsive.
// If this probe fails repeatedly, Kubernetes restarts the pod container.
func LivenessProbe() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ALIVE",
		})
	}
}

// ReadinessProbe handles GET /api/v1/health/ready
// Used by Kubernetes to determine if the container is ready to accept user network traffic.
// If the database connection fails, it returns HTTP 503 so Kubernetes removes the pod from
// the Service load balancer endpoints without terminating the container.
func ReadinessProbe(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if database == nil || database.DB == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "NOT_READY",
				"error":  "Database connection not initialized",
			})
			return
		}

		if err := database.Ping(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "NOT_READY",
				"error":  "Database unreachable: " + err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":   "READY",
			"database": "CONNECTED",
		})
	}
}

// StartupProbe handles GET /api/v1/health/startup
// Used by Kubernetes for slow-starting applications to verify initialization
// and database connectivity before liveness and readiness probes become active.
func StartupProbe(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if database == nil || database.DB == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "STARTING",
				"error":  "Database not connected yet",
			})
			return
		}

		if err := database.Ping(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "STARTING",
				"error":  "Waiting for database: " + err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "STARTED",
		})
	}
}
