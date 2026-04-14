package generator

import (
	"crypto/rand"
	"math/big"
	"time"
)

var idChars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// GenerateID creates a Stripe-like ID with the format: <prefix>_<time><random>
// Example: ch_123abc456def, cus_789xyz012abc
func GenerateID(prefix string) string {
	timePart := generateTimePart()
	randomPart := generateRandomPart(10)

	return prefix + "_" + timePart + randomPart
}

// generateTimePart creates a time-based component for the ID
func generateTimePart() string {
	timestamp := time.Now().Unix()
	timestamp -= 1342389380 // Reference timestamp (stripe-mock uses this)

	encoded := ""
	for i := 0; i < 5; i++ {
		encoded = string(idChars[timestamp%62]) + encoded
		timestamp /= 62
	}

	return encoded
}

// generateRandomPart creates a random string of specified length
func generateRandomPart(length int) string {
	result := make([]byte, length)

	for i := 0; i < length; i++ {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(idChars))))
		result[i] = idChars[n.Int64()]
	}

	return string(result)
}

// RandomString generates a random string of specified length
func RandomString(length int) string {
	return generateRandomPart(length)
}

