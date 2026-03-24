package middleware

import "github.com/gin-gonic/gin"

// Cors returns a Gin middleware that sets CORS headers and handles OPTIONS pre‑flight requests.
func Cors() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
        c.Writer.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
        c.Writer.Header().Set("Access-Control-Allow-Headers", "X-TOTP,Content-Type")
        if c.Request.Method == "OPTIONS" {
            c.AbortWithStatus(204)
            return
        }
        c.Next()
    }
}
