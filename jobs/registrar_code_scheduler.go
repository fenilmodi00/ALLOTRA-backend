package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// RegistrarCodeScheduler schedules jobs to fetch registrar company codes
// Runs every 30 minutes starting at 1 PM IST on result_date
type RegistrarCodeScheduler struct {
	db       *sql.DB
	interval time.Duration
	stopChan chan struct{}
}

// NewRegistrarCodeScheduler creates a new scheduler for registrar code fetching
func NewRegistrarCodeScheduler(db *sql.DB, interval time.Duration) *RegistrarCodeScheduler {
	return &RegistrarCodeScheduler{
		db:       db,
		interval: interval,
		stopChan: make(chan struct{}),
	}
}

// Start launches the scheduler in a background goroutine
// It runs every interval, checking if it's >= 1 PM IST and result_date is today
func (s *RegistrarCodeScheduler) Start() {
	logrus.WithField("interval", s.interval.String()).Info("Starting registrar code scheduler")

	go func() {
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.scheduleRegistrarCodeJobs()
			case <-s.stopChan:
				logrus.Info("Registrar code scheduler stopped")
				return
			}
		}
	}()
}

// Stop gracefully shuts down the scheduler
func (s *RegistrarCodeScheduler) Stop() {
	close(s.stopChan)
}

// scheduleRegistrarCodeJobs checks conditions and creates job_dispatch entries
func (s *RegistrarCodeScheduler) scheduleRegistrarCodeJobs() {
	logger := logrus.WithField("component", "registrar_code_scheduler")

	// Load IST timezone
	istLocation, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		logger.WithError(err).Error("Failed to load IST timezone")
		return
	}

	// Get current time in IST
	istNow := time.Now().In(istLocation)

	// Check if current hour is >= 13 (1 PM IST)
	if istNow.Hour() < 13 {
		logger.WithField("hour_ist", istNow.Hour()).Debug("Skipping: before 1 PM IST")
		return
	}

	logger.WithField("hour_ist", istNow.Hour()).Debug("Time check passed (>= 1 PM IST), querying IPOs")

	// Query IPOs where result_date = today and registrar is not empty
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	query := `
		SELECT id, name, registrar 
		FROM ipo_list 
		WHERE DATE(result_date AT TIME ZONE 'Asia/Kolkata') = DATE($1 AT TIME ZONE 'Asia/Kolkata')
		  AND registrar != ''
	`

	rows, err := s.db.QueryContext(ctx, query, istNow)
	if err != nil {
		logger.WithError(err).Error("Failed to query IPO list for result_date")
		return
	}
	defer rows.Close()

	var ipos []struct {
		ID        uuid.UUID
		Name      string
		Registrar string
	}

	for rows.Next() {
		var ipo struct {
			ID        uuid.UUID
			Name      string
			Registrar string
		}

		if err := rows.Scan(&ipo.ID, &ipo.Name, &ipo.Registrar); err != nil {
			logger.WithError(err).Warn("Failed to scan IPO row")
			continue
		}

		ipos = append(ipos, ipo)
	}

	if err := rows.Err(); err != nil {
		logger.WithError(err).Error("Error iterating IPO rows")
		return
	}

	logger.WithField("ipo_count", len(ipos)).Info("Found IPOs with result_date today")

	// For each IPO, check if registrar codes are already resolved, then insert job_dispatch
	for _, ipo := range ipos {
		registrarShortCode := extractRegistrarShortCode(ipo.Registrar)
		if registrarShortCode == "" {
			logrus.WithFields(logrus.Fields{
				"ipo_id":    ipo.ID.String(),
				"ipo_name":  ipo.Name,
				"registrar": ipo.Registrar,
			}).Warn("Could not extract registrar short code")
			continue
		}

		// Check if ipo_registrar_codes already exists with is_resolved = true
		alreadyResolved, err := s.isCodeAlreadyResolved(ctx, ipo.ID, registrarShortCode)
		if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{
				"ipo_id":   ipo.ID.String(),
				"ipo_name": ipo.Name,
			}).Warn("Failed to check if code is already resolved")
			continue
		}

		if alreadyResolved {
			logrus.WithFields(logrus.Fields{
				"ipo_id":               ipo.ID.String(),
				"ipo_name":             ipo.Name,
				"registrar_short_code": registrarShortCode,
			}).Debug("Registrar code already resolved, skipping")
			continue
		}

		// Check for existing pending/running jobs to avoid duplicates
		hasExistingJob, err := s.hasPendingJob(ctx, ipo.ID)
		if err != nil {
			logrus.WithError(err).WithField("ipo_id", ipo.ID.String()).Warn("Failed to check for existing jobs")
			continue
		}
		if hasExistingJob {
			logrus.WithFields(logrus.Fields{
				"ipo_id":   ipo.ID.String(),
				"ipo_name": ipo.Name,
			}).Debug("Pending job already exists, skipping")
			continue
		}

		// Create payload
		payload := FetchRegistrarCodePayload{
			IPOID:              ipo.ID,
			RegistrarShortCode: registrarShortCode,
			IPOName:            ipo.Name,
		}

		payloadJSON, err := json.Marshal(payload)
		if err != nil {
			logrus.WithError(err).WithField("ipo_id", ipo.ID.String()).Error("Failed to marshal payload")
			continue
		}

		// Insert job_dispatch row
		insertQuery := `
			INSERT INTO job_dispatch (job_type, payload, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id
		`

		var jobID string
		err = s.db.QueryRowContext(
			ctx,
			insertQuery,
			"fetch_registrar_company_code",
			payloadJSON,
			"pending",
			time.Now(),
			time.Now(),
		).Scan(&jobID)

		if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{
				"ipo_id":   ipo.ID.String(),
				"ipo_name": ipo.Name,
			}).Error("Failed to insert job_dispatch")
			continue
		}

		logrus.WithFields(logrus.Fields{
			"job_id":               jobID,
			"ipo_id":               ipo.ID.String(),
			"ipo_name":             ipo.Name,
			"registrar_short_code": registrarShortCode,
		}).Info("Created fetch_registrar_company_code job")
	}
}

