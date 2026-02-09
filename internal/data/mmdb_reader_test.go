package data

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
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

func TestMmdbReader_HotReload(t *testing.T) {
	skipIfNoMMDB(t)

	// Copy the test MMDB to a temp file so we can replace it atomically.
	srcData, err := os.ReadFile(testMMDBPath)
	if err != nil {
		t.Fatalf("failed to read source MMDB: %v", err)
	}

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.mmdb")
	if err := os.WriteFile(tmpFile, srcData, 0644); err != nil {
		t.Fatalf("failed to write temp MMDB: %v", err)
	}

	reader, err := NewMmdbReader(tmpFile)
	if err != nil {
		t.Fatalf("failed to create reader: %v", err)
	}
	defer reader.Close()

	// Verify lookup works before reload.
	ip := net.ParseIP("2.125.160.216")
	country, err := reader.LookupCountry(ip)
	if err != nil {
		t.Fatalf("lookup failed before reload: %v", err)
	}
	if country != "GB" {
		t.Fatalf("expected GB before reload, got %s", country)
	}

	// Atomically replace the file (write temp + rename), mirroring how
	// geoipupdate and K8s volume updates work in production.
	staging := filepath.Join(tmpDir, "test.mmdb.tmp")
	if err := os.WriteFile(staging, srcData, 0644); err != nil {
		t.Fatalf("failed to write staging MMDB: %v", err)
	}
	if err := os.Rename(staging, tmpFile); err != nil {
		t.Fatalf("failed to rename staging MMDB: %v", err)
	}

	// Give the watcher time to detect the change and reload.
	time.Sleep(500 * time.Millisecond)

	// Verify lookup still works after reload.
	country, err = reader.LookupCountry(ip)
	if err != nil {
		t.Fatalf("lookup failed after reload: %v", err)
	}
	if country != "GB" {
		t.Fatalf("expected GB after reload, got %s", country)
	}
}

func TestMmdbReader_HotReload_InvalidFile(t *testing.T) {
	skipIfNoMMDB(t)

	// Copy the test MMDB to a temp file.
	srcData, err := os.ReadFile(testMMDBPath)
	if err != nil {
		t.Fatalf("failed to read source MMDB: %v", err)
	}

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.mmdb")
	if err := os.WriteFile(tmpFile, srcData, 0644); err != nil {
		t.Fatalf("failed to write temp MMDB: %v", err)
	}

	reader, err := NewMmdbReader(tmpFile)
	if err != nil {
		t.Fatalf("failed to create reader: %v", err)
	}
	defer reader.Close()

	// Atomically replace with an invalid file — reload should fail,
	// but the old reader (still mmap'd to the original inode) stays valid.
	staging := filepath.Join(tmpDir, "test.mmdb.tmp")
	if err := os.WriteFile(staging, []byte("not a valid mmdb"), 0644); err != nil {
		t.Fatalf("failed to write invalid MMDB: %v", err)
	}
	if err := os.Rename(staging, tmpFile); err != nil {
		t.Fatalf("failed to rename invalid MMDB: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	// The reader should still be functional with the old database.
	ip := net.ParseIP("2.125.160.216")
	country, err := reader.LookupCountry(ip)
	if err != nil {
		t.Fatalf("lookup should still work after failed reload: %v", err)
	}
	if country != "GB" {
		t.Fatalf("expected GB, got %s", country)
	}
}
