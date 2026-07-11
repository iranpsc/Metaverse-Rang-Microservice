package testutil

// Hand-written handler/service mocks live next to tests under internal/handler/*_test.go
// and internal/service/*_test.go (they implement handler port interfaces or local mock repos).
// This package provides shared infra only (MySQL DSN, bufconn); see doc.go.
