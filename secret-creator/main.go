// secret-creator — standalone TOTP secret generator for Apig0.
// Generates a random base32 secret, saves a scannable QR code PNG,
// and prints the env var you need to export before running the gateway.
//
// Usage:
//   go run . [username]
//
// username defaults to "devin" if not provided.
package main

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"net/url"
	"os"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

func main() {
	user := "devin"
	if len(os.Args) > 1 {
		user = strings.TrimSpace(os.Args[1])
	}

	// Generate 20 random bytes → 32-char base32 secret (standard TOTP size)
	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		fmt.Fprintf(os.Stderr, "error generating secret: %v\n", err)
		os.Exit(1)
	}
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)

	// Build the otpauth URI
	otpauth := fmt.Sprintf(
		"otpauth://totp/Apig0:%s?secret=%s&issuer=Apig0&algorithm=SHA1&digits=6&period=30",
		url.PathEscape(user), secret,
	)

	// Save QR code as PNG
	outFile := user + "-totp-qr.png"
	if err := qrcode.WriteFile(otpauth, qrcode.Medium, 256, outFile); err != nil {
		fmt.Fprintf(os.Stderr, "error writing QR code: %v\n", err)
		os.Exit(1)
	}

	envVar := "APIG0_TOTP_SECRET_" + strings.ToUpper(strings.ReplaceAll(user, "-", "_"))

	fmt.Println()
	fmt.Println("========== TOTP SECRET CREATED ==========")
	fmt.Printf("  User   : %s\n", user)
	fmt.Printf("  Secret : %s\n", secret)
	fmt.Println()
	fmt.Println("  Set this env var before running Apig0:")
	fmt.Printf("  export %s=%s\n", envVar, secret)
	fmt.Println()
	fmt.Printf("  QR code saved to: %s\n", outFile)
	fmt.Println("  Scan it with Google Authenticator / Authy / any TOTP app.")
	fmt.Println("=========================================")
	fmt.Println()
}
