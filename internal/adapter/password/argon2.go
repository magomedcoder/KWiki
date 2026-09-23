package password

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

type Params struct {
	Memory  uint32
	Time    uint32
	Threads uint8
	SaltLen uint32
	KeyLen  uint32
	Pepper  []byte
}

type Hasher struct {
	params Params
	dummy  string
}

func Recommended(pepper []byte) (*Hasher, error) {
	return New(Params{
		Memory:  64 * 1024,
		Time:    3,
		Threads: 1,
		SaltLen: 16,
		KeyLen:  32,
		Pepper:  pepper,
	})
}

func New(p Params) (*Hasher, error) {
	if p.Memory == 0 || p.Time == 0 || p.Threads == 0 || p.SaltLen < 16 || p.KeyLen < 16 {
		return nil, errors.New("слишком слабые параметры хеша пароля")
	}

	h := &Hasher{params: p}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}

	dummy, err := h.Hash(string(secret))
	if err != nil {
		return nil, err
	}
	h.dummy = dummy

	return h, nil
}

func (h *Hasher) Dummy() string {
	return h.dummy
}

func (h *Hasher) Hash(password string) (string, error) {
	salt := make([]byte, h.params.SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	sum := argon2.IDKey(h.material(password), salt, h.params.Time, h.params.Memory, h.params.Threads, h.params.KeyLen)
	return fmt.Sprintf(
		"argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		h.params.Memory,
		h.params.Time,
		h.params.Threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(sum),
	), nil
}

func (h *Hasher) Verify(password, encoded string) (bool, error) {
	memory, timeCost, threads, salt, sum, err := decode(encoded)
	if err != nil {
		return false, err
	}

	got := argon2.IDKey(h.material(password), salt, timeCost, memory, threads, uint32(len(sum)))
	return subtle.ConstantTimeCompare(got, sum) == 1, nil
}

func (h *Hasher) material(password string) []byte {
	if len(h.params.Pepper) == 0 {
		return []byte(password)
	}

	mac := hmac.New(sha256.New, h.params.Pepper)
	_, _ = mac.Write([]byte(password))
	return mac.Sum(nil)
}

func decode(encoded string) (memory, timeCost uint32, threads uint8, salt, sum []byte, err error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 5 || parts[0] != "argon2id" || parts[1] != "v=19" {
		return 0, 0, 0, nil, nil, errors.New("неизвестный формат хеша")
	}

	var m, t, p int
	if _, err := fmt.Sscanf(parts[2], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return 0, 0, 0, nil, nil, err
	}

	if m <= 0 || t <= 0 || p <= 0 || p > 255 {
		return 0, 0, 0, nil, nil, errors.New("некорректные параметры хеша")
	}

	salt, err = base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return 0, 0, 0, nil, nil, err
	}

	sum, err = base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return 0, 0, 0, nil, nil, err
	}

	return uint32(m), uint32(t), uint8(p), salt, sum, nil
}
