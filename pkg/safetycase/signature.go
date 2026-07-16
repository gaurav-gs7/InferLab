package safetycase

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
)

const signatureDomain = "InferLab safety case signature v1\x00"

func GenerateKeyPair() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate Ed25519 key: %w", err)
	}
	return publicKey, privateKey, nil
}

func MarshalPrivateKeyPEM(key ed25519.PrivateKey) ([]byte, error) {
	if len(key) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("%w: invalid Ed25519 private-key length", ErrInvalidSignature)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("marshal private key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

func MarshalPublicKeyPEM(key ed25519.PublicKey) ([]byte, error) {
	if len(key) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("%w: invalid Ed25519 public-key length", ErrInvalidSignature)
	}
	der, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return nil, fmt.Errorf("marshal public key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), nil
}

func ParsePrivateKeyPEM(data []byte) (ed25519.PrivateKey, error) {
	block, rest := pem.Decode(data)
	if block == nil || block.Type != "PRIVATE KEY" || len(rest) != 0 {
		return nil, fmt.Errorf("%w: expected one PKCS#8 PRIVATE KEY", ErrInvalidSignature)
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%w: parse private key: %v", ErrInvalidSignature, err)
	}
	key, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("%w: private key is not Ed25519", ErrInvalidSignature)
	}
	return key, nil
}

func ParsePublicKeyPEM(data []byte) (ed25519.PublicKey, error) {
	block, rest := pem.Decode(data)
	if block == nil || block.Type != "PUBLIC KEY" || len(rest) != 0 {
		return nil, fmt.Errorf("%w: expected one PKIX PUBLIC KEY", ErrInvalidSignature)
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%w: parse public key: %v", ErrInvalidSignature, err)
	}
	key, ok := parsed.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("%w: public key is not Ed25519", ErrInvalidSignature)
	}
	return key, nil
}

func KeyID(publicKey ed25519.PublicKey) (string, error) {
	if len(publicKey) != ed25519.PublicKeySize {
		return "", fmt.Errorf("%w: invalid Ed25519 public-key length", ErrInvalidSignature)
	}
	der, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", fmt.Errorf("marshal public key identity: %w", err)
	}
	digest := sha256.Sum256(der)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

func Sign(manifest Manifest, privateKey ed25519.PrivateKey) (Signature, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return Signature{}, fmt.Errorf("%w: invalid Ed25519 private-key length", ErrInvalidSignature)
	}
	canonical, err := CanonicalManifestJSON(manifest)
	if err != nil {
		return Signature{}, err
	}
	publicKey := privateKey.Public().(ed25519.PublicKey)
	keyID, err := KeyID(publicKey)
	if err != nil {
		return Signature{}, err
	}
	digest, _ := ManifestDigest(manifest)
	value := ed25519.Sign(privateKey, append([]byte(signatureDomain), canonical...))
	return Signature{
		Schema: SignatureSchema, SchemaVersion: CurrentSchemaVersion, Algorithm: "ed25519",
		ManifestDigest: digest, KeyID: keyID, Value: base64.RawStdEncoding.EncodeToString(value),
	}, nil
}

func VerifySignature(manifest Manifest, signature Signature, publicKey ed25519.PublicKey) error {
	if len(publicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("%w: invalid Ed25519 public-key length", ErrInvalidSignature)
	}
	if err := validateSignature(signature); err != nil {
		return err
	}
	canonical, err := CanonicalManifestJSON(manifest)
	if err != nil {
		return err
	}
	digest, _ := ManifestDigest(manifest)
	keyID, err := KeyID(publicKey)
	if err != nil {
		return err
	}
	if signature.ManifestDigest != digest || signature.KeyID != keyID {
		return fmt.Errorf("%w: manifest digest or key identity differs", ErrInvalidSignature)
	}
	value, err := base64.RawStdEncoding.DecodeString(signature.Value)
	if err != nil || len(value) != ed25519.SignatureSize || !ed25519.Verify(publicKey, append([]byte(signatureDomain), canonical...), value) {
		return fmt.Errorf("%w: Ed25519 verification failed", ErrInvalidSignature)
	}
	return nil
}
