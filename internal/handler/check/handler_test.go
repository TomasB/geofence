package check

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// mockLookup implements data.CountryLookup for testing.
type mockLookup struct {
	country string
	err     error
}

func (m *mockLookup) LookupCountry(_ net.IP) (string, error) {
	return m.country, m.err
}

func (m *mockLookup) Close() error {
	return nil
}

func setupRouter(lookup *mockLookup) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandler(lookup)
	r.POST("/api/v1/check", h.Check)
	return r
}

func TestCheck_AllowedCountry(t *testing.T) {
	router := setupRouter(&mockLookup{country: "US"})

	body, _ := json.Marshal(CheckRequest{
		IP:               "1.2.3.4",
		AllowedCountries: []string{"US", "CA"},
	})

	req, _ := http.NewRequest("POST", "/api/v1/check", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp CheckResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Allowed {
		t.Error("expected allowed to be true")
	}
	if resp.Country != "US" {
		t.Errorf("expected country US, got %s", resp.Country)
	}
	if resp.Error != "" {
		t.Errorf("expected empty error, got %s", resp.Error)
	}
}

func TestCheck_DeniedCountry(t *testing.T) {
	router := setupRouter(&mockLookup{country: "RU"})

	body, _ := json.Marshal(CheckRequest{
		IP:               "1.2.3.4",
		AllowedCountries: []string{"US", "CA"},
	})

	req, _ := http.NewRequest("POST", "/api/v1/check", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp CheckResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Allowed {
		t.Error("expected allowed to be false")
	}
	if resp.Country != "RU" {
		t.Errorf("expected country RU, got %s", resp.Country)
	}
}

func TestCheck_InvalidIP(t *testing.T) {
	router := setupRouter(&mockLookup{country: "US"})

	body, _ := json.Marshal(map[string]interface{}{
		"ip":                "not-an-ip",
		"allowed_countries": []string{"US"},
	})

	req, _ := http.NewRequest("POST", "/api/v1/check", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}

	var resp CheckResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Error != "invalid IP address" {
		t.Errorf("expected 'invalid IP address' error, got %q", resp.Error)
	}
}

func TestCheck_MissingIP(t *testing.T) {
	router := setupRouter(&mockLookup{country: "US"})

	body, _ := json.Marshal(map[string]interface{}{
		"allowed_countries": []string{"US"},
	})

	req, _ := http.NewRequest("POST", "/api/v1/check", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestCheck_MissingAllowedCountries(t *testing.T) {
	router := setupRouter(&mockLookup{country: "US"})

	body, _ := json.Marshal(map[string]interface{}{
		"ip": "1.2.3.4",
	})

	req, _ := http.NewRequest("POST", "/api/v1/check", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestCheck_EmptyAllowedCountries(t *testing.T) {
	router := setupRouter(&mockLookup{country: "US"})

	body, _ := json.Marshal(map[string]interface{}{
		"ip":                "1.2.3.4",
		"allowed_countries": []string{},
	})

	req, _ := http.NewRequest("POST", "/api/v1/check", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestCheck_LookupError(t *testing.T) {
	router := setupRouter(&mockLookup{err: fmt.Errorf("db failure")})

	body, _ := json.Marshal(CheckRequest{
		IP:               "1.2.3.4",
		AllowedCountries: []string{"US"},
	})

	req, _ := http.NewRequest("POST", "/api/v1/check", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}

	var resp CheckResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Error != "lookup failed" {
		t.Errorf("expected 'lookup failed' error, got %q", resp.Error)
	}
}

func TestCheck_InvalidJSON(t *testing.T) {
	router := setupRouter(&mockLookup{country: "US"})

	req, _ := http.NewRequest("POST", "/api/v1/check", bytes.NewReader([]byte("{bad json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestCheck_IPv6(t *testing.T) {
	router := setupRouter(&mockLookup{country: "DE"})

	body, _ := json.Marshal(CheckRequest{
		IP:               "2001:db8::1",
		AllowedCountries: []string{"DE"},
	})

	req, _ := http.NewRequest("POST", "/api/v1/check", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp CheckResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if !resp.Allowed {
		t.Error("expected allowed to be true for IPv6")
	}
	if resp.Country != "DE" {
		t.Errorf("expected country DE, got %s", resp.Country)
	}
}
