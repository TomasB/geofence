package grpc

import (
	"context"
	"fmt"
	"net"
	"testing"

	geofencev1 "github.com/TomasB/geofence/pkg/geofence/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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

func TestCheckAllowed(t *testing.T) {
	h := NewHandler(&mockLookup{country: "US"})

	resp, err := h.Check(context.Background(), &geofencev1.CheckRequest{
		Ip:               "1.2.3.4",
		AllowedCountries: []string{"US", "CA"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Allowed {
		t.Error("expected allowed to be true")
	}
	if resp.Country != "US" {
		t.Errorf("expected country US, got %s", resp.Country)
	}
}

func TestCheckDenied(t *testing.T) {
	h := NewHandler(&mockLookup{country: "RU"})

	resp, err := h.Check(context.Background(), &geofencev1.CheckRequest{
		Ip:               "1.2.3.4",
		AllowedCountries: []string{"US", "CA"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Allowed {
		t.Error("expected allowed to be false")
	}
	if resp.Country != "RU" {
		t.Errorf("expected country RU, got %s", resp.Country)
	}
}

func TestCheckInvalidIP(t *testing.T) {
	h := NewHandler(&mockLookup{country: "US"})

	_, err := h.Check(context.Background(), &geofencev1.CheckRequest{
		Ip:               "not-an-ip",
		AllowedCountries: []string{"US"},
	})
	assertCode(t, err, codes.InvalidArgument)
}

func TestCheckMissingIP(t *testing.T) {
	h := NewHandler(&mockLookup{country: "US"})

	_, err := h.Check(context.Background(), &geofencev1.CheckRequest{
		AllowedCountries: []string{"US"},
	})
	assertCode(t, err, codes.InvalidArgument)
}

func TestCheckMissingAllowedCountries(t *testing.T) {
	h := NewHandler(&mockLookup{country: "US"})

	_, err := h.Check(context.Background(), &geofencev1.CheckRequest{
		Ip: "1.2.3.4",
	})
	assertCode(t, err, codes.InvalidArgument)
}

func TestCheckNilRequest(t *testing.T) {
	h := NewHandler(&mockLookup{country: "US"})

	_, err := h.Check(context.Background(), nil)
	assertCode(t, err, codes.InvalidArgument)
}

func TestCheckLookupError(t *testing.T) {
	h := NewHandler(&mockLookup{err: fmt.Errorf("db failure")})

	_, err := h.Check(context.Background(), &geofencev1.CheckRequest{
		Ip:               "1.2.3.4",
		AllowedCountries: []string{"US"},
	})
	assertCode(t, err, codes.Internal)
}

func TestCheckEmptyCountry(t *testing.T) {
	h := NewHandler(&mockLookup{country: ""})

	_, err := h.Check(context.Background(), &geofencev1.CheckRequest{
		Ip:               "1.2.3.4",
		AllowedCountries: []string{"US"},
	})
	assertCode(t, err, codes.Internal)
}

func assertCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %v", want)
	}
	if status.Code(err) != want {
		t.Fatalf("expected code %v, got %v", want, status.Code(err))
	}
}
