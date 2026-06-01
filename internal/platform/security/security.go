package security

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type EntityType string

const (
	EntityCandidate          EntityType = "candidate"
	EntityExpert             EntityType = "expert"
	EntityAdmin              EntityType = "admin"
	EntitySuperAdmin         EntityType = "super_admin"
	EntityInstitute          EntityType = "institute"
	EntityTransportAuthority EntityType = "transport_authority"
)

type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)

type Claims struct {
	SubjectID    uuid.UUID  `json:"sub_id"`
	EntityType   EntityType `json:"entity_type"`
	TokenType    TokenType  `json:"token_type"`
	Email        string     `json:"email"`
	TestCenterID *uuid.UUID `json:"test_center_id,omitempty"`
	jwt.RegisteredClaims
}

type AuthContext struct {
	SubjectID    uuid.UUID
	EntityType   EntityType
	Email        string
	TestCenterID *uuid.UUID
}

type Manager struct {
	secret []byte
}

type authContextKey struct{}

func NewManager(secret string) *Manager {
	return &Manager{secret: []byte(secret)}
}

func (m *Manager) Sign(id uuid.UUID, entityType EntityType, email string, testCenterID *uuid.UUID) (string, error) {
	return m.SignAccessToken(id, entityType, email, testCenterID)
}

func (m *Manager) SignAccessToken(id uuid.UUID, entityType EntityType, email string, testCenterID *uuid.UUID) (string, error) {
	return m.signWithTTL(id, entityType, email, testCenterID, TokenTypeAccess, 12*time.Hour)
}

func (m *Manager) SignRefreshToken(id uuid.UUID, entityType EntityType, email string, testCenterID *uuid.UUID) (string, error) {
	return m.signWithTTL(id, entityType, email, testCenterID, TokenTypeRefresh, 7*24*time.Hour)
}

func (m *Manager) signWithTTL(id uuid.UUID, entityType EntityType, email string, testCenterID *uuid.UUID, tokenType TokenType, ttl time.Duration) (string, error) {
	now := time.Now().UTC()
	claims := Claims{
		SubjectID:    id,
		EntityType:   entityType,
		TokenType:    tokenType,
		Email:        email,
		TestCenterID: testCenterID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   id.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			NotBefore: jwt.NewNumericDate(now.Add(-1 * time.Minute)),
			Issuer:    "adlts",
			Audience:  []string{"adlts-api"},
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

func (m *Manager) Parse(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %s", token.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	if claims.TokenType == "" {
		claims.TokenType = TokenTypeAccess
	}
	return claims, nil
}

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func CheckPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func Authenticate(manager *Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := r.Header.Get("Authorization")
			if tokenString == "" {
				http.Error(w, "missing authorization header", http.StatusUnauthorized)
				return
			}
			parts := strings.SplitN(tokenString, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				http.Error(w, "invalid authorization header", http.StatusUnauthorized)
				return
			}
			claims, err := manager.Parse(parts[1])
			if err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), authContextKey{}, &AuthContext{
				SubjectID:    claims.SubjectID,
				EntityType:   claims.EntityType,
				Email:        claims.Email,
				TestCenterID: claims.TestCenterID,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func CurrentAuth(r *http.Request) (*AuthContext, bool) {
	auth, ok := r.Context().Value(authContextKey{}).(*AuthContext)
	return auth, ok
}

func RequireEntities(allowed ...EntityType) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth, ok := CurrentAuth(r)
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			for _, e := range allowed {
				if auth.EntityType == e {
					next.ServeHTTP(w, r)
					return
				}
			}
			http.Error(w, "forbidden", http.StatusForbidden)
		})
	}
}

func RequireInternalToken(expected string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if expected == "" {
				http.Error(w, "internal token not configured", http.StatusForbidden)
				return
			}
			if got := r.Header.Get("X-Internal-Token"); got != expected {
				http.Error(w, "invalid internal token", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
