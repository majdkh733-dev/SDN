package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/majdkh733-dev/SDN/internal/apierr"
	"github.com/majdkh733-dev/SDN/internal/authn"
)

type User struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

type Membership struct {
	OrganizationID   string `json:"organization_id"`
	OrganizationName string `json:"organization_name"`
	OrganizationSlug string `json:"organization_slug"`
	Role             string `json:"role"`
}

type Principal struct {
	User         User
	Organization Membership
	Memberships  []Membership
}

func (s *Store) UpsertUser(ctx context.Context, email, displayName, password string) (User, error) {
	var hash *string
	if password != "" {
		raw, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return User{}, err
		}
		value := string(raw)
		hash = &value
	}
	var user User
	err := s.pool.QueryRow(ctx, `
		INSERT INTO users (email, display_name, password_hash, oidc_subject)
		VALUES (lower($1), $2, $3, $4)
		ON CONFLICT ((lower(email))) DO UPDATE
		SET display_name = EXCLUDED.display_name,
		    password_hash = COALESCE(EXCLUDED.password_hash, users.password_hash)
		RETURNING id::text, email, display_name
	`, email, displayName, hash, "local|"+email).Scan(&user.ID, &user.Email, &user.DisplayName)
	if err != nil {
		// The unique index is an expression index, so ON CONFLICT ((lower(email))) needs the constraint inferred.
		return User{}, err
	}
	return user, nil
}

func (s *Store) EnsureUser(ctx context.Context, email, displayName, password string) (User, error) {
	existing, err := s.userByEmail(ctx, email)
	if err == nil {
		if password != "" {
			raw, hashErr := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			if hashErr != nil {
				return User{}, hashErr
			}
			_, err = s.pool.Exec(ctx, `UPDATE users SET password_hash = $2, display_name = $3 WHERE id = $1`, existing.ID, string(raw), displayName)
			if err != nil {
				return User{}, err
			}
			existing.DisplayName = displayName
		}
		return existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) && !isNotFound(err) {
		var ae *apierr.Error
		if !errors.As(err, &ae) {
			return User{}, err
		}
	}
	var hash any
	if password != "" {
		raw, hashErr := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if hashErr != nil {
			return User{}, hashErr
		}
		hash = string(raw)
	}
	var user User
	err = s.pool.QueryRow(ctx, `
		INSERT INTO users (email, display_name, password_hash, oidc_subject)
		VALUES (lower($1), $2, $3, $4)
		RETURNING id::text, email, display_name
	`, email, displayName, hash, "local|"+email).Scan(&user.ID, &user.Email, &user.DisplayName)
	return user, err
}

func isNotFound(err error) bool {
	var ae *apierr.Error
	return errors.As(err, &ae) && ae.Code == "not_found"
}

func (s *Store) userByEmail(ctx context.Context, email string) (User, error) {
	var user User
	var hash *string
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, email, display_name, password_hash
		FROM users WHERE lower(email) = lower($1)
	`, email).Scan(&user.ID, &user.Email, &user.DisplayName, &hash)
	if err != nil {
		return User{}, mapDB(err, "user")
	}
	return user, nil
}

func (s *Store) Authenticate(ctx context.Context, email, password string) (Principal, error) {
	var user User
	var hash *string
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, email, display_name, password_hash
		FROM users WHERE lower(email) = lower($1)
	`, email).Scan(&user.ID, &user.Email, &user.DisplayName, &hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Principal{}, apierr.Unauthorized()
		}
		return Principal{}, err
	}
	if hash == nil || bcrypt.CompareHashAndPassword([]byte(*hash), []byte(password)) != nil {
		return Principal{}, apierr.Unauthorized()
	}
	return s.principal(ctx, user)
}

func (s *Store) principal(ctx context.Context, user User) (Principal, error) {
	var memberships []Membership
	err := s.withUser(ctx, user.ID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT m.organization_id::text, o.name, o.slug, m.role
			FROM memberships m
			JOIN organizations o ON o.id = m.organization_id
			WHERE m.user_id = $1
			ORDER BY o.name
		`, user.ID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item Membership
			if err := rows.Scan(&item.OrganizationID, &item.OrganizationName, &item.OrganizationSlug, &item.Role); err != nil {
				return err
			}
			memberships = append(memberships, item)
		}
		return rows.Err()
	})
	if err != nil {
		return Principal{}, err
	}
	if len(memberships) == 0 {
		return Principal{}, apierr.Forbidden("This account has no organization membership")
	}
	return Principal{User: user, Organization: memberships[0], Memberships: memberships}, nil
}

func (s *Store) PrincipalByID(ctx context.Context, userID, orgID string) (Principal, error) {
	var user User
	err := s.pool.QueryRow(ctx, `SELECT id::text, email, display_name FROM users WHERE id = $1`, userID).Scan(&user.ID, &user.Email, &user.DisplayName)
	if err != nil {
		return Principal{}, mapDB(err, "user")
	}
	principal, err := s.principal(ctx, user)
	if err != nil {
		return Principal{}, err
	}
	for _, item := range principal.Memberships {
		if item.OrganizationID == orgID {
			principal.Organization = item
			return principal, nil
		}
	}
	return Principal{}, apierr.Forbidden("organization membership was not found")
}

func (s *Store) CreateSession(ctx context.Context, principal Principal, ttl time.Duration) (string, error) {
	token, err := authn.NewToken()
	if err != nil {
		return "", err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO sessions (user_id, organization_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)
	`, principal.User.ID, principal.Organization.OrganizationID, authn.Hash(token), time.Now().Add(ttl))
	if err != nil {
		return "", err
	}
	return token, nil
}

