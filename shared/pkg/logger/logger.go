// Package logger provides structured logging helpers for services.
package logger

import (
	"context"
	"os"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// Logger wraps logrus logger
type Logger struct {
	*logrus.Logger
}

// NewLogger creates a new logger instance
func NewLogger(serviceName string) *Logger {
	log := logrus.New()

	// Set JSON formatter
	log.SetFormatter(&logrus.JSONFormatter{
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "timestamp",
			logrus.FieldKeyLevel: "level",
			logrus.FieldKeyMsg:   "message",
		},
	})

	// Set output
	log.SetOutput(os.Stdout)

	// Set log level from environment
	level := os.Getenv("LOG_LEVEL")
	switch level {
	case "debug":
		log.SetLevel(logrus.DebugLevel)
	case "info":
		log.SetLevel(logrus.InfoLevel)
	case "warn":
		log.SetLevel(logrus.WarnLevel)
	case "error":
		log.SetLevel(logrus.ErrorLevel)
	default:
		log.SetLevel(logrus.InfoLevel)
	}

	// Add default fields
	log = log.WithField("service", serviceName).Logger

	return &Logger{Logger: log}
}

// WithRequestID adds request ID to logger
func (l *Logger) WithRequestID(requestID string) *logrus.Entry {
	return l.WithField("request_id", requestID)
}

// WithUserID adds user ID to logger
func (l *Logger) WithUserID(userID uint64) *logrus.Entry {
	return l.WithField("user_id", userID)
}

// UnaryServerInterceptor returns a new unary server interceptor for logging
func UnaryServerInterceptor(logger *Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Log the request
		logger.WithFields(logrus.Fields{
			"method": info.FullMethod,
			"type":   "unary",
		}).Info("gRPC request")

		// Call handler
		resp, err := handler(ctx, req)

		// Log the response
		if err != nil {
			logger.WithFields(logrus.Fields{
				"method": info.FullMethod,
				"error":  err.Error(),
			}).Error("gRPC request failed")
		} else {
			logger.WithField("method", info.FullMethod).Debug("gRPC request completed")
		}

		return resp, err
	}
}

// StreamServerInterceptor returns a new stream server interceptor for logging
func StreamServerInterceptor(logger *Logger) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		// Log the stream start
		logger.WithFields(logrus.Fields{
			"method": info.FullMethod,
			"type":   "stream",
		}).Info("gRPC stream started")

		// Call handler
		err := handler(srv, stream)

		// Log the stream end
		if err != nil {
			logger.WithFields(logrus.Fields{
				"method": info.FullMethod,
				"error":  err.Error(),
			}).Error("gRPC stream failed")
		} else {
			logger.WithField("method", info.FullMethod).Debug("gRPC stream completed")
		}

		return err
	}
}
