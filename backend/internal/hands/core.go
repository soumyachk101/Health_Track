package hands

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"healthtrack/internal/config"
	"healthtrack/internal/database"
	"healthtrack/internal/models"
)

// asStr safely pulls a string value out of a pgx row map.
func asStr(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// asInt safely pulls an integer out of a pgx row map (Postgres INTEGER comes
// back as int32 by default).
func asInt(v any) int {
	switch n := v.(type) {
	case int32:
		return int(n)
	case int64:
		return int(n)
	case int:
		return n
	}
	return 0
}

// asInt64 safely pulls an int64 out of a pgx row map.
func asInt64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int32:
		return int64(n)
	case int:
		return int64(n)
	}
	return 0
}

// numPtr returns a pointer to the numeric value, or nil for SQL NULL.
func numPtr(v any) *float64 {
	switch n := v.(type) {
	case float64:
		return &n
	case float32:
		f := float64(n)
		return &f
	case int32:
		f := float64(n)
		return &f
	case int64:
		f := float64(n)
		return &f
	}
	return nil
}

// strPtr returns a pointer to the string value, or nil for SQL NULL.
func strPtr(v any) *string {
	if s, ok := v.(string); ok {
		return &s
	}
	return nil
}

// int32Ptr returns a pointer to the integer value, or nil for SQL NULL.
func int32Ptr(v any) *int32 {
	switch n := v.(type) {
	case int32:
		return &n
	case int64:
		s := int32(n)
		return &s
	case int:
		s := int32(n)
		return &s
	}
	return nil
}

// boolPtr coerces a pgx integer/string flag to a Go bool pointer (nil when NULL).
func boolPtr(v any) *bool {
	switch n := v.(type) {
	case bool:
		return &n
	case int32:
		b := n != 0
		return &b
	case int64:
		b := n != 0
		return &b
	}
	return nil
}

// dateOnly returns the YYYY-MM-DD part of a timestamp/DATE value.
func dateOnly(v any) string {
	switch n := v.(type) {
	case string:
		if len(n) >= 10 {
			return n[:10]
		}
		return n
	case time.Time:
		return n.UTC().Format("2006-01-02")
	}
	return ""
}

