package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/deriva-inc/keyper-go/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

func setupStatsTestRouter(t *testing.T) (*gin.Engine, *db.DB, sqlmock.Sqlmock) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	t.Cleanup(func() { _ = mockDB.Close() })

	sqlxDB := sqlx.NewDb(mockDB, "sqlmock")
	return gin.New(), db.NewFromDB(sqlxDB), mock
}

func TestComputeSecurityScore(t *testing.T) {
	if got := computeSecurityScore(entryTelemetryAgg{}); got.OverallScore != 100 {
		t.Errorf("expected 100 for empty vault, got %d", got.OverallScore)
	}

	got := computeSecurityScore(entryTelemetryAgg{
		TotalEntries:         10,
		EntriesWithURLCount:  8,
		EntriesGroupedCount:  10,
		ArchivedEntriesCount: 1,
		StaleEntriesCount:    2,
	})
	// 35*(0.8) + 35*(1.0) + 30*(0.8) = 28 + 35 + 24 = 87
	if got.OverallScore != 87 {
		t.Errorf("expected score 87, got %d", got.OverallScore)
	}
	if got.EntriesWithURLCount != 8 || got.EntriesGroupedCount != 10 || got.ArchivedEntriesCount != 1 || got.StaleEntriesCount != 2 {
		t.Errorf("unexpected telemetry counts: %+v", got)
	}
}

