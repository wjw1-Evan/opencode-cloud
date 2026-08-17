package batch

import (
	"crypto/rand"
	"encoding/csv"
	"fmt"
	"io"
	"math/big"
	"sort"
	"strings"
)

const passwordChars = "abcdefghjkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// GenerateUsername produces a username with the given prefix and index,
// zero-padded to at least 3 digits, e.g. stu001, stu010, stu100.
func GenerateUsername(prefix string, index int) string {
	return fmt.Sprintf("%s%03d", prefix, index)
}

// SlugPrefix derives a username prefix from a course name: lowercases it and
// keeps only latin letters/digits; a leading digit gets an "s" prefix and an
// empty result falls back to "stu". e.g. "Python 基础" -> "python",
// "2026 春季班" -> "s2026", "算法课" -> "stu".
func SlugPrefix(course string) string {
	var b strings.Builder
	for _, r := range course {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + 32)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		}
	}
	s := b.String()
	if s == "" {
		return "stu"
	}
	if s[0] >= '0' && s[0] <= '9' {
		s = "s" + s
	}
	return s
}

// GeneratePassword returns a random printable password of the given length.
func GeneratePassword(length int) (string, error) {
	if length < 8 {
		length = 8
	}
	b := make([]byte, length)
	// rand.Int has no modulo bias, so every character is equally likely.
	max := big.NewInt(int64(len(passwordChars)))
	for i := range b {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b[i] = passwordChars[n.Int64()]
	}
	return string(b), nil
}

type Account struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// GenerateAccounts creates n accounts with unique usernames.
// Existing usernames (already in use) are skipped and a fresh index is used.
func GenerateAccounts(prefix string, n int, existing map[string]bool, passwordLen int) ([]Account, error) {
	var out []Account
	index := 1
	used := map[string]bool{}
	for len(out) < n {
		username := GenerateUsername(prefix, index)
		index++
		if existing[username] || used[username] {
			continue
		}
		pw, err := GeneratePassword(passwordLen)
		if err != nil {
			return nil, err
		}
		used[username] = true
		out = append(out, Account{Username: username, Password: pw})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Username < out[j].Username })
	return out, nil
}

// WriteAccountsCSV writes accounts as CSV with a header row.
func WriteAccountsCSV(w io.Writer, accounts []Account) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"username", "password"}); err != nil {
		return err
	}
	for _, a := range accounts {
		if err := cw.Write([]string{a.Username, a.Password}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// ParseUsernameLine splits a comma line into username and optional password.
func ParseUsernameLine(line string) (username, password string, err error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", "", fmt.Errorf("empty line")
	}
	parts := strings.Split(line, ",")
	username = strings.TrimSpace(parts[0])
	if username == "" {
		return "", "", fmt.Errorf("empty username")
	}
	if len(parts) > 1 {
		password = strings.TrimSpace(parts[1])
	}
	return username, password, nil
}
