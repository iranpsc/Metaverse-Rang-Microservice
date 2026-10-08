package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
)

func TestUnaryDeadlineInterceptor(t *testing.T) {
	interceptor := unaryDeadlineInterceptor(5*time.Second, 15*time.Second)

	t.Run("send rpc", func(t *testing.T) {
		_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{
			FullMethod: "/notifications.SMSService/SendOTP",
		}, func(ctx context.Context, _ interface{}) (interface{}, error) {
			deadline, ok := ctx.Deadline()
			if !ok {
				t.Fatal("expected deadline")
			}
			remain := time.Until(deadline)
			if remain < 14*time.Second || remain > 15*time.Second {
				t.Fatalf("remain=%s", remain)
			}
			return nil, nil
		})
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("read rpc", func(t *testing.T) {
		_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{
			FullMethod: "/notifications.NotificationService/GetNotifications",
		}, func(ctx context.Context, _ interface{}) (interface{}, error) {
			deadline, ok := ctx.Deadline()
			if !ok {
				t.Fatal("expected deadline")
			}
			remain := time.Until(deadline)
			if remain < 4*time.Second || remain > 5*time.Second {
				t.Fatalf("remain=%s", remain)
			}
			return nil, nil
		})
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("keeps caller deadline", func(t *testing.T) {
		parent, cancel := context.WithTimeout(context.Background(), time.Hour)
		defer cancel()
		parentDeadline, _ := parent.Deadline()

		_, err := interceptor(parent, nil, &grpc.UnaryServerInfo{
			FullMethod: "/notifications.EmailService/SendEmail",
		}, func(ctx context.Context, _ interface{}) (interface{}, error) {
			deadline, ok := ctx.Deadline()
			if !ok || !deadline.Equal(parentDeadline) {
				t.Fatalf("deadline changed: %v", deadline)
			}
			return nil, nil
		})
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestDatabaseDSNIncludesTimeouts(t *testing.T) {
	t.Setenv("DB_USER", "notifications_service")
	t.Setenv("DB_PASSWORD", "secret")
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_DATABASE", "metarang_db")

	dsn := databaseDSN(3306)
	for _, part := range []string{"timeout=5s", "readTimeout=10s", "writeTimeout=10s", "notifications_service"} {
		if !strings.Contains(dsn, part) {
			t.Fatalf("dsn %q missing %s", dsn, part)
		}
	}
}
