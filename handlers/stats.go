package handlers

import (
	"database/sql"
	"errors"
	"math"
	"net/http"
	"time"

	"github.com/deriva-inc/keyper-go/db"
	"github.com/deriva-inc/keyper-go/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// PopularItemSummary represents a lightweight summary for popular profile or group.
type PopularItemSummary struct {
	ID   uuid.UUID `db:"id" json:"id"`
	Name string    `db:"name" json:"name"`
	Icon *string   `db:"icon" json:"icon,omitempty"`
}

// SecurityScoreSummary represents non-sensitive metadata completeness and staleness telemetry.
type SecurityScoreSummary struct {
	OverallScore         int `json:"overallScore"`
	EntriesWithURLCount  int `json:"entriesWithUrlCount"`
	EntriesGroupedCount  int `json:"entriesGroupedCount"`
	ArchivedEntriesCount int `json:"archivedEntriesCount"`
	StaleEntriesCount    int `json:"staleEntriesCount"`
}

// EntryTypeDistributionItem represents the count of vault entries per entry type.
type EntryTypeDistributionItem struct {
	Type  string `db:"type" json:"type"`
	Count int    `db:"count" json:"count"`
}

// RecentActivityItem represents a recent vault entry activity event.
type RecentActivityItem struct {
	EntryID   uuid.UUID `db:"entry_id" json:"entryId"`
	EntryName string    `db:"entry_name" json:"entryName"`
	EntryType string    `db:"entry_type" json:"entryType"`
	Action    string    `db:"action" json:"action"`
	Timestamp time.Time `db:"timestamp" json:"timestamp"`
}

// DashboardStatsSummary holds the core KPI counts and popular entities.
type DashboardStatsSummary struct {
	TotalProfiles  int                 `json:"totalProfiles"`
	TotalGroups    int                 `json:"totalGroups"`
	TotalEntries   int                 `json:"totalEntries"`
	TotalFavorites int                 `json:"totalFavorites"`
	PopularProfile *PopularItemSummary `json:"popularProfile"`
	PopularGroup   *PopularItemSummary `json:"popularGroup"`
}

type entryTelemetryAgg struct {
	TotalEntries         int `db:"total_entries"`
	TotalFavorites       int `db:"total_favorites"`
	EntriesWithURLCount  int `db:"entries_with_url_count"`
	EntriesGroupedCount  int `db:"entries_grouped_count"`
	ArchivedEntriesCount int `db:"archived_entries_count"`
	StaleEntriesCount    int `db:"stale_entries_count"`
}

type dashboardScope struct {
	userID      uuid.UUID
	profileID   *uuid.UUID
	whereClause string
	args        []interface{}
}

// parseDashboardScope extracts authenticated userID and optional X-Profile-Id filter.
func parseDashboardScope(c *gin.Context) (*dashboardScope, bool) {
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return nil, false
	}

	var profileID *uuid.UUID
	if profileIDHeader := c.GetHeader("X-Profile-Id"); profileIDHeader != "" {
		parsedID, parseErr := uuid.Parse(profileIDHeader)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid X-Profile-Id header format"})
			return nil, false
		}
		profileID = &parsedID
	}

	whereClause, args := buildProfileFilter(userID, profileID)
	return &dashboardScope{
		userID:      userID,
		profileID:   profileID,
		whereClause: whereClause,
		args:        args,
	}, true
}

// buildProfileFilter builds a parameterized SQL condition and argument slice scoped to userId (and optional profileId).
func buildProfileFilter(userID uuid.UUID, profileID *uuid.UUID) (string, []interface{}) {
	if profileID != nil {
		return "p.user_id = $1 AND p.id = $2", []interface{}{userID, *profileID}
	}
	return "p.user_id = $1", []interface{}{userID}
}

