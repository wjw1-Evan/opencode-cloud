package auth

import "testing"

func TestHashAndVerify(t *testing.T) {
	hash, err := HashPassword("s3cret-口令")
	if err != nil {
		t.Fatal(err)
	}
	ok, err := VerifyPassword("s3cret-口令", hash)
	if err != nil || !ok {
		t.Fatalf("expected valid password, ok=%v err=%v", ok, err)
	}
	ok, err = VerifyPassword("wrong", hash)
	if err != nil || ok {
		t.Fatalf("expected invalid password, ok=%v err=%v", ok, err)
	}
}

func TestVerifyRejectsMalformed(t *testing.T) {
	if _, err := VerifyPassword("x", "not-a-hash"); err == nil {
		t.Fatal("expected error for malformed hash")
	}
	if _, err := VerifyPassword("x", "$2y$10$ab"); err == nil {
		t.Fatal("expected error for bcrypt hash")
	}
}

func TestHashesAreUnique(t *testing.T) {
	h1, _ := HashPassword("same")
	h2, _ := HashPassword("same")
	if h1 == h2 {
		t.Fatal("hashes should include random salts")
	}
}
