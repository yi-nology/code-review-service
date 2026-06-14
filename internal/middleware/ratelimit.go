package middleware

import (
	mw "github.com/oekit/middleware"
	"github.com/cloudwego/hertz/pkg/app"
	"golang.org/x/time/rate"
)

// RateLimitMiddleware creates a rate limiting middleware using ekit
func RateLimitMiddleware(rps int, burst int) app.HandlerFunc {
	return mw.RateLimitMiddleware(mw.RateLimitConfig{
		Rate:  rate.Limit(rps),
		Burst: burst,
	})
}

// PerUserRateLimitMiddleware creates a per-user rate limiting middleware
func PerUserRateLimitMiddleware(rps int, burst int) app.HandlerFunc {
	return mw.RateLimitMiddleware(mw.RateLimitConfig{
		Rate:  rate.Limit(rps),
		Burst: burst,
		KeyFunc: func(c *app.RequestContext) string {
			userID := c.GetString("user_id")
			if userID == "" {
				userID = c.ClientIP()
			}
			return userID
		},
	})
}

// APIRateLimitMiddleware creates an API-specific rate limiting middleware
func APIRateLimitMiddleware(requestsPerMinute int) app.HandlerFunc {
	return mw.RateLimitMiddleware(mw.RateLimitConfig{
		Rate:  rate.Limit(requestsPerMinute / 60),
		Burst: requestsPerMinute,
		KeyFunc: func(c *app.RequestContext) string {
			apiKey := string(c.GetHeader("X-API-Key"))
			if apiKey == "" {
				apiKey = c.ClientIP()
			}
			return apiKey
		},
	})
}

// RateLimitConfig contains rate limit configuration
type RateLimitConfig struct {
	Enabled               bool
	RequestsPerSecond     int
	BurstSize             int
	PerUserLimit          bool
	UserRequestsPerSecond int
	UserBurstSize         int
}

// DefaultRateLimitConfig returns default rate limit configuration
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		Enabled:               true,
		RequestsPerSecond:     100,
		BurstSize:             200,
		PerUserLimit:          true,
		UserRequestsPerSecond: 10,
		UserBurstSize:         20,
	}
}