// computeSecurityScore calculates a 0-100 metadata hygiene score without inspecting plaintext secrets.
func computeSecurityScore(agg entryTelemetryAgg) SecurityScoreSummary {
	overall := 100
	if agg.TotalEntries > 0 {
		total := float64(agg.TotalEntries)
		urlRatio := float64(agg.EntriesWithURLCount) / total
		groupedRatio := float64(agg.EntriesGroupedCount) / total
		freshRatio := float64(agg.TotalEntries-agg.StaleEntriesCount) / total
		score := int(math.Round(35*urlRatio + 35*groupedRatio + 30*freshRatio))
		if score < 0 {
			score = 0
		} else if score > 100 {
			score = 100
		}
		overall = score
	}

	return SecurityScoreSummary{
		OverallScore:         overall,
		EntriesWithURLCount:  agg.EntriesWithURLCount,
		EntriesGroupedCount:  agg.EntriesGroupedCount,
		ArchivedEntriesCount: agg.ArchivedEntriesCount,
		StaleEntriesCount:    agg.StaleEntriesCount,
	}
}

func fetchEntryTelemetry(database *db.DB, scope *dashboardScope) (entryTelemetryAgg, error) {
	var agg entryTelemetryAgg
	query := `
		SELECT
			COUNT(*) AS total_entries,
			COUNT(*) FILTER (WHERE ve.is_favorite = TRUE) AS total_favorites,
			COUNT(*) FILTER (WHERE ve.website_url IS NOT NULL AND TRIM(ve.website_url) <> '') AS entries_with_url_count,
			COUNT(*) FILTER (WHERE ve.group_id IS NOT NULL) AS entries_grouped_count,
			COUNT(*) FILTER (WHERE ve.is_archived = TRUE) AS archived_entries_count,
			COUNT(*) FILTER (WHERE ve.updated_at < NOW() - INTERVAL '90 days') AS stale_entries_count
		FROM vault_entries ve
		JOIN profiles p ON ve.profile_id = p.id
		WHERE ` + scope.whereClause
	err := database.Get(&agg, query, scope.args...)
	return agg, err
}

