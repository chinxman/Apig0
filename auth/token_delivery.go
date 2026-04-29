package auth

import (
	"net/http"
	"strings"

	"apig0/config"

	"github.com/gin-gonic/gin"
)

func pendingDeliveryResponse(delivery config.PendingAPITokenDelivery) gin.H {
	backendLabel := delivery.Service
	if delivery.Service != "" {
		if service, ok := config.GetServiceConfig(delivery.Service); ok {
			switch {
			case strings.TrimSpace(service.Provider) != "" && service.Provider != service.Name:
				backendLabel = service.Provider + " • " + service.Name
			case strings.TrimSpace(service.Provider) != "":
				backendLabel = service.Provider
			default:
				backendLabel = service.Name
			}
		}
	}
	return gin.H{
		"id":            delivery.ID,
		"token_id":      delivery.TokenID,
		"token_prefix":  delivery.TokenPrefix,
		"key_type":      delivery.KeyType,
		"service":       delivery.Service,
		"backend_label": backendLabel,
		"created_by":    delivery.CreatedBy,
		"created_at":    delivery.CreatedAt,
		"expires_at":    delivery.ExpiresAt,
	}
}

func ListPendingTokenDeliveriesHandler(c *gin.Context) {
	username := c.GetString("session_user")
	deliveries := config.ListPendingAPITokenDeliveriesForUser(username)
	payload := make([]gin.H, 0, len(deliveries))
	for _, delivery := range deliveries {
		payload = append(payload, pendingDeliveryResponse(delivery))
	}
	c.JSON(http.StatusOK, gin.H{
		"deliveries": payload,
	})
}

func ClaimPendingTokenDeliveryHandler(c *gin.Context) {
	username := c.GetString("session_user")
	raw, delivery, err := config.ClaimPendingAPITokenDelivery(username, c.Param("id"))
	if err != nil {
		status := http.StatusNotFound
		if strings.Contains(strings.ToLower(err.Error()), "active") {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":        true,
		"raw_token": raw,
		"delivery":  pendingDeliveryResponse(delivery),
	})
}
