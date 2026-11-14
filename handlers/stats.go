package handlers

import (
	"log"
	"net/http"

	"github.com/deriva-inc/keyper-go/db"
	"github.com/deriva-inc/keyper-go/middleware"
	"github.com/deriva-inc/keyper-go/models"
	"github.com/gin-gonic/gin"
)

// GET [/api/v1/stats/dashboard] - retrieves all dashboard stats.
func GetDashboardStats(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userId, err := middleware.GetUserIDFromContext(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		profileId := c.GetHeader("X-Profile-Id")

		if profileId == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "X-Profile-Id headers are required"})
			return
		}

		var profilesCount, groupsCount, entriesCount int

		query := `SELECT COUNT(*) FROM vault_entries`
		countErr := database.Get(&entriesCount, query)
		if countErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not retrieve entries count: " + countErr.Error()})
			return
		}

		query = `SELECT COUNT(*) FROM groups`
		countErr = database.Get(&groupsCount, query)
		if countErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not retrieve groups count: " + countErr.Error()})
			return
		}

		query = `SELECT COUNT(*) FROM profiles`
		countErr = database.Get(&profilesCount, query)
		if countErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not retrieve profiles count: " + countErr.Error()})
			return
		}

		// Most popular profile (has most vault entries)
		var popularProfile models.Profile
		query = `
			SELECT p.* FROM profiles p
			WHERE p.user_id = $1
			ORDER BY (SELECT COUNT(*) FROM vault_entries WHERE profile_id = p.id) DESC
			LIMIT 1
		`
		if err := database.Get(&popularProfile, query, userId); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No profile found for the user: " + err.Error()})
			log.Printf("No profiles found for user %s", userId)
		}

		// Most popular group (has most vault entries)
		var popularGroup models.Group
		query = `
			SELECT g.* FROM groups g
			ORDER BY (SELECT COUNT(*) FROM vault_entries WHERE group_id = g.id) DESC
			LIMIT 1
		`
		if err := database.Get(&popularGroup, query); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No groups found for the profile: " + err.Error()})
			log.Printf("No groups found for profile %s", profileId)
		}

		c.JSON(http.StatusOK, gin.H{"message": "Entries retrieved successfully", "data": gin.H{
			"totalProfiles":  profilesCount,
			"totalGroups":    groupsCount,
			"totalEntries":   entriesCount,
			"popularProfile": popularProfile,
			"popularGroup":   popularGroup,
		}})
	}
}
