package main

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	bcryptCost     = 12
	tokenDuration  = 7 * 24 * time.Hour
	contextKeyUser = contextKey("user")
)

// Sentinel errors for user-facing auth failures.
var (
	ErrEmailRequired    = errors.New("email is required")
	ErrPasswordTooShort = errors.New("password must be at least 8 characters")
	ErrPasswordTooLong  = errors.New("password is too long (maximum 128 characters)")
	ErrEmailTaken       = errors.New("email already registered")
	ErrInvalidCreds     = errors.New("invalid email or password")
	ErrRegistrationOff  = errors.New("registration is disabled")
)

type contextKey string

// Claims is the JWT payload.
type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// AuthService wraps JWT and bcrypt operations.
type AuthService struct {
	db     *sql.DB
	secret []byte
}

func NewAuthService(db *sql.DB, secret string) *AuthService {
	return &AuthService{db: db, secret: []byte(secret)}
}

// Register creates a new user. Returns the user and a signed JWT.
func (a *AuthService) Register(email, password string) (User, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return User{}, "", ErrEmailRequired
	}
	if len(password) < 8 {
		return User{}, "", ErrPasswordTooShort
	}
	if len(password) > 128 {
		return User{}, "", ErrPasswordTooLong
	}

	// Check for duplicate email.
	existing, _, err := getUserByEmail(a.db, email)
	if err != nil {
		return User{}, "", fmt.Errorf("check email: %w", err)
	}
	if existing.ID != "" {
		return User{}, "", ErrEmailTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return User{}, "", fmt.Errorf("hash password: %w", err)
	}

	user, err := createUser(a.db, email, string(hash))
	if err != nil {
		return User{}, "", fmt.Errorf("create user: %w", err)
	}

	token, err := a.signToken(user)
	if err != nil {
		return User{}, "", err
	}

	return user, token, nil
}

// Login validates credentials and returns a JWT.
func (a *AuthService) Login(email, password string) (User, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	user, hash, err := getUserByEmail(a.db, email)
	if err != nil {
		return User{}, "", fmt.Errorf("lookup user: %w", err)
	}
	if user.ID == "" {
		return User{}, "", ErrInvalidCreds
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return User{}, "", ErrInvalidCreds
	}

	token, err := a.signToken(user)
	if err != nil {
		return User{}, "", err
	}

	return user, token, nil
}

// ValidateToken parses and validates a JWT, returning the claims.
func (a *AuthService) ValidateToken(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return a.secret, nil
	})
	if err != nil || !token.Valid {
		return nil, errors.New("invalid or expired token")
	}
	return claims, nil
}

func (a *AuthService) signToken(user User) (string, error) {
	claims := &Claims{
		UserID: user.ID,
		Email:  user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(tokenDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   user.ID,
			ID:        uuid.New().String(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.secret)
}