func TestGetDashboardOverview_UserWideSuccess(t *testing.T) {
	r, database, mock := setupStatsTestRouter(t)
	userID := uuid.New()
	profID := uuid.New()
	grpID := uuid.New()
	entID := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)

	r.GET("/api/v1/dashboard", func(c *gin.Context) {
		c.Set("userId", userID.String())
		GetDashboardOverview(database)(c)
	})

	mock.ExpectQuery("FROM vault_entries ve").
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{
			"total_entries", "total_favorites", "entries_with_url_count",
			"entries_grouped_count", "archived_entries_count", "stale_entries_count",
		}).AddRow(10, 4, 8, 10, 1, 2))

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM profiles WHERE user_id = \\$1").
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM groups g JOIN profiles p ON g.profile_id = p.id WHERE p.user_id = \\$1").
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(8))

	mock.ExpectQuery("SELECT p.id, p.name, p.icon FROM profiles p").
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "icon"}).AddRow(profID, "Personal", "user"))

	mock.ExpectQuery("SELECT g.id, g.name, g.icon FROM groups g").
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "icon"}).AddRow(grpID, "Cloud & DevOps", "aws"))

	mock.ExpectQuery("SELECT UPPER\\(ve.type::text\\) AS type, COUNT\\(\\*\\) AS count").
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"type", "count"}).
			AddRow("LOGIN", 7).
			AddRow("SECURE_NOTE", 3))

	mock.ExpectQuery("ve.id AS entry_id").
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"entry_id", "entry_name", "entry_type", "action", "timestamp"}).
			AddRow(entID, "GitHub Enterprise", "LOGIN", "UPDATED", now))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data struct {
			TotalProfiles         int                         `json:"totalProfiles"`
			TotalGroups           int                         `json:"totalGroups"`
			TotalEntries          int                         `json:"totalEntries"`
			TotalFavorites        int                         `json:"totalFavorites"`
			PopularProfile        *PopularItemSummary         `json:"popularProfile"`
			PopularGroup          *PopularItemSummary         `json:"popularGroup"`
			SecurityScore         SecurityScoreSummary        `json:"securityScore"`
			EntryTypeDistribution []EntryTypeDistributionItem `json:"entryTypeDistribution"`
			RecentActivity        []RecentActivityItem        `json:"recentActivity"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Data.TotalProfiles != 3 || resp.Data.TotalGroups != 8 || resp.Data.TotalEntries != 10 || resp.Data.TotalFavorites != 4 {
		t.Errorf("unexpected top-level counts: %+v", resp.Data)
	}
	if resp.Data.PopularProfile == nil || resp.Data.PopularProfile.Name != "Personal" {
		t.Errorf("unexpected popularProfile: %+v", resp.Data.PopularProfile)
	}
	if resp.Data.PopularGroup == nil || resp.Data.PopularGroup.Name != "Cloud & DevOps" {
		t.Errorf("unexpected popularGroup: %+v", resp.Data.PopularGroup)
	}
	if resp.Data.SecurityScore.OverallScore != 87 {
		t.Errorf("expected overallScore 87, got %d", resp.Data.SecurityScore.OverallScore)
	}
	if len(resp.Data.EntryTypeDistribution) != 2 || resp.Data.EntryTypeDistribution[0].Type != "LOGIN" {
		t.Errorf("unexpected entryTypeDistribution: %+v", resp.Data.EntryTypeDistribution)
	}
	if len(resp.Data.RecentActivity) != 1 || resp.Data.RecentActivity[0].Action != "UPDATED" {
		t.Errorf("unexpected recentActivity: %+v", resp.Data.RecentActivity)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

func TestGetDashboardOverview_ProfileScopedAndEmpty(t *testing.T) {
	r, database, mock := setupStatsTestRouter(t)
	userID := uuid.New()
	profileID := uuid.New()

	r.GET("/api/v1/dashboard", func(c *gin.Context) {
		c.Set("userId", userID.String())
		GetDashboardOverview(database)(c)
	})

	mock.ExpectQuery("FROM vault_entries ve").
		WithArgs(userID, profileID).
		WillReturnRows(sqlmock.NewRows([]string{
			"total_entries", "total_favorites", "entries_with_url_count",
			"entries_grouped_count", "archived_entries_count", "stale_entries_count",
		}).AddRow(0, 0, 0, 0, 0, 0))

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM profiles WHERE user_id = \\$1").
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery("WHERE p.user_id = \\$1 AND p.id = \\$2").
		WithArgs(userID, profileID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectQuery("SELECT p.id, p.name, p.icon FROM profiles p").
		WithArgs(userID).
		WillReturnError(sql.ErrNoRows)

	mock.ExpectQuery("SELECT g.id, g.name, g.icon FROM groups g").
		WithArgs(userID, profileID).
		WillReturnError(sql.ErrNoRows)

	mock.ExpectQuery("SELECT UPPER\\(ve.type::text\\) AS type, COUNT\\(\\*\\) AS count").
		WithArgs(userID, profileID).
		WillReturnRows(sqlmock.NewRows([]string{"type", "count"}))

	mock.ExpectQuery("ve.id AS entry_id").
		WithArgs(userID, profileID).
		WillReturnRows(sqlmock.NewRows([]string{"entry_id", "entry_name", "entry_type", "action", "timestamp"}))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	req.Header.Set("X-Profile-Id", profileID.String())
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

func TestGetDashboardChildEndpoints(t *testing.T) {
	r, database, mock := setupStatsTestRouter(t)
	userID := uuid.New()
	profID := uuid.New()
	entID := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)

	authStub := func(c *gin.Context) {
		c.Set("userId", userID.String())
		c.Next()
	}
	dash := r.Group("/api/v1/dashboard", authStub)
	dash.GET("/stats", GetDashboardStats(database))
	dash.GET("/security-score", GetSecurityScore(database))
	dash.GET("/distribution", GetEntryDistribution(database))
	dash.GET("/activity", GetRecentActivity(database))

	// 1. /stats
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM profiles WHERE user_id = \\$1").
		WithArgs(userID).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM groups g").
		WithArgs(userID).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))
	mock.ExpectQuery("FROM vault_entries ve").
		WithArgs(userID).WillReturnRows(sqlmock.NewRows([]string{
		"total_entries", "total_favorites", "entries_with_url_count",
		"entries_grouped_count", "archived_entries_count", "stale_entries_count",
	}).AddRow(12, 3, 10, 11, 0, 1))
	mock.ExpectQuery("SELECT p.id, p.name, p.icon FROM profiles p").
		WithArgs(userID).WillReturnRows(sqlmock.NewRows([]string{"id", "name", "icon"}).AddRow(profID, "Work", "briefcase"))
	mock.ExpectQuery("SELECT g.id, g.name, g.icon FROM groups g").
		WithArgs(userID).WillReturnError(sql.ErrNoRows)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/dashboard/stats", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("/stats expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// 2. /security-score
	mock.ExpectQuery("FROM vault_entries ve").
		WithArgs(userID).WillReturnRows(sqlmock.NewRows([]string{
		"total_entries", "total_favorites", "entries_with_url_count",
		"entries_grouped_count", "archived_entries_count", "stale_entries_count",
	}).AddRow(10, 2, 10, 10, 0, 0))

	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/api/v1/dashboard/security-score", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("/security-score expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// 3. /distribution
	mock.ExpectQuery("SELECT UPPER\\(ve.type::text\\) AS type, COUNT\\(\\*\\) AS count").
		WithArgs(userID).WillReturnRows(sqlmock.NewRows([]string{"type", "count"}).AddRow("LOGIN", 10))

	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/api/v1/dashboard/distribution", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("/distribution expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// 4. /activity
	mock.ExpectQuery("ve.id AS entry_id").
		WithArgs(userID).WillReturnRows(sqlmock.NewRows([]string{"entry_id", "entry_name", "entry_type", "action", "timestamp"}).
		AddRow(entID, "AWS", "LOGIN", "CREATED", now))

	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/api/v1/dashboard/activity", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("/activity expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

func TestGetDashboardOverview_InvalidProfileID(t *testing.T) {
	r, database, _ := setupStatsTestRouter(t)
	userID := uuid.New()

	r.GET("/api/v1/dashboard", func(c *gin.Context) {
		c.Set("userId", userID.String())
		GetDashboardOverview(database)(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	req.Header.Set("X-Profile-Id", "not-a-uuid")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d", w.Code)
	}
}
