package jwt

import (
	"errors"
	"huozhi/internal/config"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID   uint   `json:"uid"`
	Username string `json:"username"`
	// TokenVersion 随 User.TokenVersion 一起签进 token。
	// 改密码时 User.TokenVersion+1，旧 token 的 tv 对不上即失效——
	// 不需要维护黑名单，一次查库就能判定。
	TokenVersion int `json:"tv"`
	jwt.RegisteredClaims
}

// GenerateToken 签发 token。tokenVersion 必须取自 User.TokenVersion，
// 传 0 只在测试里可接受：真实签发传错会让已登录用户被误踢。
func GenerateToken(userID uint, username string, tokenVersion int) (string, error) {
	cfg := config.AppConfig.JWT
	now := time.Now()
	claims := Claims{
		UserID:       userID,
		Username:     username,
		TokenVersion: tokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(cfg.ExpireHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    cfg.Issuer,
			Subject:   username,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.Secret))
}

func ParseToken(tokenString string) (*Claims, error) {
	cfg := config.AppConfig.JWT
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(cfg.Secret), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token")
}
