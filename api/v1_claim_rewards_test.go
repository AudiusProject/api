package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"bridgerton.audius.co/config"
	"github.com/AudiusProject/audiusd/pkg/rewards"
	"github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/assert"
)

func TestFetchAttestations(t *testing.T) {
	// Track which URLs are called
	urlCallCount := make(map[string]int)

	// Create mock HTTP servers for AAO and validators
	aaoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		urlCallCount[r.Host]++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"result": "aabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccdd"}`))
	}))
	defer aaoServer.Close()

	// Create separate validator servers
	validator1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		urlCallCount[r.Host]++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"attestation": "aabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccdd", "owner": "0x1111111111111111111111111111111111111111"}`))
	}))
	defer validator1.Close()

	validator2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		urlCallCount[r.Host]++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		// Duplicate owner should not be selected
		w.Write([]byte(`{"attestation": "aabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccdd", "owner": "0x1111111111111111111111111111111111111111"}`))
	}))
	defer validator2.Close()

	validator3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		urlCallCount[r.Host]++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"attestation": "aabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccdd", "owner": "0x3333333333333333333333333333333333333333"}`))
	}))
	defer validator3.Close()

	validator4 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		urlCallCount[r.Host]++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		w.Write([]byte(`{"error": "unhappy validator"}`))
	}))
	defer validator4.Close()

	validator5 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		urlCallCount[r.Host]++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"attestation": "aabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccdd", "owner": "0x5555555555555555555555555555555555555555"}`))
	}))
	defer validator5.Close()

	// Test data
	rewardClaim := RewardClaim{
		RewardClaim: rewards.RewardClaim{
			RewardID:                  "test-reward",
			Amount:                    1000,
			Specifier:                 "test-spec",
			RecipientEthAddress:       "0x1234567890123456789012345678901234567890",
			AntiAbuseOracleEthAddress: "",
		},
		Handle:   "testuser",
		UserBank: solana.MustPublicKeyFromBase58("11111111111111111111111111111112"),
	}

	allValidators := []config.Node{
		{Endpoint: validator1.URL, OwnerWallet: "0x1111111111111111111111111111111111111111"},
		{Endpoint: validator2.URL, OwnerWallet: "0x1111111111111111111111111111111111111111"},
		{Endpoint: validator3.URL, OwnerWallet: "0x3333333333333333333333333333333333333333"},
		{Endpoint: validator4.URL, OwnerWallet: "0x4444444444444444444444444444444444444444"},
		{Endpoint: validator5.URL, OwnerWallet: "0x5555555555555555555555555555555555555555"},
	}

	antiAbuseOracle := config.Node{
		Endpoint:            aaoServer.URL,
		DelegateOwnerWallet: "0xAAA0000000000000000000000000000000000000",
	}

	// Set up the AAO map for the test
	antiAbuseOracleMap[aaoServer.URL] = "0xAAA0000000000000000000000000000000000000"

	// Call fetchAttestations
	attestations, err := fetchAttestations(
		context.Background(),
		rewardClaim,
		allValidators,
		[]string{}, // no excluded operators
		antiAbuseOracle,
		"test-signature",
		false, // need AAO attestation
		3,     // minVotes
	)

	// Verify results
	assert.NoError(t, err)
	assert.Len(t, attestations, 4) // 1 AAO + 3 validators

	// First should be AAO
	assert.Equal(t, "0xAAa0000000000000000000000000000000000000", attestations[0].EthAddress.Hex())

	// Rest should be unique validator owners
	addresses := make(map[string]bool)
	for _, att := range attestations {
		addr := att.EthAddress.Hex()
		assert.False(t, addresses[addr], "Duplicate address: %s", addr)
		addresses[addr] = true
	}

	// Assert that validator4 is not present
	assert.False(t, addresses["0x4444444444444444444444444444444444444444"], "validator4 should not be present in attestations")

	// Verify no URL was called more than once
	for url, count := range urlCallCount {
		assert.LessOrEqual(t, count, 1, "URL %s should never be called more than once, but was called %d times", url, count)
	}
}
