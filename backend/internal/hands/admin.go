package hands

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"healthtrack/internal/database"
	"healthtrack/internal/models"
)

func adminStats(db *database.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		fail := func(msg string) { c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": msg}) }

		count := func(sql string) (int64, error) {
			row, err := db.QueryOne(ctx, sql)
			if err != nil {
				return 0, err
			}
			if row == nil {
				return 0, nil
			}
			return asInt64(row["count"]), nil
		}

		totalUsers, err := count("SELECT COUNT(*) as count FROM users")
		if err != nil {
			fail(dbErrorMessage(err))
			return
		}
		patients, err := count("SELECT COUNT(*) as count FROM users WHERE user_type = 'patient'")
		if err != nil {
			fail(dbErrorMessage(err))
			return
		}
		providers, err := count("SELECT COUNT(*) as count FROM users WHERE user_type = 'provider'")
		if err != nil {
			fail(dbErrorMessage(err))
			return
		}
		pending, err := count("SELECT COUNT(*) as count FROM users WHERE is_approved = 0 AND user_type IN ('provider','doctor')")
		if err != nil {
			fail(dbErrorMessage(err))
			return
		}
		totalRecords, err := count("SELECT COUNT(*) as count FROM health_records")
		if err != nil {
			fail(dbErrorMessage(err))
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"stats": gin.H{
				"total_users":       totalUsers,
				"patients":          patients,
				"providers":         providers,
				"pending_approvals": pending,
				"total_records":     totalRecords,
			},
		})
	}
}

func adminUsers(db *database.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		fail := func(msg string) { c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": msg}) }

		typ := c.Query("type")
		search := c.Query("search")
		if len(typ) > 64 || len(search) > 256 {
			fail("Query parameter too long")
			return
		}

		sql := "SELECT * FROM users"
		var args []any
		conditions := make([]string, 0, 2)
		n := 0
		if typ != "" {
			n++
			conditions = append(conditions, "user_type = $"+strconv.Itoa(n))
			args = append(args, typ)
		}
		if search != "" {
			like := "%" + search + "%"
			n++
			emailIdx := n
			n++
			usernameIdx := n
			conditions = append(conditions, "(email LIKE $"+strconv.Itoa(emailIdx)+" OR username LIKE $"+strconv.Itoa(usernameIdx)+")")
			args = append(args, like, like)
		}
		if len(conditions) > 0 {
			sql += " WHERE " + strings.Join(conditions, " AND ")
		}
		sql += " ORDER BY created_at DESC"

		users, err := db.QueryMany(ctx, sql, args...)
		if err != nil {
			fail(dbErrorMessage(err))
			return
		}

		userList := make([]gin.H, 0, len(users))
		for _, user := range users {
			userID := asInt64(user["id"])
			userType := stringOrEmpty(user["user_type"])

			displayRole := userType
			if userType == "provider" {
				providerRow, err := db.QueryOne(ctx, "SELECT * FROM service_providers WHERE user_id = $1", userID)
				if err != nil {
					fail(dbErrorMessage(err))
					return
				}
				if providerRow != nil {
					displayRole = stringOrEmpty(providerRow["provider_type"])
				}
			}

			isApproved := boolPtr(user["is_approved"])
			dateStr := dateOnly(user["date_joined"])

			userList = append(userList, gin.H{
				"id":          userID,
				"username":    stringOrEmpty(user["username"]),
				"email":       stringOrEmpty(user["email"]),
				"user_type":   displayRole,
				"is_approved": isApproved,
				"date_joined": dateStr,
			})
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "users": userList})
	}
}

func adminUserAction(db *database.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		adminUser := c.MustGet("user").(*models.User)

		fail := func(msg string) { c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": msg}) }

		var b struct {
			Action string `json:"action"`
		}
		_ = c.ShouldBindJSON(&b)

		userID := c.Param("user_id")

		user, err := db.QueryOne(ctx, "SELECT * FROM users WHERE id = $1", userID)
		if err != nil {
			fail(dbErrorMessage(err))
			return
		}
		if user == nil {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "User not found"})
			return
		}

		email := stringOrEmpty(user["email"])
		switch b.Action {
		case "approve":
			if _, err := db.Exec(ctx, "UPDATE users SET is_approved = 1 WHERE id = $1", userID); err != nil {
				fail(dbErrorMessage(err))
				return
			}
			if _, err := db.Exec(ctx,
				"INSERT INTO activity_logs (user_id, action, details) VALUES ($1, $2, $3)",
				adminUser.ID, "admin_action", "Approved user "+email); err != nil {
				fail(dbErrorMessage(err))
				return
			}
		case "reject":
			if _, err := db.Exec(ctx, "UPDATE users SET is_approved = 0 WHERE id = $1", userID); err != nil {
				fail(dbErrorMessage(err))
				return
			}
			if _, err := db.Exec(ctx,
				"INSERT INTO activity_logs (user_id, action, details) VALUES ($1, $2, $3)",
				adminUser.ID, "admin_action", "Rejected user "+email); err != nil {
				fail(dbErrorMessage(err))
				return
			}
		case "delete":
			if _, err := db.Exec(ctx, "DELETE FROM users WHERE id = $1", userID); err != nil {
				fail(dbErrorMessage(err))
				return
			}
			if _, err := db.Exec(ctx,
				"INSERT INTO activity_logs (user_id, action, details) VALUES ($1, $2, $3)",
				adminUser.ID, "admin_action", "Deleted user "+email); err != nil {
				fail(dbErrorMessage(err))
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}
