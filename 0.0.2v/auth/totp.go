package auth

import (
	"log"
	"net/url"
	"os"
	"sync"
	"time"

	"apig0/config"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// ── Anti-replay cache ────────────────────────────────────────────────────────
// Tracks (user → set of already-used codes) so each code is accepted only once.
// Entries expire after 90 s — the full ±1-period skew window.

var (
	replayMu  sync.Mutex
	usedCodes = map[string]map[string]time.Time{} // user → code → expiry
)

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

func PrintQR(user string) {

	secret := config.UserSecrets[user]

	key, err := otp.NewKeyFromURL(
		"otpauth://totp/Apig0:" + user +
			"?secret=" + secret +
			"&issuer=Apig0&algorithm=SHA1&digits=6&period=30",
	)

	if err != nil {
		log.Fatal(err)
	}

	qrLink := "https://api.qrserver.com/v1/create-qr-code/?size=250x250&data=" +
		url.QueryEscape(key.URL())

	log.Println("========== TOTP SETUP ==========")
	log.Printf("User: %s", user)
	log.Printf("Secret: %s", secret)
	log.Printf("Scan QR: %s", qrLink)
	log.Println("================================")
}

