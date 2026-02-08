package data

import "net"

// CountryLookup defines the interface for IP-to-country lookups.
type CountryLookup interface {
	// LookupCountry returns the ISO-3166 country code for the given IP address.
	// Returns an error if the lookup fails or the IP cannot be resolved.
	LookupCountry(ip net.IP) (string, error)

	// Close releases any resources held by the lookup implementation.
	Close() error
}
