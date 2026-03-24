package auth

import (
	"log"
	"net/http"
	"net/url"
	"os"

	"apig0/config"

	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

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

func Middleware() gin.HandlerFunc {

	return func(c *gin.Context) {

		// Skip TOTP auth for health check endpoint
		if c.Request.URL.Path == "/healthz" {
			c.Next()
			return
		}

		code := c.GetHeader("X-TOTP")
		if code == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "X-TOTP header required",
			})
			return
		}

		user := "devin"
		secret := config.UserSecrets[user]

		valid := totp.Validate(code, secret)

		if !valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid TOTP code",
			})
			return
		}

		c.Next()
	}
}
