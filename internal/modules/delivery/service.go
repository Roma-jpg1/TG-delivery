package delivery

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrAddressNotFound = errors.New("address not found")
	ErrOutOfZone       = errors.New("address outside delivery zone")
	ErrBelowMinOrder   = errors.New("cart total is below minimum order amount")
)

type QuoteInput struct {
	UserID       uuid.UUID
	BranchID     uuid.UUID
	AddressID    uuid.UUID
	CartSubtotal int
}

type Quote struct {
	DeliveryFee    int     `json:"delivery_fee"`
	MinOrderAmount int     `json:"min_order_amount"`
	FreeFrom       int     `json:"free_delivery_from"`
	DistanceMeters int     `json:"distance_meters"`
	WithinZone     bool    `json:"within_zone"`
	BranchRadius   int     `json:"branch_radius_meters"`
	AddressLat     float64 `json:"address_latitude"`
	AddressLon     float64 `json:"address_longitude"`
}

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

func (s *Service) Quote(ctx context.Context, in QuoteInput) (Quote, error) {
	var (
		branchLat float64
		branchLon float64
		radius    int
		minOrder  int
		baseFee   int
		freeFrom  int
	)

	err := s.db.QueryRow(ctx, `
		SELECT
			COALESCE(b.latitude::float8, 0),
			COALESCE(b.longitude::float8, 0),
			COALESCE(b.delivery_radius_meters, 0),
			COALESCE(b.min_order_amount, 0),
			COALESCE((r.settings->>'delivery_fee')::int, 0),
			COALESCE((r.settings->>'free_delivery_from')::int, 0)
		FROM branches b
		JOIN restaurants r ON r.id = b.restaurant_id
		WHERE b.id = $1
	`, in.BranchID).Scan(&branchLat, &branchLon, &radius, &minOrder, &baseFee, &freeFrom)
	if err != nil {
		return Quote{}, fmt.Errorf("load branch delivery settings: %w", err)
	}

	var addressLat, addressLon float64
	err = s.db.QueryRow(ctx, `
		SELECT COALESCE(latitude::float8, 0), COALESCE(longitude::float8, 0)
		FROM addresses
		WHERE id = $1 AND user_id = $2
	`, in.AddressID, in.UserID).Scan(&addressLat, &addressLon)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Quote{}, ErrAddressNotFound
		}
		return Quote{}, fmt.Errorf("load delivery address: %w", err)
	}

	distance := haversineDistanceMeters(branchLat, branchLon, addressLat, addressLon)
	if radius > 0 && distance > radius {
		return Quote{}, ErrOutOfZone
	}
	if in.CartSubtotal < minOrder {
		return Quote{}, ErrBelowMinOrder
	}

	fee := baseFee
	if freeFrom > 0 && in.CartSubtotal >= freeFrom {
		fee = 0
	}

	return Quote{
		DeliveryFee:    fee,
		MinOrderAmount: minOrder,
		FreeFrom:       freeFrom,
		DistanceMeters: distance,
		WithinZone:     true,
		BranchRadius:   radius,
		AddressLat:     addressLat,
		AddressLon:     addressLon,
	}, nil
}

func haversineDistanceMeters(lat1, lon1, lat2, lon2 float64) int {
	const earthRadius = 6371000.0

	toRad := func(v float64) float64 {
		return v * math.Pi / 180
	}

	dLat := toRad(lat2 - lat1)
	dLon := toRad(lon2 - lon1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return int(math.Round(earthRadius * c))
}
