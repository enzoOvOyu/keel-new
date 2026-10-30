package identity

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"golang.org/x/crypto/argon2"
	"strconv"
	"strings"
)

func HashPassword(password string) (string, error) {
	if len(password) < 12 || len(password) > 256 {
		return "", fmt.Errorf("PASSWORD_LENGTH")
	}
	salt := make([]byte, 16)
	if _, e := rand.Read(salt); e != nil {
		return "", e
	}
	hash := argon2.IDKey([]byte(password), salt, 3, 64*1024, 2, 32)
	b := base64.RawStdEncoding
	return "$argon2id$v=19$m=65536,t=3,p=2$" + b.EncodeToString(salt) + "$" + b.EncodeToString(hash), nil
}
func VerifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" || len(password) > 256 {
		return false
	}
	params := strings.Split(parts[3], ",")
	if len(params) != 3 {
		return false
	}
	m, e := strconv.Atoi(strings.TrimPrefix(params[0], "m="))
	t, e2 := strconv.Atoi(strings.TrimPrefix(params[1], "t="))
	p, e3 := strconv.Atoi(strings.TrimPrefix(params[2], "p="))
	if e != nil || e2 != nil || e3 != nil || m != 65536 || t != 3 || p != 2 {
		return false
	}
	salt, e := base64.RawStdEncoding.DecodeString(parts[4])
	want, e2 := base64.RawStdEncoding.DecodeString(parts[5])
	if e != nil || e2 != nil || len(salt) != 16 || len(want) != 32 {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, uint32(t), uint32(m), uint8(p), 32)
	return subtle.ConstantTimeCompare(got, want) == 1
}
