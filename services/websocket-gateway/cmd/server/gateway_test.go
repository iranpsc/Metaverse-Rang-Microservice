package main

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

type testValidator struct{}

func (testValidator) ValidateToken(context.Context, string) (uint64, error) { return 7, nil }

func TestRunGatewayServesHealthAndStops(t *testing.T) {
	redis := miniredis.RunT(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- runGateway(ctx, testValidator{}, "redis://"+redis.Addr(), "auth.test", strconv.Itoa(port), []string{"*"})
	}()
	url := "http://127.0.0.1:" + strconv.Itoa(port) + "/health"
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		response, err := http.Get(url)
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == 200 {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	response, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatalf("health status=%d", response.StatusCode)
	}
	response, err = http.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatalf("metrics status=%d", response.StatusCode)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("gateway did not shut down")
	}
}

func TestRunGatewayRejectsBadRedisURL(t *testing.T) {
	err := runGateway(context.Background(), testValidator{}, "://invalid", "auth.test", "0", []string{"*"})
	if err == nil || !strings.Contains(err.Error(), "Redis") {
		t.Fatalf("error=%v", err)
	}
}
