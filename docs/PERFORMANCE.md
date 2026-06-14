# Performance Tuning Guide

## Overview

This guide provides recommendations for tuning the review-service performance in production environments.

## Cache Configuration

### LRU Cache

The service uses an in-memory LRU cache with TTL support for caching frequently accessed data.

**Default Configuration:**
- TTL: 5 minutes
- Max Items: 1000
- Cleanup Interval: 2.5 minutes (TTL/2)

**Tuning Recommendations:**

```go
// For high-traffic environments
cache := cache.NewLRUCache(
    10*time.Minute,  // Longer TTL for stable data
    5000,            // More items for better hit rate
)
```

**Monitoring Cache Performance:**

```go
stats := cache.Stats()
hitRate := stats.HitRate()

// Target: > 80% hit rate
if hitRate < 0.8 {
    // Consider increasing cache size or TTL
}
```

### Cache Hit Rate Optimization

| Hit Rate | Action |
|----------|--------|
| > 90% | Excellent - no changes needed |
| 80-90% | Good - monitor and adjust if needed |
| 70-80% | Consider increasing cache size |
| < 70% | Review caching strategy, increase TTL or size |

## Rate Limiting

### Global Rate Limiting

**Default Configuration:**
- Requests per second: 100
- Burst size: 200

**Tuning for High Traffic:**

```go
// In server.go
middleware.RateLimitMiddleware(500, 1000)  // 500 req/s, burst 1000
```

### Per-User Rate Limiting

**Default Configuration:**
- Requests per second: 10
- Burst size: 20

**Tuning for API Consumers:**

```go
// For API-heavy users
middleware.PerUserRateLimitMiddleware(50, 100)  // 50 req/s, burst 100
```

## Database Optimization

### Connection Pooling

```go
// In postgres/db.go
db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
sqlDB, _ := db.DB()

// Configure pool
sqlDB.SetMaxIdleConns(10)           // Idle connections
sqlDB.SetMaxOpenConns(100)          // Max open connections
sqlDB.SetConnMaxLifetime(time.Hour) // Connection lifetime
```

### Query Optimization

1. **Add Indexes:**
```sql
CREATE INDEX idx_reviews_provider_repo ON reviews(provider, repo);
CREATE INDEX idx_reviews_status ON reviews(status);
CREATE INDEX idx_sessions_review_id ON review_sessions(review_id);
```

2. **Use Pagination:**
```go
// Instead of loading all records
db.Offset(offset).Limit(limit).Find(&reviews)
```

3. **Select Only Needed Fields:**
```go
db.Select("id", "review_id", "status").Find(&reviews)
```

## WebSocket Performance

### Connection Management

```go
// Configure WebSocket buffer sizes
upgrader := websocket.HertzUpgrader{
    ReadBufferSize:  4096,
    WriteBufferSize: 4096,
}
```

### Broadcasting Optimization

1. **Use Room-Based Broadcasting:**
   - Group connections by review ID
   - Only broadcast to relevant rooms

2. **Batch Messages:**
   - Combine multiple updates into single message
   - Reduce WebSocket frame overhead

## Memory Management

### Goroutine Leaks

Monitor goroutine count:
```go
import "runtime"

goroutines := runtime.NumGoroutine()
if goroutines > 1000 {
    // Investigate potential leaks
}
```

### Garbage Collection

Tune GC for low latency:
```go
import "runtime/debug"

debug.SetGCPercent(50)  // More frequent GC, lower memory usage
```

## Monitoring

### Key Metrics to Monitor

1. **Cache Hit Rate:** Target > 80%
2. **Request Latency:** Target < 100ms for reads
3. **Error Rate:** Target < 1%
4. **Goroutine Count:** Monitor for leaks
5. **Memory Usage:** Monitor for growth

### Prometheus Metrics

```go
import "github.com/prometheus/client_golang/prometheus"

var (
    cacheHits = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "cache_hits_total",
        Help: "Total number of cache hits",
    })
    
    requestDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
        Name:    "request_duration_seconds",
        Help:    "Request duration in seconds",
        Buckets: prometheus.DefBuckets,
    })
)
```

## Load Testing

### Using k6

```javascript
import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '30s', target: 100 },  // Ramp up
    { duration: '1m', target: 100 },   // Stay at 100 users
    { duration: '30s', target: 0 },    // Ramp down
  ],
};

export default function () {
  const res = http.get('http://localhost:8080/api/v1/reviews');
  check(res, { 'status was 200': (r) => r.status === 200 });
  sleep(1);
}
```

## Production Checklist

- [ ] Cache TTL and size configured appropriately
- [ ] Rate limiting enabled and tuned
- [ ] Database connection pool configured
- [ ] Indexes added for frequently queried fields
- [ ] WebSocket buffer sizes configured
- [ ] Monitoring and alerting set up
- [ ] Load testing performed
- [ ] Memory profiling done
- [ ] Goroutine leak detection enabled
