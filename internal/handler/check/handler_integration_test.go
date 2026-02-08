package check

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/TomasB/geofence/internal/data"
	"github.com/gin-gonic/gin"
)

const testMMDBPath = "../../../testdata/GeoLite2-Country-Test.mmdb"

func skipIfNoMMDB(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(testMMDBPath); os.IsNotExist(err) {
		t.Skip("test MMDB file not found; download it first")
	}
}

func setupIntegrationRouter(t *testing.T) *gin.Engine {
	t.Helper()
	skipIfNoMMDB(t)

	gin.SetMode(gin.TestMode)
	reader, err := data.NewMmdbReader(testMMDBPath)
	if err != nil {
		t.Fatalf("failed to open MMDB: %v", err)
	}
	t.Cleanup(func() { reader.Close() })

	r := gin.New()
	h := NewHandler(reader)
	r.POST("/api/v1/check", h.Check)
	return r
}

func TestIntegration_CheckAllowedGB(t *testing.T) {
	router := setupIntegrationRouter(t)

	body, _ := json.Marshal(CheckRequest{
		IP:               "2.125.160.216",
		AllowedCountries: []string{"GB", "DE"},
	})

	req, _ := http.NewRequest("POST", "/api/v1/check", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp CheckResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if !resp.Allowed {
		t.Error("expected allowed=true for GB IP with GB in allowed list")
	}
	if resp.Country != "GB" {
		t.Errorf("expected country GB, got %s", resp.Country)
	}
}

func TestIntegration_CheckDeniedGB(t *testing.T) {
	router := setupIntegrationRouter(t)

	body, _ := json.Marshal(CheckRequest{
		IP:               "2.125.160.216",
		AllowedCountries: []string{"US", "CA"},
	})

	req, _ := http.NewRequest("POST", "/api/v1/check", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp CheckResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Allowed {
		t.Error("expected allowed=false for GB IP with only US,CA allowed")
	}
	if resp.Country != "GB" {
		t.Errorf("expected country GB, got %s", resp.Country)
	}
}

func TestIntegration_CheckUS(t *testing.T) {
	router := setupIntegrationRouter(t)

	body, _ := json.Marshal(CheckRequest{
		IP:               "216.160.83.56",
		AllowedCountries: []string{"US"},
	})

	req, _ := http.NewRequest("POST", "/api/v1/check", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp CheckResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if !resp.Allowed {
		t.Error("expected allowed=true for US IP")
	}
	if resp.Country != "US" {
		t.Errorf("expected country US, got %s", resp.Country)
	}
}
