package middleware

import (
	mw "github.com/oekit/middleware"
	"github.com/cloudwego/hertz/pkg/app"
	"golang.org/x/time/rate"
)

func RateLimitMiddleware(rps int, burst int) app.HandlerFunc {
	return mw.RateLimitMiddleware(mw.RateLimitConfig{
		Rate:  rate.Limit(rps),
		Burst: burst,
	})
}
