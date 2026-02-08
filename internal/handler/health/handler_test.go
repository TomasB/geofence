package health

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHealth(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create handler
	handler := NewHandler()

	// Create test router
	router := gin.New()
	router.GET("/health", handler.Health)

	// Create request
	req, err := http.NewRequest("GET", "/health", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Create response recorder
	w := httptest.NewRecorder()

	// Perform request
	router.ServeHTTP(w, req)

	// Check status code
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Check response body contains expected status
	expectedBody := `{"status":"ok"}`
	if w.Body.String() != expectedBody {
		t.Errorf("Expected body %s, got %s", expectedBody, w.Body.String())
	}
}

func TestReady(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create handler
	handler := NewHandler()

	// Create test router
	router := gin.New()
	router.GET("/ready", handler.Ready)

	// Create request
	req, err := http.NewRequest("GET", "/ready", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Create response recorder
	w := httptest.NewRecorder()

	// Perform request
	router.ServeHTTP(w, req)

	// Check status code
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Check response body contains expected status
	expectedBody := `{"status":"ready"}`
	if w.Body.String() != expectedBody {
		t.Errorf("Expected body %s, got %s", expectedBody, w.Body.String())
	}
}
