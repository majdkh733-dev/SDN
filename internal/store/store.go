package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/majdkh733-dev/SDN/internal/apierr"
)

type Store struct {
	pool *pgxpool.Pool
}

func Connect(ctx context.Context, url string) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

func (s *Store) Pool() *pgxpool.Pool { return s.pool }

type ctxKey int

const userKey ctxKey = 1

func WithUser(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userKey, userID)
}

func UserFrom(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(userKey).(string)
	return v, ok && v != ""
}

func (s *Store) WithOrg(ctx context.Context, orgID string, fn func(context.Context, pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_org', $1, true)`, orgID); err != nil {
		return err
	}
	if userID, ok := UserFrom(ctx); ok {
		if _, err := tx.Exec(ctx, `SELECT set_config('app.current_user', $1, true)`, userID); err != nil {
			return err
		}
	}
	if err := fn(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) withUser(ctx context.Context, userID string, fn func(context.Context, pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_user', $1, true)`, userID); err != nil {
		return err
	}
	if err := fn(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) WithBypass(ctx context.Context, fn func(context.Context, pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.bypass', 'on', true)`); err != nil {
		return err
	}
	if err := fn(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) NewID(ctx context.Context) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `SELECT gen_random_uuid()::text`).Scan(&id)
	return id, err
}

func mapDB(err error, resource string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return apierr.NotFound(resource)
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return apierr.Conflict("conflict", resource+" already exists")
	}
	return err
}

func mustJSON(v any) []byte {
	raw, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}
	return raw
}

func jsonUnmarshal(raw []byte, dest any) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, dest)
}

type Site struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Location        string    `json:"location"`
	ExecutionDomain string    `json:"execution_domain"`
	Lifecycle       string    `json:"lifecycle"`
	CreatedAt       time.Time `json:"created_at"`
}

type Connector struct {
	ID            string     `json:"id"`
	SiteID        string     `json:"site_id"`
	Name          string     `json:"name"`
	Adapter       string     `json:"adapter"`
	Version       string     `json:"version"`
	Lifecycle     string     `json:"lifecycle"`
	Health        string     `json:"health"`
	ExecutionMode string     `json:"execution_mode"`
	LastHeartbeat *time.Time `json:"last_heartbeat_at"`
	CredentialRef string     `json:"credential_ref"`
	SupportStatus string     `json:"support_status"`
	CreatedAt     time.Time  `json:"created_at"`
}

type Device struct {
	ID                 string     `json:"id"`
	SiteID             string     `json:"site_id"`
	ConnectorID        string     `json:"connector_id,omitempty"`
	Name               string     `json:"name"`
	Model              string     `json:"model"`
	OSVersion          string     `json:"os_version"`
	Role               string     `json:"role"`
	ManagementEndpoint string     `json:"management_endpoint"`
	Lifecycle          string     `json:"lifecycle"`
	Health             string     `json:"health"`
	DataOrigin         string     `json:"data_origin"`
	SupportNote        string     `json:"support_note"`
	ObservedAt         *time.Time `json:"observed_at"`
	CreatedAt          time.Time  `json:"created_at"`
	FailNextWrite      bool       `json:"-"`
	FailNextRollback   bool       `json:"-"`
	UncertainNextWrite bool       `json:"-"`
	UncertainCommitted bool       `json:"-"`
}

type Interface struct {
	ID        string   `json:"id"`
	DeviceID  string   `json:"device_id"`
	Name      string   `json:"name"`
	MAC       string   `json:"mac"`
	AdminUp   bool     `json:"admin_up"`
	OperUp    bool     `json:"oper_up"`
	SpeedMbps int      `json:"speed_mbps"`
	Addresses []string `json:"addresses"`
}

type Capability struct {
	Capability       string    `json:"capability"`
	Supported        bool      `json:"supported"`
	Evidence         string    `json:"evidence"`
	ConnectorVersion string    `json:"connector_version"`
	DiscoveredAt     time.Time `json:"discovered_at"`
}

type Fabric struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Lifecycle   string    `json:"lifecycle"`
	CreatedAt   time.Time `json:"created_at"`
}

type Tenant struct {
	ID              string    `json:"id"`
	FabricID        string    `json:"fabric_id"`
	Name            string    `json:"name"`
	IsolationDomain string    `json:"isolation_domain"`
	Lifecycle       string    `json:"lifecycle"`
	CreatedAt       time.Time `json:"created_at"`
}

type Segment struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Name      string    `json:"name"`
	CIDR      string    `json:"cidr"`
	VLAN      *int      `json:"vlan"`
	VNI       *int      `json:"vni"`
	Lifecycle string    `json:"lifecycle"`
	CreatedAt time.Time `json:"created_at"`
}
