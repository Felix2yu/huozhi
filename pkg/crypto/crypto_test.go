package crypto

import "testing"

func TestKeyFromSecret(t *testing.T) {
	k := KeyFromSecret("abc")
	if len(k) != 32 {
		t.Fatalf("key must be 32 bytes, got %d", len(k))
	}
	if string(KeyFromSecret("abc")) != string(k) {
		t.Fatal("KeyFromSecret must be deterministic")
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := KeyFromSecret("secret")
	ct, err := Encrypt([]byte("hello world"), key)
	if err != nil {
		t.Fatal(err)
	}
	if ct == "" {
		t.Fatal("empty ciphertext")
	}
	pt, err := Decrypt(ct, key)
	if err != nil {
		t.Fatal(err)
	}
	if pt != "hello world" {
		t.Fatalf("round-trip mismatch, got %q", pt)
	}
}

func TestEncryptEmpty(t *testing.T) {
	ct, err := Encrypt([]byte(""), KeyFromSecret("x"))
	if err != nil {
		t.Fatal(err)
	}
	if ct != "" {
		t.Fatalf("expected empty ciphertext, got %q", ct)
	}
}

func TestDecryptEmpty(t *testing.T) {
	pt, err := Decrypt("", KeyFromSecret("x"))
	if err != nil {
		t.Fatal(err)
	}
	if pt != "" {
		t.Fatal("expected empty plaintext")
	}
}

func TestDecryptInvalidBase64(t *testing.T) {
	if _, err := Decrypt("!!!not-base64!!!", KeyFromSecret("x")); err == nil {
		t.Fatal("expected base64 error")
	}
}

func TestDecryptTooShort(t *testing.T) {
	// valid base64 but shorter than the GCM nonce
	if _, err := Decrypt("AAAA", KeyFromSecret("x")); err == nil {
		t.Fatal("expected too-short error")
	}
}

func TestCipherInvalidKeySize(t *testing.T) {
	short := []byte("short")
	if _, err := Encrypt([]byte("x"), short); err == nil {
		t.Fatal("expected key-size error on encrypt")
	}
	if _, err := Decrypt("AAAA", short); err == nil {
		t.Fatal("expected key-size error on decrypt")
	}
}
