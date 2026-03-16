// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package mfa

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"time"
)

const (
	SecretLength = 20 // 160-bit secret
	CodeDigits   = 6
	TimeStep     = 30 // seconds
	Skew         = 1  // allow codes from +/- 1 time step
	Issuer       = "Syncthing"
)

// GenerateSecret creates a new random TOTP secret, returned as a
// base32-encoded string (without padding).
func GenerateSecret() (string, error) {
	secret := make([]byte, SecretLength)
	if _, err := rand.Read(secret); err != nil {
		return "", fmt.Errorf("generate secret: %w", err)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret), nil
}

// GenerateCode computes the TOTP code for the given secret and time.
func GenerateCode(secret string, t time.Time) (string, error) {
	key, err := decodeSecret(secret)
	if err != nil {
		return "", err
	}

	counter := uint64(t.Unix()) / TimeStep
	code := hotp(key, counter)
	return fmt.Sprintf("%0*d", CodeDigits, code), nil
}

// ValidateCode checks whether the provided code is valid for the given
// secret at the current time, accounting for clock skew.
func ValidateCode(secret, code string) (bool, error) {
	key, err := decodeSecret(secret)
	if err != nil {
		return false, err
	}

	now := time.Now().Unix()
	baseCounter := uint64(now) / TimeStep

	for i := -Skew; i <= Skew; i++ {
		counter := baseCounter + uint64(i)
		expected := fmt.Sprintf("%0*d", CodeDigits, hotp(key, counter))
		if hmac.Equal([]byte(expected), []byte(code)) {
			return true, nil
		}
	}
	return false, nil
}

// ProvisioningURI generates the otpauth:// URI for QR code generation.
func ProvisioningURI(secret, username string) string {
	label := fmt.Sprintf("%s:%s", Issuer, username)
	return fmt.Sprintf("otpauth://totp/%s?secret=%s&issuer=%s&digits=%d&period=%d",
		label, secret, Issuer, CodeDigits, TimeStep)
}

func hotp(key []byte, counter uint64) int {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)

	mac := hmac.New(sha1.New, key)
	mac.Write(buf)
	hash := mac.Sum(nil)

	offset := hash[len(hash)-1] & 0x0f
	truncated := binary.BigEndian.Uint32(hash[offset:offset+4]) & 0x7fffffff
	return int(truncated) % int(math.Pow10(CodeDigits))
}

func decodeSecret(secret string) ([]byte, error) {
	secret = strings.ToUpper(strings.TrimRight(secret, "="))
	// Re-pad for base32
	if m := len(secret) % 8; m != 0 {
		secret += strings.Repeat("=", 8-m)
	}
	return base32.StdEncoding.DecodeString(secret)
}
