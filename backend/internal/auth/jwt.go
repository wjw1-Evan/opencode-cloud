package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
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
			ID:        userID + ":access",
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
			ID:        userID + ":refresh",
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

func (tm *TokenManager) sign(c Claims) (string, error) {
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(tm.secret)
}
