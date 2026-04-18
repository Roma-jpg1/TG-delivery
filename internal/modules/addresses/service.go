package addresses

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrAddressNotFound   = errors.New("address not found")
	ErrInvalidCoordinate = errors.New("invalid coordinates")
)

type Address struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Label     string    `json:"label"`
	City      string    `json:"city"`
	Street    string    `json:"street"`
	House     string    `json:"house"`
	Apartment string    `json:"apartment,omitempty"`
	Entrance  string    `json:"entrance,omitempty"`
	Floor     string    `json:"floor,omitempty"`
	Comment   string    `json:"comment,omitempty"`
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	IsDefault bool      `json:"is_default"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UpsertInput struct {
	AddressID  *uuid.UUID
	UserID     uuid.UUID
	Label      string
	City       string
	Street     string
	House      string
	Apartment  string
	Entrance   string
	Floor      string
	Comment    string
	Latitude   float64
	Longitude  float64
	SetDefault bool
}

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

func (s *Service) List(ctx context.Context, userID uuid.UUID) ([]Address, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, user_id, COALESCE(label,''), COALESCE(city,''), COALESCE(street,''), COALESCE(house,''),
		       COALESCE(apartment,''), COALESCE(entrance,''), COALESCE(floor,''), COALESCE(comment,''),
		       COALESCE(latitude::float8, 0), COALESCE(longitude::float8, 0), is_default, created_at, updated_at
		FROM addresses
		WHERE user_id = $1
		ORDER BY is_default DESC, updated_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list addresses: %w", err)
	}
	defer rows.Close()

	items := make([]Address, 0, 8)
	for rows.Next() {
		var a Address
		if err := rows.Scan(
			&a.ID,
			&a.UserID,
			&a.Label,
			&a.City,
			&a.Street,
			&a.House,
			&a.Apartment,
			&a.Entrance,
			&a.Floor,
			&a.Comment,
			&a.Latitude,
			&a.Longitude,
			&a.IsDefault,
			&a.CreatedAt,
			&a.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan address: %w", err)
		}
		items = append(items, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate addresses: %w", err)
	}

	return items, nil
}

func (s *Service) Upsert(ctx context.Context, in UpsertInput) (Address, error) {
	if err := validateCoordinates(in.Latitude, in.Longitude); err != nil {
		return Address{}, err
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Address{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if in.SetDefault {
		if _, err := tx.Exec(ctx, `UPDATE addresses SET is_default = false, updated_at = now() WHERE user_id = $1`, in.UserID); err != nil {
			return Address{}, fmt.Errorf("clear default address: %w", err)
		}
	}

	var id uuid.UUID
	if in.AddressID == nil || *in.AddressID == uuid.Nil {
		err = tx.QueryRow(ctx, `
			INSERT INTO addresses (
				user_id, label, city, street, house, apartment, entrance, floor, comment,
				latitude, longitude, is_default, created_at, updated_at
			)
			VALUES (
				$1, NULLIF($2,''), NULLIF($3,''), NULLIF($4,''), NULLIF($5,''), NULLIF($6,''), NULLIF($7,''), NULLIF($8,''), NULLIF($9,''),
				$10, $11, $12, now(), now()
			)
			RETURNING id
		`, in.UserID, strings.TrimSpace(in.Label), strings.TrimSpace(in.City), strings.TrimSpace(in.Street), strings.TrimSpace(in.House),
			strings.TrimSpace(in.Apartment), strings.TrimSpace(in.Entrance), strings.TrimSpace(in.Floor), strings.TrimSpace(in.Comment),
			in.Latitude, in.Longitude, in.SetDefault).Scan(&id)
		if err != nil {
			return Address{}, fmt.Errorf("insert address: %w", err)
		}
	} else {
		id = *in.AddressID
		cmd, err := tx.Exec(ctx, `
			UPDATE addresses
			SET label = NULLIF($3,''),
			    city = NULLIF($4,''),
			    street = NULLIF($5,''),
			    house = NULLIF($6,''),
			    apartment = NULLIF($7,''),
			    entrance = NULLIF($8,''),
			    floor = NULLIF($9,''),
			    comment = NULLIF($10,''),
			    latitude = $11,
			    longitude = $12,
			    is_default = $13,
			    updated_at = now()
			WHERE id = $1 AND user_id = $2
		`, id, in.UserID, strings.TrimSpace(in.Label), strings.TrimSpace(in.City), strings.TrimSpace(in.Street), strings.TrimSpace(in.House),
			strings.TrimSpace(in.Apartment), strings.TrimSpace(in.Entrance), strings.TrimSpace(in.Floor), strings.TrimSpace(in.Comment),
			in.Latitude, in.Longitude, in.SetDefault)
		if err != nil {
			return Address{}, fmt.Errorf("update address: %w", err)
		}
		if cmd.RowsAffected() == 0 {
			return Address{}, ErrAddressNotFound
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return Address{}, fmt.Errorf("commit transaction: %w", err)
	}

	items, err := s.List(ctx, in.UserID)
	if err != nil {
		return Address{}, err
	}
	for _, item := range items {
		if item.ID == id {
			return item, nil
		}
	}

	return Address{}, ErrAddressNotFound
}

func (s *Service) Delete(ctx context.Context, userID, addressID uuid.UUID) error {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var wasDefault bool
	err = tx.QueryRow(ctx, `SELECT is_default FROM addresses WHERE id = $1 AND user_id = $2`, addressID, userID).Scan(&wasDefault)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrAddressNotFound
		}
		return fmt.Errorf("load address for delete: %w", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM addresses WHERE id = $1 AND user_id = $2`, addressID, userID); err != nil {
		return fmt.Errorf("delete address: %w", err)
	}

	if wasDefault {
		_, err := tx.Exec(ctx, `
			UPDATE addresses
			SET is_default = true, updated_at = now()
			WHERE id = (
				SELECT id FROM addresses WHERE user_id = $1 ORDER BY updated_at DESC LIMIT 1
			)
		`, userID)
		if err != nil {
			return fmt.Errorf("set next default address: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delete address: %w", err)
	}
	return nil
}

func validateCoordinates(lat, lon float64) error {
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return ErrInvalidCoordinate
	}
	if lat == 0 && lon == 0 {
		return ErrInvalidCoordinate
	}
	return nil
}
