package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims represents the JWT claims for FLIP2 authentication
type Claims struct {
	NodeID      string   `json:"node_id"`
	NodeType    string   `json:"node_type"`
	Permissions []string `json:"permissions"`
	jwt.RegisteredClaims
}

// JWTManager handles JWT token generation and validation
type JWTManager struct {
	secretKey     []byte
	tokenDuration time.Duration
}

// NewJWTManager creates a new JWT manager
func NewJWTManager(secretKey string, tokenDuration time.Duration) *JWTManager {
	return &JWTManager{
		secretKey:     []byte(secretKey),
		tokenDuration: tokenDuration,
	}
}

// GenerateSecretKey generates a random secret key
func GenerateSecretKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// GenerateToken creates a new JWT token for a node
func (m *JWTManager) GenerateToken(nodeID, nodeType string, permissions []string) (string, error) {
	claims := &Claims{
		NodeID:      nodeID,
		NodeType:    nodeType,
		Permissions: permissions,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.tokenDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "flip2",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secretKey)
}

// ValidateToken validates a JWT token and returns the claims
func (m *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return m.secretKey, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// Context key for claims
type contextKey string

const claimsContextKey contextKey = "claims"

// JWTMiddleware creates middleware that validates JWT tokens
func (m *JWTManager) JWTMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			// Also check X-API-Key for compatibility
			apiKey := r.Header.Get("X-API-Key")
			if apiKey != "" {
				// For API key auth, create synthetic claims
				claims := &Claims{
					NodeID:      "api-key-user",
					NodeType:    "client",
					Permissions: []string{"read", "write"},
				}
				ctx := context.WithValue(r.Context(), claimsContextKey, claims)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		// Parse Bearer token
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
			return
		}

		claims, err := m.ValidateToken(parts[1])
		if err != nil {
			http.Error(w, "Invalid token: "+err.Error(), http.StatusUnauthorized)
			return
		}

		// Add claims to context
		ctx := context.WithValue(r.Context(), claimsContextKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetClaims extracts claims from the request context
func GetClaims(r *http.Request) (*Claims, bool) {
	claims, ok := r.Context().Value(claimsContextKey).(*Claims)
	return claims, ok
}
