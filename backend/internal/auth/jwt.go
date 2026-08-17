package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	jtiAccess  = "access"
	jtiRefresh = "refresh"
)

type Claims struct {
	UserID   string `json:"uid"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

type TokenManager struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	issuer     string
}

func NewTokenManager(secret string) *TokenManager {
	return &TokenManager{
		secret:     []byte(secret),
		accessTTL:  30 * time.Minute,
		refreshTTL: 24 * time.Hour,
		issuer:     "devcapsule",
	}
}

func (tm *TokenManager) Issue(userID, username, role string) (access, refresh string, err error) {
	now := time.Now()
	access, err = tm.sign(Claims{
		UserID: userID, Username: username, Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: tm.issuer, Subject: userID,
			ExpiresAt: jwt.NewNumericDate(now.Add(tm.accessTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        newJTI(userID, jtiAccess),
		},
	})
	if err != nil {
		return "", "", err
	}
	refresh, err = tm.sign(Claims{
		UserID: userID, Username: username, Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: tm.issuer, Subject: userID,
			ExpiresAt: jwt.NewNumericDate(now.Add(tm.refreshTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        newJTI(userID, jtiRefresh),
		},
	})
	if err != nil {
		return "", "", err
	}
	return access, refresh, nil
}

func (tm *TokenManager) Parse(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return tm.secret, nil
	}, jwt.WithIssuer(tm.issuer), jwt.WithExpirationRequired())
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

// newJTI builds a unique token ID: kind is the first segment so the token
// type can be checked without decoding, and the random suffix guarantees two
// tokens issued in the same second are still distinct.
func newJTI(userID, kind string) string {
	var rnd [6]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		return kind + ":" + userID
	}
	return kind + ":" + userID + ":" + hex.EncodeToString(rnd[:])
}

// isTokenType reports whether the jti belongs to the given token kind. Both
// the new ("<kind>:<uid>:<rand>") and the legacy ("<uid>:<kind>") formats are
// recognized so tokens issued before the format change keep working.
func isTokenType(id, kind string) bool {
	if strings.HasPrefix(id, kind+":") {
		return true
	}
	return strings.HasSuffix(id, ":"+kind)
}

// ParseAccess validates an access token. Refresh tokens are rejected so a
// stolen refresh cookie cannot be used as a 24h access credential.
func (tm *TokenManager) ParseAccess(tokenString string) (*Claims, error) {
	c, err := tm.Parse(tokenString)
	if err != nil {
		return nil, err
	}
	if isTokenType(c.ID, jtiRefresh) {
		return nil, errors.New("refresh token cannot be used as an access token")
	}
	return c, nil
}

// ParseRefresh validates a refresh token. Access tokens cannot be used to
// obtain new token pairs.
func (tm *TokenManager) ParseRefresh(tokenString string) (*Claims, error) {
	c, err := tm.Parse(tokenString)
	if err != nil {
		return nil, err
	}
	if isTokenType(c.ID, jtiAccess) {
		return nil, errors.New("access token cannot be used as a refresh token")
	}
	return c, nil
}

func (tm *TokenManager) sign(c Claims) (string, error) {
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(tm.secret)
}
