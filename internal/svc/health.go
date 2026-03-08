package svc

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type serviceStatusName string

const (
	statusUp   serviceStatusName = "up"
	statusDown serviceStatusName = "down"
	statusInit serviceStatusName = "initializing"
)

type serviceStatus struct {
	mu     *sync.Mutex
	status serviceStatusName
}

type healthCheckResponse struct {
	Service serviceStatusName `json:"service"`
	Redis   serviceStatusName `json:"redis"`
}

//nolint:revive
type HealthCheckService struct {
	timeout     time.Duration
	interval    time.Duration
	rdb         *redis.Client
	redisStatus serviceStatus
}

// NewHealthCheckService creates a new handler for upstream service status checks.
func NewHealthCheckService(
	ctx context.Context,
	timeout time.Duration,
	interval time.Duration,
	rdb *redis.Client,
) *HealthCheckService {
	svc := &HealthCheckService{
		timeout:  timeout,
		interval: interval,
		rdb:      rdb,
		redisStatus: serviceStatus{
			mu:     &sync.Mutex{},
			status: statusInit,
		},
	}

	go svc.runHealthCheckLoop(ctx)

	return svc
}

// Handle is a http request handler. It returns the current status of the member ID service
// and its upstream services.
func (__this *HealthCheckService) Handle(w http.ResponseWriter, _ *http.Request) {
	__this.redisStatus.mu.Lock()
	defer __this.redisStatus.mu.Unlock()

	jsonResp := healthCheckResponse{
		Service: statusUp,
		Redis:   __this.redisStatus.status,
	}

	body, err := json.Marshal(jsonResp)
	if err != nil {
		http.Error(w, "failed to marshal status content", http.StatusInternalServerError)
		log.Printf("failed to marshal status content: %v", err)

		return
	}

	_, err = w.Write(body)
	if err != nil {
		http.Error(w, "failed to write response body", http.StatusInternalServerError)
		log.Printf("failed to write response body: %v", err)

		return
	}
}

func (__this *HealthCheckService) updateRedisHealth(ctx context.Context) {
	// status to set
	status := statusUp

	err := __this.rdb.Ping(ctx).Err()
	if err != nil {
		status = statusDown

		log.Printf("unable to reach redis: %v", err)
	}

	// update status field
	__this.redisStatus.mu.Lock()
	defer __this.redisStatus.mu.Unlock()

	__this.redisStatus.status = status
}

func (__this *HealthCheckService) runHealthCheckLoop(ctx context.Context) {
	ticker := time.NewTicker(__this.interval)

out:
	for {
		select {
		case <-ticker.C:
			ctx, cancelCtx := context.WithTimeout(ctx, __this.timeout)
			defer cancelCtx()

			__this.updateRedisHealth(ctx)
		case <-ctx.Done():
			break out
		}
	}
}
