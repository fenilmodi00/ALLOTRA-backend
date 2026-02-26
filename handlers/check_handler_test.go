package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestHashPAN(t *testing.T) {
	pan := "ABCDE1234F"
	expectedHash := sha256.Sum256([]byte(pan))
	expectedHex := hex.EncodeToString(expectedHash[:])

	actualHex := hashPAN(pan)

	if actualHex != expectedHex {
		t.Errorf("Expected hash %s, got %s", expectedHex, actualHex)
	}

	// Test consistency
	if hashPAN(pan) != actualHex {
		t.Errorf("Hash is not consistent")
	}

	// Test different PANs produce different hashes
	if hashPAN("BCDEF2345G") == actualHex {
		t.Errorf("Different PANs produced same hash")
	}
}
