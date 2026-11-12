package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/mediadock/backend/internal/cache"
	"github.com/mediadock/backend/internal/config"
	"github.com/mediadock/backend/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrEmailTaken         = errors.New("an account with this email already exists")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInvalidToken       = errors.New("invalid or expired token")
	ErrTokenRevoked       = errors.New("token has been revoked")
)

// TokenClaims extends jwt.RegisteredClaims with token type and user ID.
type TokenClaims struct {
	UserID    string `json:"uid"`
	TokenType string `json:"typ"`
	jwt.RegisteredClaims
}

// Service handles authentication, password hashing, JWT lifecycle, and revocation.
type Service struct {
	users repository.UserRepository
	cache *cache.RedisClient
	cfg   *config.Config
}

// NewService constructs an auth Service.
func NewService(users repository.UserRepository, redisCache *cache.RedisClient, cfg *config.Config) *Service {
	return &Service{
		users: users,
		cache: redisCache,
		cfg:   cfg,
	}
}

// Register creates a new user account with default settings and returns an initial token pair.
func (s *Service) Register(ctx context.Context, req RegisterRequest) (*AuthResponse, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if _, err := s.users.FindByEmail(ctx, email); err == nil {
		return nil, ErrEmailTaken
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := &repository.User{
		ID:           uuid.New(),
		Name:         strings.TrimSpace(req.Name),
		Email:        email,
		PasswordHash: string(hash),
	}
	settings := &repository.UserSettings{
		ID:            uuid.New(),
		Theme:         "system",
		DefaultFormat: "mp4",
	}

	if err := s.users.CreateWithSettings(ctx, user, settings); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") || strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return nil, ErrEmailTaken
		}
		return nil, err
	}

	tokens, err := s.issueTokenPair(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{
		User:   toUserProfile(user),
		Tokens: *tokens,
	}, nil
}

// Login verifies email and password and issues a fresh access/refresh token pair.
func (s *Service) Login(ctx context.Context, req LoginRequest) (*AuthResponse, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	tokens, err := s.issueTokenPair(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{
		User:   toUserProfile(user),
		Tokens: *tokens,
	}, nil
}

// Refresh validates a refresh token, rotates it in Redis, and issues a new token pair.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	claims, err := s.ParseToken(refreshToken, "refresh")
	if err != nil {
		return nil, err
	}

	refreshKey := refreshRedisKey(claims.ID)
	exists, err := s.cache.Exists(ctx, refreshKey)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrTokenRevoked
	}

	// Rotate refresh token by deleting the consumed JTI.
	if err := s.cache.Delete(ctx, refreshKey); err != nil {
		return nil, err
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, ErrInvalidToken
	}

	if _, err := s.users.FindByID(ctx, userID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, err
	}

	return s.issueTokenPair(ctx, userID)
}

// Logout revokes the active access token JTI and any supplied refresh token JTI.
func (s *Service) Logout(ctx context.Context, accessClaims *TokenClaims, refreshToken string) error {
	if accessClaims != nil && accessClaims.ID != "" && accessClaims.ExpiresAt != nil {
		ttl := time.Until(accessClaims.ExpiresAt.Time)
		if ttl > 0 {
			if err := s.cache.SetMarker(ctx, revokedRedisKey(accessClaims.ID), "1", ttl); err != nil {
				return err
			}
		}
	}

	if strings.TrimSpace(refreshToken) != "" {
		if refreshClaims, err := s.ParseToken(refreshToken, "refresh"); err == nil && refreshClaims.ID != "" {
			_ = s.cache.Delete(ctx, refreshRedisKey(refreshClaims.ID))
		}
	}

	return nil
}

// DefaultGuestUserID identifies the login-free shared/guest workspace when no session is supplied.
var DefaultGuestUserID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

// GuestCookieName holds an optional per-browser guest workspace UUID cookie.
const GuestCookieName = "mediadock_guest_id"

// EnsureGuestWorkspace guarantees the guest workspace record and settings exist in the database.
func (s *Service) EnsureGuestWorkspace(ctx context.Context, guestID uuid.UUID) error {
	if guestID == uuid.Nil {
		guestID = DefaultGuestUserID
	}
	return s.users.EnsureGuestUser(ctx, guestID)
}

func (s *Service) guestSigningKey() []byte {
	if s.cfg != nil && s.cfg.LocalSigningSecret != "" {
		return []byte(s.cfg.LocalSigningSecret)
	}
	if s.cfg != nil && s.cfg.JWTSecret != "" {
		return []byte(s.cfg.JWTSecret)
	}
	return []byte("mediadock-guest-fallback-signing-key")
}

