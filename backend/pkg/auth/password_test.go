package auth

import "testing"

func TestHashAndCheck(t *testing.T) {
	h, err := HashPassword("secret123")
	if err != nil {
		t.Fatal(err)
	}
	if h == "" {
		t.Fatal("empty hash")
	}
	if !CheckPassword("secret123", h) {
		t.Fatal("valid password should match")
	}
	if CheckPassword("wrong-password", h) {
		t.Fatal("wrong password should not match")
	}
}
