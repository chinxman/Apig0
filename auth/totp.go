package auth

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"apig0/config"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	qrcode "github.com/skip2/go-qrcode"
)

// ── Anti-replay cache ────────────────────────────────────────────────────────
// Tracks (user → set of already-used codes) so each code is accepted only once.
// Entries expire after 90 s — the full ±1-period skew window.

var (
	replayMu  sync.Mutex
	usedCodes = map[string]map[string]time.Time{} // user → code → expiry
)

func init() {
	// Sweep expired TOTP replay entries every 2 minutes.
	go func() {
		ticker := time.NewTicker(2 * time.Minute)
		for range ticker.C {
			sweepReplayCodes()
		}
	}()
}

func sweepReplayCodes() {
	now := time.Now()
	replayMu.Lock()
	defer replayMu.Unlock()
	for user, codes := range usedCodes {
		for c, exp := range codes {
			if now.After(exp) {
				delete(codes, c)
			}
		}
		if len(codes) == 0 {
			delete(usedCodes, user)
		}
	}
}

// ValidateTOTP validates a TOTP code for user/secret and rejects replays.
// It is the single source of truth for TOTP checking across the whole app.
func ValidateTOTP(user, code, secret string) bool {
	if !totp.Validate(code, secret) {
		return false
	}

	replayMu.Lock()
	defer replayMu.Unlock()

	now := time.Now()
	codes, ok := usedCodes[user]
	if !ok {
		codes = map[string]time.Time{}
		usedCodes[user] = codes
	}

	// Purge expired entries for this user
	for c, exp := range codes {
		if now.After(exp) {
			delete(codes, c)
		}
	}

	// Reject if this exact code was already accepted within the skew window
	if _, seen := codes[code]; seen {
		return false
	}

	// Mark as used — expire after 90 s (covers the full ±1-period window)
	codes[code] = now.Add(90 * time.Second)
	return true
}

// PrintQRIfEnabled prints the TOTP QR code only if APIG0_SHOW_QR is set to "true"
func PrintQRIfEnabled(user string) {
	if os.Getenv("APIG0_SHOW_QR") != "true" {
		return
	}
	PrintQR(user)
}

// GenerateQRDataURI generates a base64-encoded PNG data URI for a TOTP otpauth URL.
// Returns empty string on error.
func GenerateQRDataURI(otpauthURL string) string {
	png, err := qrcode.Encode(otpauthURL, qrcode.Medium, 256)
	if err != nil {
		log.Printf("[auth] QR generation failed: %v", err)
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
}

func PrintQR(user string) {
	secret := config.UserSecrets[user]

	key, err := otp.NewKeyFromURL(
		fmt.Sprintf("otpauth://totp/Apig0:%s?secret=%s&issuer=Apig0&algorithm=SHA1&digits=6&period=30",
			user, secret),
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("========== TOTP SETUP ==========")
	log.Printf("User: %s", user)
	log.Printf("Secret: %s", secret)
	if uri := GenerateQRDataURI(key.URL()); uri != "" {
		log.Printf("QR (data URI): %s", uri[:60]+"...")
	}
	log.Println("================================")
}

