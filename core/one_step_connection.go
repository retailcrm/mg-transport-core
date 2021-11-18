package core

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	retailcrm "github.com/retailcrm/api-client-go/v2"
)

// ConnectionConfig returns middleware for the one-step connection configuration route.
func ConnectionConfig(registerURL string, scopes []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.AbortWithStatusJSON(http.StatusOK, retailcrm.ConnectionConfigResponse{
			SuccessfulResponse: retailcrm.SuccessfulResponse{Success: true},
			Scopes:             scopes,
			RegisterURL:        registerURL,
		})
	}
}

// VerifyConnectRequest will verify ConnectRequest and place it into the "request" context field.
func VerifyConnectRequest(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		connectReqData := c.PostForm("register")
		if connectReqData == "" {
			c.AbortWithStatusJSON(http.StatusOK, retailcrm.ErrorResponse{ErrorMessage: "No data provided"})
			return
		}

		var r retailcrm.ConnectRequest
		err := json.Unmarshal([]byte(connectReqData), &r)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusOK, retailcrm.ErrorResponse{ErrorMessage: "Invalid JSON provided"})
			return
		}

		if !r.Verify(secret) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{})
			return
		}

		c.Set("request", r)
	}
}

// MustGetConnectRequest will extract retailcrm.ConnectRequest from the request context.
func MustGetConnectRequest(c *gin.Context) retailcrm.ConnectRequest {
	return c.MustGet("request").(retailcrm.ConnectRequest)
}
