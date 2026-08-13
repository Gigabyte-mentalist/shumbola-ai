// Package auth implements security-key authentication and role-based
// access control (RBAC), per TZ section 3: Operator, Administrator, and
// Auditor roles, with all active actions requiring an authenticated
// operator (TZ 6.1).
package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
)

// Role identifies what a principal is allowed to do.
type Role string

const (
	RoleOperator      Role = "operator"
	RoleAdministrator Role = "administrator"
	RoleAuditor       Role = "auditor"
)

// Principal is the authenticated identity attached to a request.
type Principal struct {
	ID   string
	Role Role
}

var ErrInvalidAPIKey = errors.New("auth: invalid api key")

// HashAPIKey returns the hex-encoded SHA-256 digest of a raw API key.
// Only hashed keys are ever stored or compared.
func HashAPIKey(rawKey string) string {
	sum := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(sum[:])
}

// KeyStore resolves a hashed API key to the principal it belongs to.
type KeyStore interface {
	Lookup(hashedKey string) (Principal, bool)
}

// InMemoryKeyStore is a dev/seed-time implementation of KeyStore. In
// later stages this is replaced by a persistent, admin-managed store.
type InMemoryKeyStore struct {
	byHash map[string]Principal
}

// NewInMemoryKeyStore builds a store from raw (unhashed) seed keys,
// hashing them before holding them in memory.
func NewInMemoryKeyStore(seeds map[string]Principal) *InMemoryKeyStore {
	byHash := make(map[string]Principal, len(seeds))
	for rawKey, p := range seeds {
		byHash[HashAPIKey(rawKey)] = p
	}
	return &InMemoryKeyStore{byHash: byHash}
}

// Lookup implements KeyStore using constant-time comparison over the
// hashed key to avoid timing side-channels.
func (s *InMemoryKeyStore) Lookup(hashedKey string) (Principal, bool) {
	for storedHash, p := range s.byHash {
		if subtle.ConstantTimeCompare([]byte(storedHash), []byte(hashedKey)) == 1 {
			return p, true
		}
	}
	return Principal{}, false
}

// Authenticate resolves a raw API key (as received over the wire) to a
// Principal via the given store.
func Authenticate(store KeyStore, rawKey string) (Principal, error) {
	if rawKey == "" {
		return Principal{}, ErrInvalidAPIKey
	}
	p, ok := store.Lookup(HashAPIKey(rawKey))
	if !ok {
		return Principal{}, ErrInvalidAPIKey
	}
	return p, nil
}

// HasRole reports whether the principal's role is one of the allowed roles.
func HasRole(p Principal, allowed ...Role) bool {
	for _, r := range allowed {
		if p.Role == r {
			return true
		}
	}
	return false
}
