package security

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"adlts/internal/platform/httpx"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type EntityType string

const (
	EntityCandidate         EntityType = "candidate"
	EntityExpert            EntityType = "expert"
	EntityAdmin             EntityType = "admin"
	EntitySuperAdmin        EntityType = "super_admin"
	EntityInstitute         EntityType = "institute"
	EntityTransportAuthority EntityType = "transport_authority"
)

type Claims struct {
	SubjectID    uuid.UUID  `json:"sub_id"`
	EntityType   EntityType `json:"entity_type"`
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
	now := time.Now().UTC()
	claims := Claims{
		SubjectID:    id,
		EntityType:   entityType,
		Email:        email,
		TestCenterID: testCenterID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   id.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(12 * time.Hour)),
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
				httpx.Failure(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing authorization header", nil)
				return
			}
			parts := strings.SplitN(tokenString, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				httpx.Failure(w, http.StatusUnauthorized, "INVALID_AUTH_HEADER", "invalid authorization header", nil)
				return
			}
			claims, err := manager.Parse(parts[1])
			if err != nil {
				httpx.Failure(w, http.StatusUnauthorized, "INVALID_TOKEN", "invalid token", nil)
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
				httpx.Failure(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required", nil)
				return
			}
			for _, e := range allowed {
				if auth.EntityType == e {
					next.ServeHTTP(w, r)
					return
				}
			}
			httpx.Failure(w, http.StatusForbidden, "FORBIDDEN", "forbidden", nil)
		})
	}
}

func RequireInternalToken(expected string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if expected == "" {
				httpx.Failure(w, http.StatusForbidden, "FORBIDDEN", "internal token not configured", nil)
				return
			}
			if got := r.Header.Get("X-Internal-Token"); got != expected {
				httpx.Failure(w, http.StatusForbidden, "FORBIDDEN", "invalid internal token", nil)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
