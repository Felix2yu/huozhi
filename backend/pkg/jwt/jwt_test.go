package jwt

import (
	"testing"

	"huozhi/internal/config"
)

func init() {
	config.AppConfig = &config.Config{
		JWT: config.JWTConfig{Secret: "test-secret", ExpireHours: 168, Issuer: "huozhi"},
	}
}

func TestGenerateAndParse(t *testing.T) {
	tok, err := GenerateToken(42, "alice", 7)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ParseToken(tok)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != 42 {
		t.Fatalf("user id mismatch: %d", claims.UserID)
	}
	if claims.Username != "alice" {
		t.Fatalf("username mismatch: %s", claims.Username)
	}
	if claims.Issuer != "huozhi" {
		t.Fatalf("issuer mismatch: %s", claims.Issuer)
	}
	// tv 必须原样带出，否则中间件拿它比对版本会恒等/恒不等
	if claims.TokenVersion != 7 {
		t.Fatalf("token version mismatch: %d", claims.TokenVersion)
	}
}

func TestParseInvalid(t *testing.T) {
	if _, err := ParseToken("not-a-token"); err == nil {
		t.Fatal("expected error for invalid token")
	}
}
