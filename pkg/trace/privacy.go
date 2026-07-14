package trace

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"unicode"
	"unicode/utf8"
)

const (
	minimumProtectionKeyBytes   = 32
	maximumProtectionKeyBytes   = 4 << 10
	maximumProtectionInputBytes = 1024
	maximumFingerprintTokens    = 4_000_000
)

// Protector creates stable, domain-separated identifiers without storing raw
// tenant names or reversible prompt prefixes. It is safe for concurrent use.
type Protector struct {
	key []byte
}

// NewProtector copies key and returns a privacy helper. Production keys must be
// generated randomly, stored outside traces, and rotated under operator policy.
func NewProtector(key []byte) (*Protector, error) {
	if len(key) < minimumProtectionKeyBytes || len(key) > maximumProtectionKeyBytes {
		return nil, fmt.Errorf("%w: key must contain %d..%d bytes", ErrInvalidProtectionKey, minimumProtectionKeyBytes, maximumProtectionKeyBytes)
	}
	return &Protector{key: append([]byte(nil), key...)}, nil
}

// TenantID returns a stable HMAC pseudonym for a tenant identifier.
func (p *Protector) TenantID(tenant string) (string, error) {
	if p == nil || len(p.key) < minimumProtectionKeyBytes {
		return "", ErrInvalidProtectionKey
	}
	if tenant == "" || len(tenant) > maximumProtectionInputBytes || !utf8.ValidString(tenant) {
		return "", fmt.Errorf("%w: tenant must be valid UTF-8 and contain 1..%d bytes", ErrInvalidRecord, maximumProtectionInputBytes)
	}
	for _, r := range tenant {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("%w: tenant contains control characters", ErrInvalidRecord)
		}
	}
	mac := hmac.New(sha256.New, p.key)
	_, _ = mac.Write([]byte("inferlab/tenant/v1\x00"))
	_, _ = mac.Write([]byte(tenant))
	return tenantDigestPrefix + hex.EncodeToString(mac.Sum(nil)), nil
}

// PrefixFingerprint returns a stable HMAC over model identity and token IDs.
// Token IDs avoid accepting or retaining raw prompt bytes in the trace API.
func (p *Protector) PrefixFingerprint(model string, tokenIDs []uint32) (string, error) {
	if p == nil || len(p.key) < minimumProtectionKeyBytes {
		return "", ErrInvalidProtectionKey
	}
	if err := validateIdentifier("model", model, maxIdentifierBytes); err != nil {
		return "", err
	}
	if len(tokenIDs) == 0 || len(tokenIDs) > maximumFingerprintTokens {
		return "", fmt.Errorf("%w: prefix token count must be between 1 and %d", ErrTokenLimit, maximumFingerprintTokens)
	}

	mac := hmac.New(sha256.New, p.key)
	_, _ = mac.Write([]byte("inferlab/prefix/v1\x00"))
	var encoded [8]byte
	binary.BigEndian.PutUint32(encoded[:4], uint32(len(model)))
	_, _ = mac.Write(encoded[:4])
	_, _ = mac.Write([]byte(model))
	binary.BigEndian.PutUint64(encoded[:], uint64(len(tokenIDs)))
	_, _ = mac.Write(encoded[:])
	for _, tokenID := range tokenIDs {
		binary.BigEndian.PutUint32(encoded[:4], tokenID)
		_, _ = mac.Write(encoded[:4])
	}
	return prefixDigestPrefix + hex.EncodeToString(mac.Sum(nil)), nil
}