// isCodeAlreadyResolved checks if a registrar code entry exists with is_resolved = true
func (s *RegistrarCodeScheduler) isCodeAlreadyResolved(ctx context.Context, ipoID uuid.UUID, registrarShortCode string) (bool, error) {
	query := `
		SELECT COUNT(*) 
		FROM ipo_registrar_codes 
		WHERE ipo_id = $1 
		  AND registrar_short_code = $2 
		  AND is_resolved = true
	`

	var count int
	err := s.db.QueryRowContext(ctx, query, ipoID, registrarShortCode).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// hasPendingJob checks if there's already a pending or running job for this IPO
func (s *RegistrarCodeScheduler) hasPendingJob(ctx context.Context, ipoID uuid.UUID) (bool, error) {
	query := `
		SELECT COUNT(*) 
		FROM job_dispatch 
		WHERE job_type = 'fetch_registrar_company_code' 
		  AND status IN ('pending', 'running')
		  AND payload->>'ipo_id' = $1
	`

	var count int
	err := s.db.QueryRowContext(ctx, query, ipoID.String()).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// extractRegistrarShortCode maps full registrar name to short code
// Supports common Indian IPO registrars
func extractRegistrarShortCode(registrar string) string {
	// Normalize input to lowercase for case-insensitive matching
	lowerRegistrar := strings.ToLower(registrar)

	// Map full registrar names to their short codes (all keys lowercase)
	registrarMap := map[string]string{
		"kfin":                                        "KFIN",
		"kfin technologies limited":                   "KFIN",
		"kfin technologies pvt ltd":                   "KFIN",
		"bigshare services":                           "BIGSHARE",
		"bigshare services pvt ltd":                   "BIGSHARE",
		"mufg bank japan limited":                     "MUFG",
		"mufg":                                        "MUFG",
		"bank of india":                               "BOI",
		"computershare india pvt ltd":                 "COMPUTERSHARE",
		"nsdl database management limited":            "NSDL",
		"central depository services (india) limited": "CDSL",
	}

	// Direct lookup with normalized key
	if code, exists := registrarMap[lowerRegistrar]; exists {
		return code
	}

	// Fuzzy matching: use strings.Contains for partial matches
	switch {
	case strings.Contains(lowerRegistrar, "bigshare"):
		return "BIGSHARE"
	case strings.Contains(lowerRegistrar, "mufg") || strings.Contains(lowerRegistrar, "intime") || strings.Contains(lowerRegistrar, "link"):
		return "MUFG"
	case strings.Contains(lowerRegistrar, "kfin") || strings.Contains(lowerRegistrar, "kfintech"):
		return "KFIN"
	}

	// Fallback: return empty if not recognized
	return ""
}
