package batch

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"
)

func TestSlugPrefix(t *testing.T) {
	cases := []struct {
		course string
		want   string
	}{
		{"Python 基础", "python"},
		{"2026 春季班", "s2026"},
		{"算法课", "stu"},
		{"Data Structures", "datastructures"},
		{"AI & ML Lab", "aimllab"},
		{"", "stu"},
	}
	for _, c := range cases {
		if got := SlugPrefix(c.course); got != c.want {
			t.Errorf("SlugPrefix(%q)=%q want %q", c.course, got, c.want)
		}
	}
}

func TestGenerateUsername(t *testing.T) {
	cases := []struct {
		prefix string
		index  int
		want   string
	}{
		{"stu", 1, "stu001"},
		{"stu", 50, "stu050"},
		{"stu", 100, "stu100"},
		{"user", 7, "user007"},
	}
	for _, c := range cases {
		if got := GenerateUsername(c.prefix, c.index); got != c.want {
			t.Errorf("GenerateUsername(%s,%d)=%s want %s", c.prefix, c.index, got, c.want)
		}
	}
}

func TestGeneratePassword(t *testing.T) {
	for length := 8; length <= 16; length++ {
		pw, err := GeneratePassword(length)
		if err != nil {
			t.Fatal(err)
		}
		if len(pw) != length {
			t.Errorf("len=%d want %d", len(pw), length)
		}
		for _, ch := range pw {
			if !strings.ContainsRune(passwordChars, ch) {
				t.Errorf("password contains invalid char %q", ch)
			}
		}
	}
	// minimum length clamp
	pw, _ := GeneratePassword(4)
	if len(pw) != 8 {
		t.Errorf("expected clamped length 8, got %d", len(pw))
	}
}

func TestGenerateAccountsUniqueAndSkippingTaken(t *testing.T) {
	existing := map[string]bool{"stu001": true, "stu003": true}
	accounts, err := GenerateAccounts("stu", 3, existing, 12)
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 3 {
		t.Fatalf("expected 3 accounts, got %d", len(accounts))
	}
	seen := map[string]bool{}
	for _, a := range accounts {
		if existing[a.Username] {
			t.Fatalf("generated taken username %s", a.Username)
		}
		if seen[a.Username] {
			t.Fatalf("duplicate username %s", a.Username)
		}
		seen[a.Username] = true
		if len(a.Password) != 12 {
			t.Errorf("password length %d want 12", len(a.Password))
		}
	}
	// must fill to 3 even though stu001/stu003 skipped
	if seen["stu002"] == false && seen["stu004"] == false {
		t.Log("ok: skipped indices were avoided")
	}
	if len(seen) != 3 {
		t.Fatalf("expected 3 unique usernames, got %v", seen)
	}
}

func TestWriteAccountsCSV(t *testing.T) {
	var buf bytes.Buffer
	accounts := []Account{{Username: "stu001", Password: "pw1"}, {Username: "stu002", Password: "pw2"}}
	if err := WriteAccountsCSV(&buf, accounts); err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(&buf).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected header + 2 rows, got %d", len(rows))
	}
	if rows[0][0] != "username" {
		t.Fatalf("missing header: %v", rows[0])
	}
	if rows[1][1] != "pw1" {
		t.Fatalf("bad row: %v", rows[1])
	}
}

func TestParseUsernameLine(t *testing.T) {
	u, pw, err := ParseUsernameLine("  alice  ,  secret1  ")
	if err != nil {
		t.Fatal(err)
	}
	if u != "alice" || pw != "secret1" {
		t.Fatalf("got %q %q", u, pw)
	}
	u, pw, err = ParseUsernameLine("bob")
	if err != nil || u != "bob" || pw != "" {
		t.Fatalf("got %q %q err=%v", u, pw, err)
	}
	if _, _, err := ParseUsernameLine("  "); err == nil {
		t.Fatal("expected error for empty line")
	}
}
