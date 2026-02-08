package data

import (
	"fmt"
	"net"

	"github.com/oschwald/geoip2-golang"
)

// MmdbReader implements CountryLookup using a MaxMind MMDB file.
type MmdbReader struct {
	db *geoip2.Reader
}

// NewMmdbReader opens the MMDB file at the given path and returns a reader.
func NewMmdbReader(path string) (*MmdbReader, error) {
	db, err := geoip2.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open MMDB file: %w", err)
	}
	return &MmdbReader{db: db}, nil
}

// LookupCountry returns the ISO-3166 country code for the given IP address.
func (r *MmdbReader) LookupCountry(ip net.IP) (string, error) {
	record, err := r.db.Country(ip)
	if err != nil {
		return "", fmt.Errorf("country lookup failed: %w", err)
	}
	return record.Country.IsoCode, nil
}

// Close releases the MMDB reader resources.
func (r *MmdbReader) Close() error {
	return r.db.Close()
}
