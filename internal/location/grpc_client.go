package location

import (
	"context"

	"github.com/google/uuid"
	"github.com/hugovillarreal/ridesharing/internal/models"
	"github.com/hugovillarreal/ridesharing/pkg/pb"
	"google.golang.org/grpc"
)

// GRPCClient acts as an adapter that implements location.ServiceInterface
// by making gRPC calls to the remote Location Service.
type GRPCClient struct {
	client pb.LocationServiceClient
}

// NewGRPCClient returns a new Location gRPC client adapter.
func NewGRPCClient(conn grpc.ClientConnInterface) *GRPCClient {
	return &GRPCClient{
		client: pb.NewLocationServiceClient(conn),
	}
}

// UpdateLocation sends a gRPC request to update driver coordinates.
func (c *GRPCClient) UpdateLocation(ctx context.Context, userID uuid.UUID, lat, long float64) error {
	_, err := c.client.UpdateLocation(ctx, &pb.UpdateLocationRequest{
		UserId: userID.String(),
		Lat:    lat,
		Long:   long,
	})
	return err
}

// GetNearbyDrivers sends a gRPC request to find drivers within a radius.
func (c *GRPCClient) GetNearbyDrivers(ctx context.Context, lat, long, radiusKm float64) ([]models.NearbyDriver, error) {
	resp, err := c.client.GetNearbyDrivers(ctx, &pb.GetNearbyDriversRequest{
		Lat:      lat,
		Long:     long,
		RadiusKm: radiusKm,
	})
	if err != nil {
		return nil, err
	}

	var drivers []models.NearbyDriver
	for _, d := range resp.Drivers {
		driverID, _ := uuid.Parse(d.DriverId) // ignore parse error for simplicity in adapter
		drivers = append(drivers, models.NearbyDriver{
			DriverID:     driverID,
			Lat:          d.Lat,
			Long:         d.Long,
			DistanceKm:   d.DistanceKm,
			EstimatedETA: int(d.EstimatedEta),
		})
	}
	return drivers, nil
}

// ActiveDriverCount sends a gRPC request to count available drivers for surge pricing.
func (c *GRPCClient) ActiveDriverCount(ctx context.Context, lat, long, radiusKm float64) (int, error) {
	resp, err := c.client.ActiveDriverCount(ctx, &pb.ActiveDriverCountRequest{
		Lat:      lat,
		Long:     long,
		RadiusKm: radiusKm,
	})
	if err != nil {
		return 0, err
	}
	return int(resp.Count), nil
}
