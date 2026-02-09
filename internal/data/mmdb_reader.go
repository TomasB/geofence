package data

import (
	"fmt"
	"log/slog"
	"net"
	"path/filepath"
	"sync/atomic"

	"github.com/fsnotify/fsnotify"
	"github.com/oschwald/geoip2-golang"
)

// MmdbReader implements CountryLookup using a MaxMind MMDB file.
// It watches the underlying file for changes and performs atomic
// hot-reload, so callers never observe downtime.
type MmdbReader struct {
	db   atomic.Pointer[geoip2.Reader]
	path string
	done chan struct{} // signals the watcher goroutine to stop
}

// NewMmdbReader opens the MMDB file at the given path, starts a background
// file watcher that automatically reloads the database when the file changes,
// and returns a reader. Call Close to release resources and stop the watcher.
func NewMmdbReader(path string) (*MmdbReader, error) {
	db, err := geoip2.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open MMDB file: %w", err)
	}

	r := &MmdbReader{
		path: path,
		done: make(chan struct{}),
	}
	r.db.Store(db)

	if err := r.startWatcher(); err != nil {
		// Watcher failure is non-fatal: log and continue with a static reader.
		slog.Warn("mmdb file watcher not started; hot-reload disabled", "path", path, "error", err)
	}

	return r, nil
}

// LookupCountry returns the ISO-3166 country code for the given IP address.
func (r *MmdbReader) LookupCountry(ip net.IP) (string, error) {
	record, err := r.db.Load().Country(ip)
	if err != nil {
		return "", fmt.Errorf("country lookup failed: %w", err)
	}
	return record.Country.IsoCode, nil
}

// Close stops the file watcher and releases the MMDB reader resources.
func (r *MmdbReader) Close() error {
	close(r.done)
	return r.db.Load().Close()
}

// reload opens a new MMDB reader from disk and atomically swaps it in,
// then closes the old reader.
func (r *MmdbReader) reload() error {
	newDB, err := geoip2.Open(r.path)
	if err != nil {
		return fmt.Errorf("failed to open new MMDB file: %w", err)
	}

	oldDB := r.db.Swap(newDB)
	if err := oldDB.Close(); err != nil {
		slog.Warn("failed to close old MMDB reader", "error", err)
	}

	slog.Info("mmdb database reloaded", "path", r.path)
	return nil
}

// startWatcher sets up an fsnotify watcher on the parent directory of the MMDB
// file and spawns a goroutine that reloads the database when the file is
// written or created. Watching the directory (not the file) correctly handles
// both in-place writes and atomic rename-into-place strategies used by tools
// like geoipupdate and Kubernetes volume mounts.
//
// NOTE: On macOS with Docker Desktop, host-side file changes on bind mounts are
// proxied through gRPC-FUSE / VirtioFS and do NOT reliably generate inotify
// events inside the container. This means the watcher will not fire when you
// edit files on the host. It works correctly on native Linux (production).
// A polling fallback (startPoller) covers this gap.
// To simulate file changes in development on macOS,
// use docker cp to copy the updated MMDB file into the container, which triggers events correctly:
//
//	docker compose cp ./testdata/GeoLite2-Country-Test.mmdb geofence:/data/GeoLite2-Country-Test.mmdb
//
// You will need to change the volume mount in docker-compose.yaml to a read-write mount for this to work.
func (r *MmdbReader) startWatcher() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}

	dir := filepath.Dir(r.path)
	if err := watcher.Add(dir); err != nil {
		watcher.Close()
		return fmt.Errorf("failed to watch mmdb directory: %w", err)
	}

	base := filepath.Base(r.path)
	slog.Info("mmdb file watcher started", "path", r.path, "watching_dir", dir)

	go func() {
		defer watcher.Close()
		for {
			select {
			case <-r.done:
				return
			case event, ok := <-watcher.Events:

				slog.Info("mmdb file change detected", "event", event.Op.String(), "path", event.Name)

				if !ok {
					slog.Error("mmdb file watcher event channel closed")
					return
				}
				// Only react to events on our specific file.
				if filepath.Base(event.Name) != base {
					continue
				}
				// Reload on write or create (covers both in-place updates
				// and atomic rename-into-place strategies).
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
					slog.Info("mmdb file change detected", "event", event.Op.String(), "path", event.Name)
					if err := r.reload(); err != nil {
						slog.Error("mmdb hot-reload failed", "error", err)
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				slog.Error("mmdb file watcher error", "error", err)
			}
		}
	}()

	return nil
}
