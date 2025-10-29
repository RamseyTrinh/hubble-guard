package client

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	prommodel "github.com/prometheus/common/model"
)

// PrometheusClient wraps Prometheus API client
type PrometheusClient struct {
	client v1.API
	url    string
}

// NewPrometheusClient creates a new Prometheus client
func NewPrometheusClient(url string) (*PrometheusClient, error) {
	promClient, err := api.NewClient(api.Config{
		Address: url,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Prometheus client: %v", err)
	}

	v1API := v1.NewAPI(promClient)
	return &PrometheusClient{
		client: v1API,
		url:    url,
	}, nil
}

// Query executes a Prometheus query
func (p *PrometheusClient) Query(ctx context.Context, query string, timeout time.Duration) (prommodel.Value, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result, _, err := p.client.Query(ctx, query, time.Now())
	return result, err
}

// QueryRange executes a Prometheus range query
func (p *PrometheusClient) QueryRange(ctx context.Context, query string, r v1.Range, timeout time.Duration) (prommodel.Value, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result, _, err := p.client.QueryRange(ctx, query, r)
	return result, err
}

// GetClient returns the underlying Prometheus API client
func (p *PrometheusClient) GetClient() v1.API {
	return p.client
}
