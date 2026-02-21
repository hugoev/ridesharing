// Package middleware provides Gin middleware for auth, rate limiting, request tracing, and recovery.
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"

	apperrors "github.com/hugovillarreal/ridesharing/pkg/errors"
	jwtutil "github.com/hugovillarreal/ridesharing/pkg/jwt"
	"github.com/hugovillarreal/ridesharing/pkg/tracing"
)

// TraceID extracts the OpenTelemetry trace ID and sets it as a response header.
func TraceID() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if tid := tracing.TraceID(c.Request.Context()); tid != "" {
			c.Header("X-Trace-ID", tid)
		}
	}
}

// RequestID injects a unique request ID into each request context and response header.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" {
			id = uuid.New().String()
		}
		c.Set("request_id", id)
		c.Header("X-Request-ID", id)
		c.Next()
	}
}

// Recovery catches panics and returns a 500 error with logging.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				log.Error().
					Interface("panic", r).
					Str("path", c.Request.URL.Path).
					Msg("panic recovered")
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error":      "internal server error",
					"error_code": apperrors.CodeInternal,
				})
			}
		}()
		c.Next()
	}
}

// RateLimit applies a token-bucket rate limiter.
func RateLimit(rps float64, burst int) gin.HandlerFunc {
	limiter := rate.NewLimiter(rate.Limit(rps), burst)
	return func(c *gin.Context) {
		if !limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":      "rate limit exceeded",
				"error_code": apperrors.CodeRateLimited,
			})
			return
		}
		c.Next()
	}
}

// MaxBodySize limits the size of request bodies. Requests exceeding the
// limit are rejected with a 413 Payload Too Large response.
func MaxBodySize(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}

// CORS adds Cross-Origin Resource Sharing headers.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
		c.Header("Access-Control-Max-Age", "86400")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// Auth validates JWT tokens and sets user info in the context.
// It expects the Authorization header in "Bearer <token>" format.
func Auth(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":      "authorization header required",
				"error_code": apperrors.CodeUnauthorized,
			})
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":      "authorization header must be Bearer <token>",
				"error_code": apperrors.CodeUnauthorized,
			})
			return
		}

		claims, err := jwtutil.ValidateToken(parts[1], jwtSecret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":      "invalid or expired token",
				"error_code": apperrors.CodeUnauthorized,
			})
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("user_role", claims.Role)
		c.Next()
	}
}

// RequireRole ensures the authenticated user has the required role.
func RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("user_role")
		if !exists || userRole.(string) != role {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":      "insufficient permissions",
				"error_code": apperrors.CodeForbidden,
			})
			return
		}
		c.Next()
	}
}