// SignGuestID creates an HMAC-signed cookie value (<uuid>.<hex>) for an anonymous visitor workspace.
func (s *Service) SignGuestID(guestID uuid.UUID) string {
	mac := hmac.New(sha256.New, s.guestSigningKey())
	mac.Write([]byte(guestID.String()))
	return guestID.String() + "." + hex.EncodeToString(mac.Sum(nil))
}

// VerifyGuestCookie validates an HMAC-signed guest cookie and returns the embedded workspace UUID.
func (s *Service) VerifyGuestCookie(raw string) (uuid.UUID, bool) {
	parts := strings.SplitN(strings.TrimSpace(raw), ".", 2)
	if len(parts) != 2 {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(parts[0])
	if err != nil || id == uuid.Nil {
		return uuid.Nil, false
	}
	mac := hmac.New(sha256.New, s.guestSigningKey())
	mac.Write([]byte(id.String()))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[1])) {
		return uuid.Nil, false
	}
	return id, true
}

// GetMe returns the profile for the given user ID.
func (s *Service) GetMe(ctx context.Context, userID uuid.UUID) (*UserProfile, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	profile := toUserProfile(user)
	return &profile, nil
}

// UpdateProfile updates the user's display name.
func (s *Service) UpdateProfile(ctx context.Context, userID uuid.UUID, name string) (*UserProfile, error) {
	user, err := s.users.UpdateName(ctx, userID, strings.TrimSpace(name))
	if err != nil {
		return nil, err
	}
	profile := toUserProfile(user)
	return &profile, nil
}

// CookieSettings returns token TTLs and production mode flag for HTTP-only cookie attributes.
func (s *Service) CookieSettings() (accessTTL, refreshTTL time.Duration, secure bool) {
	return s.cfg.JWTAccessTTL, s.cfg.JWTRefreshTTL, s.cfg.IsProduction()
}

// ParseAndValidateAccess parses an access JWT and checks Redis revocation.
func (s *Service) ParseAndValidateAccess(ctx context.Context, tokenStr string) (*TokenClaims, uuid.UUID, error) {
	claims, err := s.ParseToken(tokenStr, "access")
	if err != nil {
		return nil, uuid.Nil, err
	}

	revoked, err := s.cache.Exists(ctx, revokedRedisKey(claims.ID))
	if err != nil {
		return nil, uuid.Nil, err
	}
	if revoked {
		return nil, uuid.Nil, ErrTokenRevoked
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, uuid.Nil, ErrInvalidToken
	}
	return claims, userID, nil
}

// ParseToken verifies JWT signature, expiration, and expected token type.
func (s *Service) ParseToken(tokenStr, expectedType string) (*TokenClaims, error) {
	parsed, err := jwt.ParseWithClaims(tokenStr, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.cfg.JWTSecret), nil
	})
	if err != nil || !parsed.Valid {
		return nil, ErrInvalidToken
	}

	claims, ok := parsed.Claims.(*TokenClaims)
	if !ok || claims.TokenType != expectedType || claims.ID == "" || claims.UserID == "" {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func (s *Service) issueTokenPair(ctx context.Context, userID uuid.UUID) (*TokenPair, error) {
	now := time.Now().UTC()
	accessJTI := uuid.NewString()
	refreshJTI := uuid.NewString()

	accessClaims := TokenClaims{
		UserID:    userID.String(),
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        accessJTI,
			Subject:   userID.String(),
			Issuer:    "mediadock",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.JWTAccessTTL)),
		},
	}

	refreshClaims := TokenClaims{
		UserID:    userID.String(),
		TokenType: "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        refreshJTI,
			Subject:   userID.String(),
			Issuer:    "mediadock",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.JWTRefreshTTL)),
		},
	}

	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return nil, fmt.Errorf("sign access token: %w", err)
	}

	refreshToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return nil, fmt.Errorf("sign refresh token: %w", err)
	}

	if err := s.cache.SetMarker(ctx, refreshRedisKey(refreshJTI), userID.String(), s.cfg.JWTRefreshTTL); err != nil {
		return nil, fmt.Errorf("persist refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.cfg.JWTAccessTTL.Seconds()),
	}, nil
}

func toUserProfile(u *repository.User) UserProfile {
	return UserProfile{
		ID:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

func revokedRedisKey(jti string) string {
	return "auth:revoked:" + jti
}

func refreshRedisKey(jti string) string {
	return "auth:refresh:" + jti
}

