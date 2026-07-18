package helpers

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
)

// IDGenerator generates various types of IDs
type IDGenerator struct {
	rand *rand.Rand
}

// NewIDGenerator creates a new ID generator
func NewIDGenerator() *IDGenerator {
	return &IDGenerator{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// GenerateUUID generates a UUID v4
func (g *IDGenerator) GenerateUUID() string {
	return uuid.New().String()
}

// GenerateTransactionID generates a transaction ID (VARCHAR format)
// Format: TRX-YYYYMMDD-XXXXXX (e.g., TRX-20241029-A1B2C3)
func (g *IDGenerator) GenerateTransactionID() string {
	now := time.Now()
	dateStr := now.Format("20060102")

	// Generate 6 character random alphanumeric suffix
	suffix := g.randomAlphanumeric(6)

	return fmt.Sprintf("TRX-%s-%s", dateStr, suffix)
}

// GenerateFeaturePropertyID generates a feature property ID (VARCHAR format with prefix/postfix)
// Example: FP-12345-67890
func (g *IDGenerator) GenerateFeaturePropertyID(prefix string, postfix uint64) string {
	return fmt.Sprintf("%s-%d", prefix, postfix)
}

// GenerateOrderID generates an order ID
func (g *IDGenerator) GenerateOrderID() string {
	now := time.Now()
	timestamp := now.Unix()
	random := g.rand.Intn(9999)
	return fmt.Sprintf("ORD-%d-%04d", timestamp, random)
}

// GenerateCode generates a random code (for user codes, etc.)
func (g *IDGenerator) GenerateCode(length int) string {
	return g.randomAlphanumeric(length)
}

// GenerateNumericCode generates a numeric code (for OTP, etc.)
func (g *IDGenerator) GenerateNumericCode(length int) string {
	code := ""
	for i := 0; i < length; i++ {
		code += fmt.Sprintf("%d", g.rand.Intn(10))
	}
	return code
}

// randomAlphanumeric generates a random alphanumeric string
func (g *IDGenerator) randomAlphanumeric(length int) string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = chars[g.rand.Intn(len(chars))]
	}
	return string(result)
}

// ParseFeaturePropertyID parses a feature property ID into prefix and postfix
func ParseFeaturePropertyID(id string) (prefix string, postfix uint64, err error) {
	var p uint64
	_, err = fmt.Sscanf(id, "%s-%d", &prefix, &p)
	if err != nil {
		return "", 0, fmt.Errorf("invalid feature property ID format: %s", id)
	}
	return prefix, p, nil
}
