package password

import "testing"

func TestArgon2RoundTrip(t *testing.T) {
	h, err := New(Params{
		Memory:  8,
		Time:    1,
		Threads: 1,
		SaltLen: 16,
		KeyLen:  16,
		Pepper:  []byte("pepper-value-012345"),
	})
	if err != nil {
		t.Fatal(err)
	}

	encoded, err := h.Hash("S3cure-Wiki-Pass")
	if err != nil {
		t.Fatal(err)
	}

	ok, err := h.Verify("S3cure-Wiki-Pass", encoded)
	if err != nil || !ok {
		t.Fatalf("проверка, верно=%v ошибка=%v", ok, err)
	}

	ok, err = h.Verify("wrong-password", encoded)
	if err != nil || ok {
		t.Fatalf("неверный, верно=%v ошибка=%v", ok, err)
	}

	other, err := New(Params{
		Memory:  8,
		Time:    1,
		Threads: 1,
		SaltLen: 16,
		KeyLen:  16,
		Pepper:  []byte("other-pepper-012345"),
	})
	if err != nil {
		t.Fatal(err)
	}

	ok, err = other.Verify("S3cure-Wiki-Pass", encoded)
	if err != nil || ok {
		t.Fatalf("секрет не совпал, верно=%v ошибка=%v", ok, err)
	}
}
