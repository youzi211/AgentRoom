package logging

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestLogger returns a Gin middleware that logs each HTTP request via slog.
// Fields logged: method, path, status, latency_ms, client_ip, user_agent.
// Level selection: 5xx = Error, 4xx = Warn, other = Info.
// Request bodies and sensitive data are never logged.
func RequestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		event := logger.Info
		if status >= 500 {
			event = logger.Error
		} else if status >= 400 {
			event = logger.Warn
		}

		event("HTTP request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", status,
			"latency_ms", latency.Milliseconds(),
			"client_ip", c.ClientIP(),
			"user_agent", c.Request.UserAgent(),
		)
	}
}

// Recovery returns a Gin middleware that recovers from panics and logs them
// via slog as Error level, then returns HTTP 500.
func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("panic recovered",
					"error", r,
					"stack", string(debug.Stack()),
					"path", c.Request.URL.Path,
					"method", c.Request.Method,
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": "internal server error",
				})
			}
		}()
		c.Next()
	}
}
