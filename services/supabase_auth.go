package services

import (
	"context"
	"fmt"
	"os"

	supabase "github.com/supabase-community/supabase-go"
)

// SupabaseAuth provides JWT token verification via Supabase GoTrue.
type SupabaseAuth struct {
	client *supabase.Client
}

// AuthUser is a minimal user record returned after token verification.
type AuthUser struct {
	ID    string
	Email string
}

// NewSupabaseAuth creates a new SupabaseAuth using env vars SUPABASE_URL and
// SUPABASE_SERVICE_KEY.  The service-role key is used so the backend can
// call admin-level auth endpoints without a user session.
func NewSupabaseAuth() (*SupabaseAuth, error) {
	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseKey := os.Getenv("SUPABASE_SERVICE_KEY")

	if supabaseURL == "" || supabaseKey == "" {
		return nil, fmt.Errorf("SUPABASE_URL and SUPABASE_SERVICE_KEY are required")
	}

	client, err := supabase.NewClient(supabaseURL, supabaseKey, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Supabase client: %w", err)
	}

	return &SupabaseAuth{client: client}, nil
}

// VerifyToken validates a bearer token by calling GoTrue's GET /user endpoint.
// The token is forwarded as the Authorization header; GoTrue returns the user
// if the JWT is valid and not expired.
func (s *SupabaseAuth) VerifyToken(_ context.Context, token string) (*AuthUser, error) {
	// WithToken returns a copy of the auth client with the bearer token set,
	// then GetUser calls GET /user with that token.
	userResp, err := s.client.Auth.WithToken(token).GetUser()
	if err != nil {
		return nil, fmt.Errorf("token verification failed: %w", err)
	}

	return &AuthUser{
		ID:    userResp.ID.String(),
		Email: userResp.Email,
	}, nil
}
