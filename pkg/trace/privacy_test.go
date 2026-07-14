package trace

import (
	"errors"
	"strings"
	"testing"
)

func TestProtectorTenantID(t *testing.T) {
	t.Parallel()

	key := []byte("0123456789abcdef0123456789abcdef")
	protector, err := NewProtector(key)
	if err != nil {
		t.Fatalf("NewProtector() error: %v", err)
	}
	first, err := protector.TenantID("payments-copilot")
	if err != nil {
		t.Fatalf("TenantID() error: %v", err)
	}
	second, err := protector.TenantID("payments-copilot")
	if err != nil {
		t.Fatalf("TenantID() repeated error: %v", err)
	}
	if first != second {
		t.Fatalf("TenantID() is not deterministic: %q != %q", first, second)
	}
	if strings.Contains(first, "payments") || !validDigest(first, tenantDigestPrefix) {
		t.Fatalf("TenantID() returned unsafe or invalid pseudonym %q", first)
	}
	want := tenantDigestPrefix + "fd3b6db1b1fb3b355cd91b1b69d46e7f00915f17e5761596a79a5f7a4c335167"
	if first != want {
		t.Fatalf("TenantID() = %q, want compatibility digest %q", first, want)
	}

	key[0] ^= 0xff
	afterMutation, err := protector.TenantID("payments-copilot")
	if err != nil {
		t.Fatalf("TenantID() after key mutation error: %v", err)
	}
	if afterMutation != first {
		t.Fatal("NewProtector() did not copy the protection key")
	}
}

func TestProtectorPrefixFingerprint(t *testing.T) {
	t.Parallel()

	protector, err := NewProtector([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("NewProtector() error: %v", err)
	}
	tokens := []uint32{1, 42, 65_535, 17}
	first, err := protector.PrefixFingerprint("qwen-32b", tokens)
	if err != nil {
		t.Fatalf("PrefixFingerprint() error: %v", err)
	}
	second, err := protector.PrefixFingerprint("qwen-32b", tokens)
	if err != nil {
		t.Fatalf("PrefixFingerprint() repeated error: %v", err)
	}
	otherModel, err := protector.PrefixFingerprint("qwen-14b", tokens)
	if err != nil {
		t.Fatalf("PrefixFingerprint() other model error: %v", err)
	}
	otherTokens, err := protector.PrefixFingerprint("qwen-32b", []uint32{1, 42, 65_535, 18})
	if err != nil {
		t.Fatalf("PrefixFingerprint() other tokens error: %v", err)
	}
	if first != second || first == otherModel || first == otherTokens || !validDigest(first, prefixDigestPrefix) {
		t.Fatalf("unexpected fingerprint behavior: first=%q second=%q otherModel=%q otherTokens=%q", first, second, otherModel, otherTokens)
	}
	want := prefixDigestPrefix + "93737e470561434a5c2d8e46895542ef97ec8ad8b97443d97a0ddc6697775473"
	if first != want {
		t.Fatalf("PrefixFingerprint() = %q, want compatibility digest %q", first, want)
	}
}

func TestProtectorRejectsUnsafeInputs(t *testing.T) {
	t.Parallel()

	if _, err := NewProtector([]byte("short")); !errors.Is(err, ErrInvalidProtectionKey) {
		t.Fatalf("NewProtector() error = %v, want ErrInvalidProtectionKey", err)
	}
	if _, err := NewProtector(make([]byte, maximumProtectionKeyBytes+1)); !errors.Is(err, ErrInvalidProtectionKey) {
		t.Fatalf("NewProtector() oversized key error = %v, want ErrInvalidProtectionKey", err)
	}
	protector, err := NewProtector([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("NewProtector() error: %v", err)
	}
	if _, err := protector.TenantID(""); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("TenantID() error = %v, want ErrInvalidRecord", err)
	}
	if _, err := protector.PrefixFingerprint("model", nil); !errors.Is(err, ErrTokenLimit) {
		t.Fatalf("PrefixFingerprint() error = %v, want ErrTokenLimit", err)
	}
}

func FuzzProtectorDeterministic(f *testing.F) {
	f.Add("tenant-a", "model", []byte{1, 2, 3, 4})
	f.Add("payments", "qwen-32b", []byte{0, 0, 0, 1})
	protector, err := NewProtector([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		f.Fatalf("NewProtector() error: %v", err)
	}
	f.Fuzz(func(t *testing.T, tenant, model string, tokenBytes []byte) {
		if tenant != "" {
			first, firstErr := protector.TenantID(tenant)
			second, secondErr := protector.TenantID(tenant)
			if (firstErr == nil) != (secondErr == nil) || firstErr == nil && first != second {
				t.Fatalf("TenantID() is nondeterministic: %q/%v != %q/%v", first, firstErr, second, secondErr)
			}
		}
		if model == "" || len(tokenBytes) == 0 {
			return
		}
		if len(tokenBytes) > 1_024 {
			tokenBytes = tokenBytes[:1_024]
		}
		tokens := make([]uint32, len(tokenBytes))
		for i, value := range tokenBytes {
			tokens[i] = uint32(value)
		}
		first, firstErr := protector.PrefixFingerprint(model, tokens)
		second, secondErr := protector.PrefixFingerprint(model, tokens)
		if (firstErr == nil) != (secondErr == nil) || firstErr == nil && first != second {
			t.Fatalf("PrefixFingerprint() is nondeterministic: %q/%v != %q/%v", first, firstErr, second, secondErr)
		}
	})
}
