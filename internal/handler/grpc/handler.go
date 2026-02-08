package grpc

import (
	"context"
	"net"

	"github.com/TomasB/geofence/internal/data"
	geofencev1 "github.com/TomasB/geofence/pkg/geofence/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Handler implements the gRPC GeofenceService.
type Handler struct {
	geofencev1.UnimplementedGeofenceServiceServer
	lookup data.CountryLookup
}

// NewHandler creates a new gRPC handler with the given CountryLookup.
func NewHandler(lookup data.CountryLookup) *Handler {
	return &Handler{lookup: lookup}
}

// Check validates whether an IP is allowed for the given country list.
func (h *Handler) Check(_ context.Context, req *geofencev1.CheckRequest) (*geofencev1.CheckResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if req.Ip == "" {
		return nil, status.Error(codes.InvalidArgument, "ip is required")
	}
	if len(req.AllowedCountries) == 0 {
		return nil, status.Error(codes.InvalidArgument, "allowed_countries is required")
	}

	ip := net.ParseIP(req.Ip)
	if ip == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid IP address")
	}

	country, err := h.lookup.LookupCountry(ip)
	if err != nil {
		return nil, status.Error(codes.Internal, "lookup failed")
	}
	if country == "" {
		return nil, status.Error(codes.Internal, "lookup returned empty country")
	}

	allowed := false
	for _, ac := range req.AllowedCountries {
		if ac == country {
			allowed = true
			break
		}
	}

	return &geofencev1.CheckResponse{
		Allowed: allowed,
		Country: country,
		Error:   "",
	}, nil
}
