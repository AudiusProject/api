package jobs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIsValidBpm(t *testing.T) {
	cases := []struct {
		bpm  float64
		want bool
	}{
		{0, false},
		{-1, false},
		{0.5, true},
		{128, true},
		{998.9, true},
		{999, false},
		{1000, false},
	}
	for _, c := range cases {
		if got := isValidBpm(c.bpm); got != c.want {
			t.Errorf("isValidBpm(%v) = %v, want %v", c.bpm, got, c.want)
		}
	}
}

func TestIsValidMusicalKey(t *testing.T) {
	valid := []string{"A major", "D flat minor", "Silence", "G flat major"}
	for _, k := range valid {
		if !isValidMusicalKey(k) {
			t.Errorf("isValidMusicalKey(%q) = false, want true", k)
		}
	}
	// Sharps are not part of the enum (flats only).
	invalid := []string{"", "C# minor", "H sharp", "D# major", "a major"}
	for _, k := range invalid {
		if isValidMusicalKey(k) {
			t.Errorf("isValidMusicalKey(%q) = true, want false", k)
		}
	}
}

func TestAnalysisResultCaseFallback(t *testing.T) {
	// Modern mediorum serves lowercase keys.
	lower := analysisResult{BPM: 120, Key: "A major"}
	if lower.bpm() != 120 || lower.key() != "A major" {
		t.Errorf("lowercase: bpm=%v key=%q", lower.bpm(), lower.key())
	}
	// Legacy python content node served capitalized keys.
	upper := analysisResult{BPMCap: 90, KeyCap: "C minor"}
	if upper.bpm() != 90 || upper.key() != "C minor" {
		t.Errorf("uppercase: bpm=%v key=%q", upper.bpm(), upper.key())
	}
}

func TestFetchAnalysisModern(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"audio_analysis_results":{"bpm":128.5,"key":"D flat minor"},"audio_analysis_error_count":1}`))
	}))
	defer srv.Close()

	j := &RepairAudioAnalysesJob{httpClient: &http.Client{Timeout: 2 * time.Second}}
	result, errorCount, ok := j.fetchAnalysis(context.Background(), srv.URL+"/uploads/up-1", false)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if result == nil || result.bpm() != 128.5 || result.key() != "D flat minor" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if errorCount != 1 {
		t.Errorf("errorCount = %d, want 1", errorCount)
	}
}

func TestFetchAnalysisLegacy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Legacy payload uses "results" + "error_count" and capitalized fields.
		w.Write([]byte(`{"results":{"BPM":95,"Key":"C minor"},"error_count":2}`))
	}))
	defer srv.Close()

	j := &RepairAudioAnalysesJob{httpClient: &http.Client{Timeout: 2 * time.Second}}
	result, errorCount, ok := j.fetchAnalysis(context.Background(), srv.URL+"/tracks/legacy/Qm123/analysis", true)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if result == nil || result.bpm() != 95 || result.key() != "C minor" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if errorCount != 2 {
		t.Errorf("errorCount = %d, want 2", errorCount)
	}
}

func TestFetchAnalysisNon2xxIsNotOk(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	j := &RepairAudioAnalysesJob{httpClient: &http.Client{Timeout: 2 * time.Second}}
	_, _, ok := j.fetchAnalysis(context.Background(), srv.URL+"/uploads/missing", false)
	if ok {
		t.Error("expected ok=false for non-2xx response")
	}
}

func TestFetchAnalysisEmptyResultsOkButNil(t *testing.T) {
	// Node responded but has no analysis yet: ok=true, result=nil.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"audio_analysis_error_count":0}`))
	}))
	defer srv.Close()

	j := &RepairAudioAnalysesJob{httpClient: &http.Client{Timeout: 2 * time.Second}}
	result, _, ok := j.fetchAnalysis(context.Background(), srv.URL+"/uploads/up-2", false)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
}