func (s *Store) PrincipalFromSession(ctx context.Context, token string) (Principal, error) {
	var user User
	var orgID string
	err := s.pool.QueryRow(ctx, `
		SELECT u.id::text, u.email, u.display_name, s.organization_id::text
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > now()
	`, authn.Hash(token)).Scan(&user.ID, &user.Email, &user.DisplayName, &orgID)
	if err != nil {
		return Principal{}, mapDB(err, "session")
	}
	principal, err := s.principal(ctx, user)
	if err != nil {
		return Principal{}, err
	}
	for _, item := range principal.Memberships {
		if item.OrganizationID == orgID {
			principal.Organization = item
			return principal, nil
		}
	}
	return Principal{}, apierr.Unauthorized()
}

func (s *Store) DeleteSession(ctx context.Context, token string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, authn.Hash(token))
	return err
}

func (s *Store) SwitchOrganization(ctx context.Context, token, orgID string) (Principal, error) {
	principal, err := s.PrincipalFromSession(ctx, token)
	if err != nil {
		return Principal{}, err
	}
	var match *Membership
	for i := range principal.Memberships {
		if principal.Memberships[i].OrganizationID == orgID {
			match = &principal.Memberships[i]
			break
		}
	}
	if match == nil {
		return Principal{}, apierr.Forbidden("You are not a member of that organization")
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE sessions SET organization_id = $2 WHERE token_hash = $1 AND expires_at > now()
	`, authn.Hash(token), orgID)
	if err != nil {
		return Principal{}, err
	}
	if tag.RowsAffected() != 1 {
		return Principal{}, apierr.Unauthorized()
	}
	principal.Organization = *match
	return principal, nil
}

type AuthCode struct {
	UserID         string
	OrganizationID string
	Challenge      string
	RedirectURI    string
	ClientID       string
}

func (s *Store) SaveAuthCode(ctx context.Context, code string, ac AuthCode, ttl time.Duration) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO oidc_auth_codes (code_hash, client_id, user_id, organization_id, code_challenge, redirect_uri, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, authn.Hash(code), ac.ClientID, ac.UserID, ac.OrganizationID, ac.Challenge, ac.RedirectURI, time.Now().Add(ttl))
	return err
}

func (s *Store) ConsumeAuthCode(ctx context.Context, code, clientID, redirectURI string) (AuthCode, error) {
	var ac AuthCode
	var used *time.Time
	var expires time.Time
	err := s.pool.QueryRow(ctx, `
		UPDATE oidc_auth_codes
		SET used_at = now()
		WHERE code_hash = $1 AND used_at IS NULL AND expires_at > now()
		RETURNING client_id, user_id::text, organization_id::text, code_challenge, redirect_uri, used_at, expires_at
	`, authn.Hash(code)).Scan(&ac.ClientID, &ac.UserID, &ac.OrganizationID, &ac.Challenge, &ac.RedirectURI, &used, &expires)
	if err != nil {
		return AuthCode{}, mapDB(err, "authorization code")
	}
	if ac.ClientID != clientID || ac.RedirectURI != redirectURI {
		return AuthCode{}, apierr.Invalid("authorization code does not match this client")
	}
	return ac, nil
}

func (s *Store) AddMembership(ctx context.Context, orgID, userID, role string) error {
	return s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO memberships (organization_id, user_id, role) VALUES ($1, $2, $3)
		`, orgID, userID, role)
		return mapDB(err, "membership")
	})
}

func (s *Store) ListMemberships(ctx context.Context, orgID string) ([]map[string]any, error) {
	var out []map[string]any
	err := s.WithOrg(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT u.id::text, u.email, u.display_name, m.role, m.created_at
			FROM memberships m
			JOIN users u ON u.id = m.user_id
			WHERE m.organization_id = $1
			ORDER BY u.email
		`, orgID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id, email, name, role string
			var created time.Time
			if err := rows.Scan(&id, &email, &name, &role, &created); err != nil {
				return err
			}
			out = append(out, map[string]any{
				"user_id": id, "email": email, "display_name": name, "role": role, "created_at": created,
			})
		}
		return rows.Err()
	})
	if out == nil {
		out = []map[string]any{}
	}
	return out, err
}