// stringOrEmpty returns the string value or "" when NULL.
func stringOrEmpty(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// jsStringNum mirrors the legacy JS `x ? String(x) : null` used for
// blood_sugar / weight / oxygen_level / sleep_hours display fields: a SQL NULL
// or a zero value serializes to JSON null, otherwise to the JS-style string.
func jsStringNum(v any) any {
	var f float64
	switch n := v.(type) {
	case float32:
		f = float64(n)
	case float64:
		f = n
	case int32:
		f = float64(n)
	case int64:
		f = float64(n)
	default:
		return nil
	}
	if f == 0 {
		return nil
	}
	// JS String(number) -> no trailing ".0", no scientific notation in the
	// ranges we deal with; strconv with prec -1 matches that.
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// bpStatus mirrors the legacy JS getBpStatus helper.
func bpStatus(sys, dia int32) string {
	if sys == 0 || dia == 0 {
		return "Unknown"
	}
	if sys < 120 && dia < 80 {
		return "Normal"
	}
	if sys < 130 && dia < 80 {
		return "Elevated"
	}
	if sys < 140 || dia < 90 {
		return "High (Stage 1)"
	}
	return "High (Stage 2)"
}

// moodDisplay mirrors the legacy JS moodDisplay helper.
func moodDisplay(score int) string {
	switch score {
	case 1:
		return "Very Low"
	case 2:
		return "Low"
	case 3:
		return "Neutral"
	case 4:
		return "Good"
	case 5:
		return "Excellent"
	default:
		return "Unknown"
	}
}

// frequencyDisplay mirrors the legacy JS frequencyDisplay helper.
func frequencyDisplay(freq string) string {
	switch freq {
	case "once":
		return "Once Daily"
	case "twice":
		return "Twice Daily"
	case "thrice":
		return "Three Times Daily"
	case "asneeded":
		return "As Needed"
	default:
		return freq
	}
}

// policyTypeDisplay mirrors the legacy JS policyTypeDisplay helper.
func policyTypeDisplay(pt string) string {
	switch pt {
	case "health":
		return "Health Insurance"
	case "life":
		return "Life Insurance"
	case "term":
		return "Term Life Insurance"
	default:
		return pt
	}
}

// displayName returns "first last" trimmed, or "" when empty.
func displayName(first, last string) string {
	return strings.TrimSpace(first + " " + last)
}

// trunc50 mirrors the legacy JS "truncate to 50 chars + '...'" behavior.
func trunc50(s string) string {
	if len(s) > 50 {
		return s[:50] + "..."
	}
	return s
}

func coreDashboard(db *database.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		user := c.MustGet("user").(*models.User)

		latestRecord, err := db.QueryOne(ctx,
			"SELECT * FROM health_records WHERE user_id = $1 ORDER BY recorded_at DESC LIMIT 1", user.ID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": dbErrorMessage(err)})
			return
		}
		var latestRecordData gin.H
		if latestRecord != nil {
			sys := int32Ptr(latestRecord["blood_pressure_systolic"])
			dia := int32Ptr(latestRecord["blood_pressure_diastolic"])
			var sysVal, diaVal any
			if sys != nil {
				sysVal = *sys
			}
			if dia != nil {
				diaVal = *dia
			}
			var bpSys, bpDia int32
			if sys != nil {
				bpSys = *sys
			}
			if dia != nil {
				bpDia = *dia
			}
			latestRecordData = gin.H{
				"blood_pressure_systolic":  sysVal,
				"blood_pressure_diastolic": diaVal,
				"bp_status":                bpStatus(bpSys, bpDia),
				"blood_sugar":              jsStringNum(latestRecord["blood_sugar"]),
				"weight":                   jsStringNum(latestRecord["weight"]),
				"heart_rate":               int32Ptr(latestRecord["heart_rate"]),
				"recorded_at":              latestRecord["recorded_at"],
			}
		}

		medCountRow, err := db.QueryOne(ctx,
			"SELECT COUNT(*) as count FROM medicines WHERE user_id = $1 AND is_active = 1", user.ID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": dbErrorMessage(err)})
			return
		}
		var activeCount any
		if medCountRow != nil {
			activeCount = asInt64(medCountRow["count"])
		}

		activities, err := db.QueryMany(ctx,
			"SELECT * FROM activity_logs WHERE user_id = $1 ORDER BY created_at DESC LIMIT 5", user.ID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": dbErrorMessage(err)})
			return
		}
		activitiesOut := make([]gin.H, 0, len(activities))
		for _, a := range activities {
			activitiesOut = append(activitiesOut, gin.H{
				"action":           stringOrEmpty(a["action"]),
				"action_display":   stringOrEmpty(a["action"]),
				"details":          stringOrEmpty(a["details"]),
				"created_at":       a["created_at"],
				"created_at_since": a["created_at"],
			})
		}

		latestMental, err := db.QueryOne(ctx,
			"SELECT * FROM mental_health_logs WHERE user_id = $1 ORDER BY recorded_at DESC LIMIT 1", user.ID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": dbErrorMessage(err)})
			return
		}
		var mentalData any
		if latestMental != nil {
			mentalData = gin.H{
				"sleep_hours":  jsStringNum(latestMental["sleep_hours"]),
				"mood_score":   asInt(latestMental["mood_score"]),
				"stress_level": asInt(latestMental["stress_level"]),
			}
		}

		name := displayName(user.FirstName, user.LastName)
		if name == "" {
			name = user.Username
		}

		c.JSON(http.StatusOK, gin.H{
			"user": gin.H{
				"name":  name,
				"email": user.Email,
			},
			"latest_record":          latestRecordData,
			"active_medicines":       activeCount,
			"active_medicines_count": activeCount,
			"recent_activities":      activitiesOut,
			"latest_mental_health":   mentalData,
		})
	}
}

func coreAddHealthRecord(db *database.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		user := c.MustGet("user").(*models.User)

		var b struct {
			BloodPressureSystolic  *int32   `json:"blood_pressure_systolic"`
			BloodPressureDiastolic *int32   `json:"blood_pressure_diastolic"`
			BloodSugar             *float32 `json:"blood_sugar"`
			Weight                 *float32 `json:"weight"`
			HeartRate              *int32   `json:"heart_rate"`
			Temperature            *float32 `json:"temperature"`
			OxygenLevel            *int32   `json:"oxygen_level"`
			Notes                  *string  `json:"notes"`
		}
		_ = c.ShouldBindJSON(&b)
		notes := ""
		if b.Notes != nil {
			notes = *b.Notes
		}
		if len(notes) > 2048 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "notes is too long (max 2048 chars)"})
			return
		}

		if _, err := db.Exec(ctx,
			`INSERT INTO health_records (user_id, blood_pressure_systolic, blood_pressure_diastolic, blood_sugar, weight, heart_rate, temperature, oxygen_level, notes)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			user.ID, b.BloodPressureSystolic, b.BloodPressureDiastolic, b.BloodSugar, b.Weight,
			b.HeartRate, b.Temperature, b.OxygenLevel, notes); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": dbErrorMessage(err)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Health record added successfully"})
	}
}

func coreAddMedicine(db *database.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		user := c.MustGet("user").(*models.User)

		var b struct {
			Name         string  `json:"name"`
			Dosage       string  `json:"dosage"`
			Frequency    string  `json:"frequency"`
			StartDate    string  `json:"start_date"`
			EndDate      *string `json:"end_date"`
			PrescribedBy string  `json:"prescribed_by"`
			Notes        string  `json:"notes"`
		}
		_ = c.ShouldBindJSON(&b)
		// Advisory bounds on user input; schema columns are TEXT so these
		// guard against oversized payloads, not DB constraints.
		tooLong := func(name, val string, capN int) bool {
			if len(val) > capN {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": name + " is too long"})
				return true
			}
			return false
		}
		if tooLong("name", b.Name, 256) || tooLong("dosage", b.Dosage, 256) ||
			tooLong("notes", b.Notes, 2048) || tooLong("prescribed_by", b.PrescribedBy, 256) ||
			tooLong("start_date", b.StartDate, 64) {
			return
		}
		if b.EndDate != nil && tooLong("end_date", *b.EndDate, 64) {
			return
		}
		frequency := b.Frequency
		if frequency == "" {
			frequency = "once"
		}
		prescribedBy := b.PrescribedBy
		if prescribedBy == "" {
			prescribedBy = ""
		}
		if _, err := db.Exec(ctx,
			`INSERT INTO medicines (user_id, name, dosage, frequency, start_date, end_date, prescribed_by, notes)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			user.ID, b.Name, b.Dosage, frequency, b.StartDate, b.EndDate, prescribedBy, b.Notes); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": dbErrorMessage(err)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Medicine added successfully"})
	}
}

