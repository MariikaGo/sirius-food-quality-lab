package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"sirius.local/food/internal/config"
	"sirius.local/food/internal/domain"
	"strconv"
	"time"
)

type Security struct {
	Secret []byte
	Cost   int
}

func NewSecurity(secret string) *Security {
	return &Security{[]byte(secret), config.Int("BCRYPT_COST", 12)}
}
func (s *Security) Hash(p string) (string, error) {
	b, e := bcrypt.GenerateFromPassword([]byte(p), s.Cost)
	return string(b), e
}
func (s *Security) Verify(p, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(p)) == nil
}
func Random() string {
	b := make([]byte, 48)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func HashToken(t string) string { h := sha256.Sum256([]byte(t)); return hex.EncodeToString(h[:]) }

type Claims struct {
	Version int64  `json:"ver"`
	Kind    string `json:"type"`
	jwt.RegisteredClaims
}

func (s *Security) Access(u domain.User) (string, error) {
	if len(s.Secret) < 32 {
		return "", fmt.Errorf("JWT_SECRET needs at least 32 bytes")
	}
	now := time.Now()
	c := Claims{u.Version, "access", jwt.RegisteredClaims{Issuer: "sirius-food", Subject: strconv.FormatInt(u.ID, 10), IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)), ID: Random()}}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(s.Secret)
}
func (s *Security) Decode(token string) (int64, int64, error) {
	c := new(Claims)
	_, e := jwt.ParseWithClaims(token, c, func(t *jwt.Token) (any, error) { return s.Secret, nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired(), jwt.WithIssuedAt(), jwt.WithIssuer("sirius-food"))
	if e != nil || c.Kind != "access" {
		return 0, 0, domain.Fail("unauthorized", "Недействительный токен")
	}
	id, e := strconv.ParseInt(c.Subject, 10, 64)
	if e != nil || id <= 0 {
		return 0, 0, domain.Fail("unauthorized", "Недействительный sub")
	}
	return id, c.Version, nil
}