func fetchCoreStats(database *db.DB, scope *dashboardScope, agg *entryTelemetryAgg) (*DashboardStatsSummary, error) {
	var profilesCount int
	if err := database.Get(&profilesCount, `SELECT COUNT(*) FROM profiles WHERE user_id = $1`, scope.userID); err != nil {
		return nil, err
	}

	var groupsCount int
	groupsQuery := `SELECT COUNT(*) FROM groups g JOIN profiles p ON g.profile_id = p.id WHERE ` + scope.whereClause
	if err := database.Get(&groupsCount, groupsQuery, scope.args...); err != nil {
		return nil, err
	}

	if agg == nil {
		fetched, err := fetchEntryTelemetry(database, scope)
		if err != nil {
			return nil, err
		}
		agg = &fetched
	}

	var popularProfile *PopularItemSummary
	var prof PopularItemSummary
	popProfileQuery := `
		SELECT p.id, p.name, p.icon FROM profiles p
		WHERE p.user_id = $1
		ORDER BY (SELECT COUNT(*) FROM vault_entries WHERE profile_id = p.id) DESC, p.created_at ASC
		LIMIT 1`
	if err := database.Get(&prof, popProfileQuery, scope.userID); err == nil {
		popularProfile = &prof
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	var popularGroup *PopularItemSummary
	var grp PopularItemSummary
	popGroupQuery := `
		SELECT g.id, g.name, g.icon FROM groups g
		JOIN profiles p ON g.profile_id = p.id
		WHERE ` + scope.whereClause + `
		ORDER BY (SELECT COUNT(*) FROM vault_entries WHERE group_id = g.id) DESC, g.created_at ASC
		LIMIT 1`
	if err := database.Get(&grp, popGroupQuery, scope.args...); err == nil {
		popularGroup = &grp
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	return &DashboardStatsSummary{
		TotalProfiles:  profilesCount,
		TotalGroups:    groupsCount,
		TotalEntries:   agg.TotalEntries,
		TotalFavorites: agg.TotalFavorites,
		PopularProfile: popularProfile,
		PopularGroup:   popularGroup,
	}, nil
}

func fetchEntryDistribution(database *db.DB, scope *dashboardScope) ([]EntryTypeDistributionItem, error) {
	var distribution []EntryTypeDistributionItem
	query := `
		SELECT UPPER(ve.type::text) AS type, COUNT(*) AS count
		FROM vault_entries ve
		JOIN profiles p ON ve.profile_id = p.id
		WHERE ` + scope.whereClause + `
		GROUP BY ve.type
		ORDER BY count DESC`
	if err := database.Select(&distribution, query, scope.args...); err != nil {
		return nil, err
	}
	if distribution == nil {
		distribution = []EntryTypeDistributionItem{}
	}
	return distribution, nil
}

func fetchRecentActivity(database *db.DB, scope *dashboardScope) ([]RecentActivityItem, error) {
	var recentActivity []RecentActivityItem
	query := `
		SELECT
			ve.id AS entry_id,
			ve.name AS entry_name,
			UPPER(ve.type::text) AS entry_type,
			CASE
				WHEN ve.is_archived THEN 'ARCHIVED'
				WHEN ve.updated_at > ve.created_at + INTERVAL '1 second' THEN 'UPDATED'
				ELSE 'CREATED'
			END AS action,
			ve.updated_at AS timestamp
		FROM vault_entries ve
		JOIN profiles p ON ve.profile_id = p.id
		WHERE ` + scope.whereClause + `
		ORDER BY ve.updated_at DESC
		LIMIT 10`
	if err := database.Select(&recentActivity, query, scope.args...); err != nil {
		return nil, err
	}
	if recentActivity == nil {
		recentActivity = []RecentActivityItem{}
	}
	return recentActivity, nil
}

// GetDashboardOverview handles GET /api/v1/dashboard and returns all dashboard sections in one payload.
func GetDashboardOverview(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		scope, ok := parseDashboardScope(c)
		if !ok {
			return
		}

		agg, err := fetchEntryTelemetry(database, scope)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not retrieve entry telemetry: " + err.Error()})
			return
		}

		stats, err := fetchCoreStats(database, scope, &agg)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not retrieve dashboard stats: " + err.Error()})
			return
		}

		distribution, err := fetchEntryDistribution(database, scope)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not retrieve entry type distribution: " + err.Error()})
			return
		}

		recentActivity, err := fetchRecentActivity(database, scope)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not retrieve recent activity: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Dashboard stats retrieved successfully",
			"data": gin.H{
				"totalProfiles":         stats.TotalProfiles,
				"totalGroups":           stats.TotalGroups,
				"totalEntries":          stats.TotalEntries,
				"totalFavorites":        stats.TotalFavorites,
				"popularProfile":        stats.PopularProfile,
				"popularGroup":          stats.PopularGroup,
				"securityScore":         computeSecurityScore(agg),
				"entryTypeDistribution": distribution,
				"recentActivity":        recentActivity,
			},
		})
	}
}

// GetDashboardStats handles GET /api/v1/dashboard/stats for KPI counts and popular profile/group.
func GetDashboardStats(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		scope, ok := parseDashboardScope(c)
		if !ok {
			return
		}

		stats, err := fetchCoreStats(database, scope, nil)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not retrieve dashboard stats: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Dashboard stats retrieved successfully",
			"data":    stats,
		})
	}
}

// GetSecurityScore handles GET /api/v1/dashboard/security-score for zero-knowledge metadata telemetry.
func GetSecurityScore(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		scope, ok := parseDashboardScope(c)
		if !ok {
			return
		}

		agg, err := fetchEntryTelemetry(database, scope)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not retrieve security score: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Security score retrieved successfully",
			"data":    computeSecurityScore(agg),
		})
	}
}

// GetEntryDistribution handles GET /api/v1/dashboard/distribution for entry counts grouped by type.
func GetEntryDistribution(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		scope, ok := parseDashboardScope(c)
		if !ok {
			return
		}

		distribution, err := fetchEntryDistribution(database, scope)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not retrieve entry type distribution: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Entry type distribution retrieved successfully",
			"data":    distribution,
		})
	}
}

// GetRecentActivity handles GET /api/v1/dashboard/activity for the 10 most recent vault entry events.
func GetRecentActivity(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		scope, ok := parseDashboardScope(c)
		if !ok {
			return
		}

		recentActivity, err := fetchRecentActivity(database, scope)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not retrieve recent activity: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Recent activity retrieved successfully",
			"data":    recentActivity,
		})
	}
}
