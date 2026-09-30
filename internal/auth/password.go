package auth

import "golang.org/x/crypto/bcrypt"

// Password hashing. bcrypt with DefaultCost (10) is the boring, correct
// choice; ~60ms per hash on modern hardware keeps brute force expensive
// without hurting login UX.

// HashPassword hashes a plaintext password for storage.
func HashPassword(plain string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

// CheckPassword reports whether plain matches the stored hash. An empty
// stored hash NEVER matches — accounts provisioned without a password
// stay un-loginable.
func CheckPassword(storedHash, plain string) bool {
	if storedHash == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(plain)) == nil
}
