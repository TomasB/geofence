package data

import (
	"net"
	"os"
	"testing"
)

const testMMDBPath = "../../testdata/GeoLite2-Country-Test.mmdb"

func skipIfNoMMDB(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(testMMDBPath); os.IsNotExist(err) {
		t.Skip("test MMDB file not found; download it with: curl -L -o testdata/GeoLite2-Country-Test.mmdb https://github.com/maxmind/MaxMind-DB/raw/main/test-data/GeoLite2-Country-Test.mmdb")
	}
}

func TestNewMmdbReader_Success(t *testing.T) {
	skipIfNoMMDB(t)

	reader, err := NewMmdbReader(testMMDBPath)
	if err != nil {
		t.Fatalf("failed to create reader: %v", err)
	}
	defer reader.Close()
}

func TestNewMmdbReader_InvalidPath(t *testing.T) {
	_, err := NewMmdbReader("/nonexistent/path.mmdb")
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

func TestMmdbReader_LookupCountry(t *testing.T) {
	skipIfNoMMDB(t)

	reader, err := NewMmdbReader(testMMDBPath)
	if err != nil {
		t.Fatalf("failed to create reader: %v", err)
	}
	defer reader.Close()

	tests := []struct {
		name    string
		ip      string
		want    string
		wantErr bool
	}{
		{
			name: "UK IP",
			ip:   "2.125.160.216",
			want: "GB",
		},
		{
			name: "US IP",
			ip:   "216.160.83.56",
			want: "US",
		},
		{
			name: "IPv6 JP",
			ip:   "2001:218::",
			want: "JP",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP: %s", tt.ip)
			}

			country, err := reader.LookupCountry(ip)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if country != tt.want {
				t.Errorf("expected country %s, got %s", tt.want, country)
			}
		})
	}
}

func TestMmdbReader_Close(t *testing.T) {
	skipIfNoMMDB(t)

	reader, err := NewMmdbReader(testMMDBPath)
	if err != nil {
		t.Fatalf("failed to create reader: %v", err)
	}

	if err := reader.Close(); err != nil {
		t.Fatalf("failed to close reader: %v", err)
	}
}
