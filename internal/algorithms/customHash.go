package algorithms

import (
	"crypto/rand"
	"fmt"
)

const (
	loop  = 2579
	prime = 31
	mod   = 1 << 32
)

const saltChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func generateSalt(length int) (string, error) {
	osRandomBytes := make([]byte, length)
	_, err := rand.Read(osRandomBytes)

	if err != nil {
		return "", err
	}

	salt := make([]byte, length)

	for i, v := range osRandomBytes {
		salt[i] = saltChars[int(v)%len(saltChars)]
	}

	return string(salt), nil

}

func customHash(password, salt string) string {
	combined := password + salt
	var hash uint64 = 5381
	for i := 1; i <= loop; i++ {
		for _, char := range combined {
			hash = ((hash * prime) ^ uint64(char)) % mod
		}
	}
	return fmt.Sprintf("%x", hash)
}

func HashPassword(password string) (string, string, error) {
	salt, err := generateSalt(16)

	if err != nil {
		return "", "", err
	}

	hashedPassword := customHash(password, salt)

	return hashedPassword, salt, nil

}

func VerifyPassword(password, storedSalt, storedPassword string) bool {
	return storedPassword == customHash(password, storedSalt)
}
