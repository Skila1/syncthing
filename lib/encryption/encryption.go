// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"
	"time"

	"golang.org/x/crypto/pbkdf2"
)

const (
	SaltSize       = 32
	KeySize        = 32 // AES-256
	PBKDF2Iter     = 100000
	NonceSize      = 12 // GCM nonce
	EscrowNonceSize = 12
)

// KeyRecord holds a user's encryption key material persisted in the DB.
type KeyRecord struct {
	UserID       int64  `json:"userId" db:"user_id"`
	EncryptedKey []byte `json:"-" db:"encrypted_key"`
	Salt         []byte `json:"-" db:"salt"`
	EscrowKey    []byte `json:"-" db:"escrow_key"`
	CreatedAt    int64  `json:"createdAt" db:"created_at"`
	UpdatedAt    int64  `json:"updatedAt" db:"updated_at"`
}

type Store interface {
	GetEncryptionKey(userID int64) (*KeyRecord, error)
	SetEncryptionKey(rec *KeyRecord) error
	DeleteEncryptionKey(userID int64) error
}

// DeriveKey derives an AES-256 key from a password and salt using PBKDF2.
func DeriveKey(password string, salt []byte) []byte {
	return pbkdf2.Key([]byte(password), salt, PBKDF2Iter, KeySize, sha256.New)
}

// GenerateDataKey creates a random 256-bit data encryption key.
func GenerateDataKey() ([]byte, error) {
	key := make([]byte, KeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	return key, nil
}

// GenerateSalt creates a random salt for key derivation.
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, SaltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	return salt, nil
}

// EncryptKey wraps a data key with a KEK derived from the user's password.
func EncryptKey(dataKey []byte, password string, salt []byte) ([]byte, error) {
	kek := DeriveKey(password, salt)
	return encryptAESGCM(kek, dataKey)
}

// DecryptKey unwraps a data key using the KEK derived from the user's password.
func DecryptKey(encryptedKey []byte, password string, salt []byte) ([]byte, error) {
	kek := DeriveKey(password, salt)
	return decryptAESGCM(kek, encryptedKey)
}

// CreateEscrowKey encrypts the data key with a separate admin recovery key
// so that an admin can recover user data if the password is lost.
func CreateEscrowKey(dataKey []byte, adminSecret string) ([]byte, error) {
	escrowKEK := sha256Hash([]byte(adminSecret))
	return encryptAESGCM(escrowKEK, dataKey)
}

// RecoverFromEscrow decrypts the data key using the admin secret.
func RecoverFromEscrow(escrowKey []byte, adminSecret string) ([]byte, error) {
	escrowKEK := sha256Hash([]byte(adminSecret))
	return decryptAESGCM(escrowKEK, escrowKey)
}

// EnrollUser generates a new data key, encrypts it with the user's password,
// and optionally creates an escrow copy.
func EnrollUser(userID int64, password, adminSecret string) (*KeyRecord, error) {
	dataKey, err := GenerateDataKey()
	if err != nil {
		return nil, err
	}
	salt, err := GenerateSalt()
	if err != nil {
		return nil, err
	}

	encKey, err := EncryptKey(dataKey, password, salt)
	if err != nil {
		return nil, err
	}

	var escrow []byte
	if adminSecret != "" {
		escrow, err = CreateEscrowKey(dataKey, adminSecret)
		if err != nil {
			return nil, err
		}
	}

	now := time.Now().UnixMilli()
	return &KeyRecord{
		UserID:       userID,
		EncryptedKey: encKey,
		Salt:         salt,
		EscrowKey:    escrow,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

// EncryptData encrypts plaintext with a data key using AES-256-GCM.
func EncryptData(dataKey, plaintext []byte) ([]byte, error) {
	return encryptAESGCM(dataKey, plaintext)
}

// DecryptData decrypts ciphertext with a data key using AES-256-GCM.
func DecryptData(dataKey, ciphertext []byte) ([]byte, error) {
	return decryptAESGCM(dataKey, ciphertext)
}

func encryptAESGCM(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func decryptAESGCM(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ct, nil)
}

func sha256Hash(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}
