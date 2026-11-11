package auth

import (
	"time"

	"github.com/google/uuid"
)

// RegisterRequest represents the user registration payload.
type RegisterRequest struct {
	Name     string `json:"name" validate:"required,min=2,max=120"`
	Email    string `json:"email" validate:"required,email,max=255"`
	Password string `json:"password" validate:"required,min=8,max=72"`
}

// LoginRequest represents the user login payload.
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=1,max=72"`
}

// RefreshRequest represents the token refresh payload.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// LogoutRequest optionally provides the refresh token to revoke alongside the access token.
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// UpdateProfileRequest represents a user profile update payload.
type UpdateProfileRequest struct {
	Name string `json:"name" validate:"required,min=2,max=120"`
}

// UserProfile is the safe public representation of a user.
type UserProfile struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TokenPair represents an access + refresh JWT pair.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

// AuthResponse returns user details and issued JWT tokens.
type AuthResponse struct {
	User   UserProfile `json:"user"`
	Tokens TokenPair   `json:"tokens"`
}

