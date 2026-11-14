package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/deriva-inc/keyper-go/db"
	"github.com/deriva-inc/keyper-go/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GenerateRefreshToken generates a secure random 32-byte hex string (64 characters).
func GenerateRefreshToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random refresh token: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// RefreshTokenReq defines request payload for /api/v1/auth/refresh
type RefreshTokenReq struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

// LogoutReq defines request payload for /api/v1/auth/logout
type LogoutReq struct {
	RefreshToken string `json:"refreshToken"`
}

// RefreshToken handles extending an active session and rotating the refresh token.
// POST /api/v1/auth/refresh
func RefreshToken(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input RefreshTokenReq
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input: refreshToken is required"})
			return
		}

		// Find existing session by refresh token that has not expired
		var session models.UserSession
		query := `
			SELECT id, user_id, refresh_token, expiry_time, browser, os, manufacturer, creation_time, last_active
			FROM user_sessions
			WHERE refresh_token = $1`
		err := database.Get(&session, query, input.RefreshToken)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
			return
		}

		// Check if session has expired
		if time.Now().After(session.ExpiryTime) {
			// Clean up expired session
			_, _ = database.Exec("DELETE FROM user_sessions WHERE id = $1", session.ID)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Session expired, please log in again"})
			return
		}

		// Generate a new 24-hour access token
		accessToken, err := GenerateJWT(session.UserID, 24*time.Hour)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate access token: " + err.Error()})
			return
		}

		// Rotate refresh token for security and update last_active timestamp
		newRefreshToken, err := GenerateRefreshToken()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to rotate refresh token: " + err.Error()})
			return
		}

		updateQuery := `
			UPDATE user_sessions
			SET refresh_token = $1, last_active = NOW()
			WHERE id = $2`
		_, err = database.Exec(updateQuery, newRefreshToken, session.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update session: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Token refreshed successfully",
			"data": gin.H{
				"accessToken":  accessToken,
				"refreshToken": newRefreshToken,
				"expiryTime":   session.ExpiryTime,
			},
		})
	}
}

// Logout invalidates a user's session by removing the refresh token.
// POST /api/v1/auth/logout
func Logout(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input LogoutReq
		_ = c.ShouldBindJSON(&input)

		if input.RefreshToken != "" {
			_, _ = database.Exec("DELETE FROM user_sessions WHERE refresh_token = $1", input.RefreshToken)
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Logged out successfully",
		})
	}
}

// GetUserSessions lists all active sessions for the authenticated user.
// GET /api/v1/users/sessions
func GetUserSessions(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDVal, exists := c.Get("userID")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		userID, ok := userIDVal.(uuid.UUID)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user ID"})
			return
		}

		var sessions []models.UserSession
		query := `
			SELECT id, user_id, refresh_token, expiry_time, browser, os, manufacturer, creation_time, last_active
			FROM user_sessions
			WHERE user_id = $1 AND expiry_time > NOW()
			ORDER BY last_active DESC`
		err := database.Select(&sessions, query, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user sessions: " + err.Error()})
			return
		}

		if sessions == nil {
			sessions = []models.UserSession{}
		}

		c.JSON(http.StatusOK, gin.H{
			"data": sessions,
		})
	}
}

// RevokeAllSessions revokes all sessions for the authenticated user ("log out from everywhere").
// POST /api/v1/auth/logout-all or DELETE /api/v1/users/sessions
func RevokeAllSessions(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDVal, exists := c.Get("userID")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		userID, ok := userIDVal.(uuid.UUID)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user ID"})
			return
		}

		_, err := database.Exec("DELETE FROM user_sessions WHERE user_id = $1", userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke sessions: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Logged out from all devices successfully",
		})
	}
}

// RevokeSession revokes a specific session belonging to the authenticated user.
// DELETE /api/v1/users/sessions/:sessionId
func RevokeSession(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDVal, exists := c.Get("userID")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		userID, ok := userIDVal.(uuid.UUID)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user ID"})
			return
		}

		sessionIDStr := c.Param("sessionId")
		sessionID, err := uuid.Parse(sessionIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session ID"})
			return
		}

		res, err := database.Exec("DELETE FROM user_sessions WHERE id = $1 AND user_id = $2", sessionID, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke session: " + err.Error()})
			return
		}

		rows, _ := res.RowsAffected()
		if rows == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Session revoked successfully",
		})
	}
}