func coreAddPrescription(db *database.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		user := c.MustGet("user").(*models.User)

		var b struct {
			DoctorName       string  `json:"doctor_name"`
			HospitalName     string  `json:"hospital_name"`
			Diagnosis        string  `json:"diagnosis"`
			PrescriptionDate string  `json:"prescription_date"`
			FollowUpDate     *string `json:"follow_up_date"`
			Notes            string  `json:"notes"`
		}
		_ = c.ShouldBindJSON(&b)

		tooLong := func(name, val string, capN int) bool {
			if len(val) > capN {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": name + " is too long"})
				return true
			}
			return false
		}
		if tooLong("doctor_name", b.DoctorName, 256) || tooLong("hospital_name", b.HospitalName, 256) ||
			tooLong("diagnosis", b.Diagnosis, 2048) || tooLong("notes", b.Notes, 2048) ||
			tooLong("prescription_date", b.PrescriptionDate, 64) {
			return
		}
		if b.FollowUpDate != nil && tooLong("follow_up_date", *b.FollowUpDate, 64) {
			return
		}

		// The legacy JS defaults an empty prescription_date to today's ISO date part.
		prescriptionDate := b.PrescriptionDate
		if prescriptionDate == "" {
			prescriptionDate = todayDate()
		}

		if _, err := db.Exec(ctx,
			`INSERT INTO prescriptions (user_id, doctor_name, hospital_name, diagnosis, prescription_date, follow_up_date, notes)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			user.ID, b.DoctorName, b.HospitalName, b.Diagnosis, prescriptionDate, b.FollowUpDate, b.Notes); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": dbErrorMessage(err)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Prescription added successfully"})
	}
}

func coreListMedicines(db *database.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		user := c.MustGet("user").(*models.User)

		medicines, err := db.QueryMany(ctx,
			"SELECT * FROM medicines WHERE user_id = $1 ORDER BY created_at DESC", user.ID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": dbErrorMessage(err)})
			return
		}
		countRow, err := db.QueryOne(ctx,
			"SELECT COUNT(*) as count FROM medicines WHERE user_id = $1 AND is_active = 1", user.ID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": dbErrorMessage(err)})
			return
		}
		var activeCount any
		if countRow != nil {
			activeCount = asInt64(countRow["count"])
		}

		list := make([]gin.H, 0, len(medicines))
		for _, m := range medicines {
			isActive := boolPtr(m["is_active"])
			list = append(list, gin.H{
				"name":              stringOrEmpty(m["name"]),
				"dosage":            stringOrEmpty(m["dosage"]),
				"frequency_display": frequencyDisplay(stringOrEmpty(m["frequency"])),
				"start_date":        m["start_date"],
				"end_date":          m["end_date"],
				"is_active":         isActive,
			})
		}
		c.JSON(http.StatusOK, gin.H{"medicines": list, "active_count": activeCount})
	}
}

func coreHealthTrack(db *database.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		user := c.MustGet("user").(*models.User)

		records, err := db.QueryMany(ctx,
			"SELECT * FROM health_records WHERE user_id = $1 ORDER BY recorded_at DESC", user.ID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": dbErrorMessage(err)})
			return
		}
		list := make([]gin.H, 0, len(records))
		for _, r := range records {
			sys := int32Ptr(r["blood_pressure_systolic"])
			dia := int32Ptr(r["blood_pressure_diastolic"])
			var bpSys, bpDia int32
			if sys != nil {
				bpSys = *sys
			}
			if dia != nil {
				bpDia = *dia
			}
			list = append(list, gin.H{
				"recorded_at":              r["recorded_at"],
				"blood_pressure_systolic":  sys,
				"blood_pressure_diastolic": dia,
				"blood_sugar":              jsStringNum(r["blood_sugar"]),
				"weight":                   jsStringNum(r["weight"]),
				"heart_rate":               int32Ptr(r["heart_rate"]),
				"oxygen_level":             jsStringNum(r["oxygen_level"]),
				"bp_status":                bpStatus(bpSys, bpDia),
			})
		}
		c.JSON(http.StatusOK, gin.H{"records": list})
	}
}

func corePrescriptions(db *database.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		user := c.MustGet("user").(*models.User)

		prescriptions, err := db.QueryMany(ctx,
			"SELECT * FROM prescriptions WHERE user_id = $1 ORDER BY created_at DESC", user.ID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": dbErrorMessage(err)})
			return
		}
		list := make([]gin.H, 0, len(prescriptions))
		for _, p := range prescriptions {
			list = append(list, gin.H{
				"prescription_date": dateOnly(p["created_at"]),
				"doctor_name":       stringOrEmpty(p["doctor_name"]),
				"hospital_name":     stringOrEmpty(p["hospital_name"]),
				"diagnosis":         trunc50(stringOrEmpty(p["notes"])),
				"follow_up_date":    p["follow_up_date"],
			})
		}
		c.JSON(http.StatusOK, gin.H{"prescriptions": list})
	}
}

func coreProfile(db *database.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := c.MustGet("user").(*models.User)

		c.JSON(http.StatusOK, gin.H{
			"user": gin.H{
				"first_name":        user.FirstName,
				"last_name":         user.LastName,
				"email":             user.Email,
				"phone":             user.Phone,
				"city":              user.City,
				"blood_group":       user.BloodGroup,
				"address":           user.Address,
				"emergency_contact": user.EmergencyContact,
				"emergency_phone":   user.EmergencyPhone,
			},
			"messages": []gin.H{
				{"tags": "success", "message": "Welcome back, " + user.Username + "!"},
			},
		})
	}
}

func coreMentalHealth(db *database.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		user := c.MustGet("user").(*models.User)

		logs, err := db.QueryMany(ctx,
			"SELECT * FROM mental_health_logs WHERE user_id = $1 ORDER BY recorded_at DESC", user.ID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": dbErrorMessage(err)})
			return
		}

		avgMood := 0.0
		if len(logs) > 0 {
			var sum float64
			for _, l := range logs {
				sum += float64(asInt(l["mood_score"]))
			}
			avgMood = math.Round(sum/float64(len(logs))*10) / 10
		}

		list := make([]gin.H, 0, len(logs))
		for _, l := range logs {
			list = append(list, gin.H{
				"recorded_at":          l["recorded_at"],
				"mood_score":           asInt(l["mood_score"]),
				"mood_score_display":   moodDisplay(asInt(l["mood_score"])),
				"stress_level_display": moodDisplay(asInt(l["stress_level"])),
				"sleep_hours":          numPtr(l["sleep_hours"]),
				"notes":                stringOrEmpty(l["notes"]),
			})
		}
		c.JSON(http.StatusOK, gin.H{"avg_mood": avgMood, "logs": list})
	}
}

func coreLifestyle(db *database.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		user := c.MustGet("user").(*models.User)

		logs, err := db.QueryMany(ctx,
			"SELECT * FROM lifestyle_logs WHERE user_id = $1 ORDER BY recorded_at DESC", user.ID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": dbErrorMessage(err)})
			return
		}
		list := make([]gin.H, 0, len(logs))
		for _, l := range logs {
			list = append(list, gin.H{
				"recorded_at":       l["recorded_at"],
				"water_intake":      asInt(l["water_intake"]),
				"exercise_minutes":  asInt(l["exercise_minutes"]),
				"steps_count":       asInt(l["steps_count"]),
				"calories_consumed": numPtr(l["calories_consumed"]),
			})
		}
		c.JSON(http.StatusOK, gin.H{"logs": list})
	}
}

func coreInsurance(db *database.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		user := c.MustGet("user").(*models.User)

		policies, err := db.QueryMany(ctx,
			"SELECT * FROM insurance_policies WHERE user_id = $1 ORDER BY created_at DESC", user.ID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": dbErrorMessage(err)})
			return
		}
		countRow, err := db.QueryOne(ctx,
			"SELECT COUNT(*) as count FROM insurance_policies WHERE user_id = $1 AND is_active = 1", user.ID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": dbErrorMessage(err)})
			return
		}
		var activeCount any
		if countRow != nil {
			activeCount = asInt64(countRow["count"])
		}

		list := make([]gin.H, 0, len(policies))
		for _, p := range policies {
			isActive := boolPtr(p["is_active"])
			list = append(list, gin.H{
				"provider_name":       stringOrEmpty(p["provider_name"]),
				"policy_type_display": policyTypeDisplay(stringOrEmpty(p["policy_type"])),
				"policy_number":       stringOrEmpty(p["policy_number"]),
				"coverage_amount":     numPtr(p["coverage_amount"]),
				"end_date":            p["end_date"],
				"is_active":           isActive,
			})
		}
		c.JSON(http.StatusOK, gin.H{"policies": list, "active_policies": activeCount})
	}
}

func corePastRecords(db *database.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		user := c.MustGet("user").(*models.User)

		healthRecords, err := db.QueryMany(ctx,
			"SELECT * FROM health_records WHERE user_id = $1 ORDER BY recorded_at DESC", user.ID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": dbErrorMessage(err)})
			return
		}
		prescriptions, err := db.QueryMany(ctx,
			"SELECT * FROM prescriptions WHERE user_id = $1 ORDER BY prescription_date DESC", user.ID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": dbErrorMessage(err)})
			return
		}

		recordsOut := make([]gin.H, 0, len(healthRecords))
		for _, r := range healthRecords {
			recordsOut = append(recordsOut, gin.H{
				"recorded_at":              dateOnly(r["recorded_at"]),
				"blood_pressure_systolic":  int32Ptr(r["blood_pressure_systolic"]),
				"blood_pressure_diastolic": int32Ptr(r["blood_pressure_diastolic"]),
			})
		}
		prescriptionsOut := make([]gin.H, 0, len(prescriptions))
		for _, p := range prescriptions {
			prescriptionsOut = append(prescriptionsOut, gin.H{
				"prescription_date": p["prescription_date"],
				"doctor_name":       stringOrEmpty(p["doctor_name"]),
			})
		}
		c.JSON(http.StatusOK, gin.H{"health_records": recordsOut, "prescriptions": prescriptionsOut})
	}
}

func coreListAppointments(db *database.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		user := c.MustGet("user").(*models.User)

		serverFail := func(msg string) { c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": msg}) }

		var appointments []map[string]any
		if user.UserType == "provider" {
			providerRow, err := db.QueryOne(ctx, "SELECT * FROM service_providers WHERE user_id = $1", user.ID)
			if err != nil {
				serverFail(dbErrorMessage(err))
				return
			}
			if providerRow == nil || stringOrEmpty(providerRow["provider_type"]) != "doctor" {
				c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Not a doctor"})
				return
			}
			appointments, err = db.QueryMany(ctx,
				`SELECT a.*, p.first_name as p_first, p.last_name as p_last, d.first_name as d_first, d.last_name as d_last
				 FROM appointments a JOIN users p ON a.patient_id = p.id JOIN users d ON a.doctor_id = d.id
				 WHERE a.doctor_id = $1 ORDER BY a.date, a.time`, user.ID)
			if err != nil {
				serverFail(dbErrorMessage(err))
				return
			}
		} else {
			var err error
			appointments, err = db.QueryMany(ctx,
				`SELECT a.*, p.first_name as p_first, p.last_name as p_last, d.first_name as d_first, d.last_name as d_last
				 FROM appointments a JOIN users p ON a.patient_id = p.id JOIN users d ON a.doctor_id = d.id
				 WHERE a.patient_id = $1 ORDER BY a.date, a.time`, user.ID)
			if err != nil {
				serverFail(dbErrorMessage(err))
				return
			}
		}

		list := make([]gin.H, 0, len(appointments))
		for _, a := range appointments {
			list = append(list, gin.H{
				"id":           asInt64(a["id"]),
				"patient_name": displayName(stringOrEmpty(a["p_first"]), stringOrEmpty(a["p_last"])),
				"doctor_name":  displayName(stringOrEmpty(a["d_first"]), stringOrEmpty(a["d_last"])),
				"date":         stringOrEmpty(a["date"]),
				"time":         stringOrEmpty(a["time"]),
				"reason":       stringOrEmpty(a["reason"]),
				"status":       stringOrEmpty(a["status"]),
				"type":         stringOrEmpty(a["type"]),
				"meeting_link": strPtr(a["meeting_link"]),
			})
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "appointments": list})
	}
}

func coreBookAppointment(db *database.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		user := c.MustGet("user").(*models.User)

		var b struct {
			DoctorID *int64  `json:"doctor_id"`
			Date     *string `json:"date"`
			Time     *string `json:"time"`
			Reason   *string `json:"reason"`
			Type     *string `json:"type"`
		}
		_ = c.ShouldBindJSON(&b)

		apptType := ""
		if b.Type != nil {
			apptType = *b.Type
		}
		if apptType == "" {
			apptType = "Video Consult"
		}
		if b.Reason != nil && len(*b.Reason) > 2048 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "reason is too long"})
			return
		}

		lastID, err := db.Run(ctx,
			"INSERT INTO appointments (patient_id, doctor_id, date, time, reason, type) VALUES ($1, $2, $3, $4, $5, $6)",
			user.ID, b.DoctorID, b.Date, b.Time, b.Reason, apptType)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": dbErrorMessage(err)})
			return
		}

		doctorIDStr := "nil"
		if b.DoctorID != nil {
			doctorIDStr = intToString(*b.DoctorID)
		}
		if _, err := db.Exec(ctx,
			"INSERT INTO activity_logs (user_id, action, details) VALUES ($1, $2, $3)",
			user.ID, "appointment_booked", "Booked appointment with doctor "+doctorIDStr); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": dbErrorMessage(err)})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "message": "Appointment booked successfully", "id": lastID})
	}
}

func coreAppointmentAction(db *database.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		user := c.MustGet("user").(*models.User)

		var b struct {
			Action string `json:"action"`
		}
		_ = c.ShouldBindJSON(&b)

		appt, err := db.QueryOne(ctx, "SELECT * FROM appointments WHERE id = $1", c.Param("appointment_id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": dbErrorMessage(err)})
			return
		}
		if appt == nil {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Appointment not found"})
			return
		}
		if asInt64(appt["doctor_id"]) != user.ID {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Permission denied"})
			return
		}

		status := stringOrEmpty(appt["status"])
		meetingLink := strPtr(appt["meeting_link"])
		switch b.Action {
		case "accept":
			status = "confirmed"
			if stringOrEmpty(appt["type"]) == "Video Consult" {
				link := "https://meet.google.com/new"
				meetingLink = &link
			}
		case "reject":
			status = "cancelled"
		case "complete":
			status = "completed"
		}

		if _, err := db.Exec(ctx,
			"UPDATE appointments SET status = $1, meeting_link = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $3",
			status, meetingLink, asInt64(appt["id"])); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": dbErrorMessage(err)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "status": status})
	}
}

func coreListServiceRequests(db *database.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		user := c.MustGet("user").(*models.User)

		joinSQL := `SELECT r.*, p.first_name as p_first, p.last_name as p_last, pv.first_name as pv_first, pv.last_name as pv_last
			FROM service_requests r JOIN users p ON r.patient_id = p.id JOIN users pv ON r.provider_id = pv.id`

		serverFail := func(msg string) { c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": msg}) }

		var requests []map[string]any
		var err error
		if user.UserType == "provider" {
			requests, err = db.QueryMany(ctx, joinSQL+" WHERE r.provider_id = $1 ORDER BY r.created_at DESC", user.ID)
		} else {
			requests, err = db.QueryMany(ctx, joinSQL+" WHERE r.patient_id = $1 ORDER BY r.created_at DESC", user.ID)
		}
		if err != nil {
			serverFail(dbErrorMessage(err))
			return
		}

		list := make([]gin.H, 0, len(requests))
		for _, r := range requests {
			list = append(list, gin.H{
				"id":             asInt64(r["id"]),
				"patient_name":   displayName(stringOrEmpty(r["p_first"]), stringOrEmpty(r["p_last"])),
				"provider_name":  displayName(stringOrEmpty(r["pv_first"]), stringOrEmpty(r["pv_last"])),
				"service_name":   stringOrEmpty(r["service_name"]),
				"price":          numPtr(r["service_price"]),
				"status":         stringOrEmpty(r["status"]),
				"address":        stringOrEmpty(r["address"]),
				"scheduled_date": strPtr(r["scheduled_date"]),
			})
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "requests": list})
	}
}

func coreServiceRequest(db *database.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		user := c.MustGet("user").(*models.User)

		var b struct {
			ProviderID  *int64   `json:"provider_id"`
			ServiceName *string  `json:"service_name"`
			Price       *float32 `json:"price"`
			Address     *string  `json:"address"`
		}
		_ = c.ShouldBindJSON(&b)
		if (b.ServiceName != nil && len(*b.ServiceName) > 256) || (b.Address != nil && len(*b.Address) > 2048) {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Input too long"})
			return
		}

		lastID, err := db.Run(ctx,
			"INSERT INTO service_requests (patient_id, provider_id, service_name, service_price, address) VALUES ($1, $2, $3, $4, $5)",
			user.ID, b.ProviderID, b.ServiceName, b.Price, b.Address)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": dbErrorMessage(err)})
			return
		}

		serviceName := "nil"
		if b.ServiceName != nil {
			serviceName = *b.ServiceName
		}
		providerIDStr := "nil"
		if b.ProviderID != nil {
			providerIDStr = intToString(*b.ProviderID)
		}
		if _, err := db.Exec(ctx,
			"INSERT INTO activity_logs (user_id, action, details) VALUES ($1, $2, $3)",
			user.ID, "service_requested", "Requested "+serviceName+" from provider "+providerIDStr); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": dbErrorMessage(err)})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "message": "Service requested successfully", "id": lastID})
	}
}

func coreServiceRequestAction(db *database.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		user := c.MustGet("user").(*models.User)

		var b struct {
			Action string `json:"action"`
		}
		_ = c.ShouldBindJSON(&b)

		reqRow, err := db.QueryOne(ctx, "SELECT * FROM service_requests WHERE id = $1", c.Param("request_id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": dbErrorMessage(err)})
			return
		}
		if reqRow == nil {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Service request not found"})
			return
		}
		if asInt64(reqRow["provider_id"]) != user.ID {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Permission denied"})
			return
		}

		status := stringOrEmpty(reqRow["status"])
		switch b.Action {
		case "accept":
			status = "accepted"
		case "decline":
			status = "declined"
		case "complete":
			status = "completed"
		}

		if _, err := db.Exec(ctx,
			"UPDATE service_requests SET status = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2",
			status, asInt64(reqRow["id"])); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": dbErrorMessage(err)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "status": status})
	}
}

// todayDate returns the local YYYY-MM-DD date, matching the legacy JS
// `new Date().toISOString().split('T')[0]` (which is UTC-based).
func todayDate() string {
	return time.Now().UTC().Format("2006-01-02")
}

// intToString formats an int64 the way JS template interpolation would.
func intToString(n int64) string {
	return strconv.FormatInt(n, 10)
}

// dbErrorMessage maps an error to a client-safe message. Any error that
// originates in the Postgres driver is reduced to a generic message so that
// table names, constraint details or DSN fragments can never reach the
// response body; non-driver errors (JSON binding, validation) keep their
// text.
func dbErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	var pge *pgconn.PgError
	if errors.As(err, &pge) {
		return "Database request failed. Please try again."
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return "Database request failed. Please try again."
	}
	// Context errors (request cancellation, deadline) are driver-visible but
	// not PgError; keep the message generic as well.
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return "Database request failed. Please try again."
	}
	return err.Error()
}
