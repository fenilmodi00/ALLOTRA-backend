package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fenilmodi00/ipo-backend/services"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// FetchRegistrarCodePayload represents the payload for fetch_registrar_code job
type FetchRegistrarCodePayload struct {
	IPOID              uuid.UUID `json:"ipo_id"`
	RegistrarShortCode string    `json:"registrar_short_code"`
	IPOName            string    `json:"ipo_name"`
}

// FetchRegistrarCodeJobExecutor creates a job executor for fetching registrar codes
// It validates that result_date is today and current time is >= 13:00 IST before resolving
func FetchRegistrarCodeJobExecutor(
	registrarCodeService *services.RegistrarCodeService,
	ipoService *services.IPOService,
) func(context.Context, JobDispatch) error {
	return func(ctx context.Context, job JobDispatch) error {
		logger := logrus.WithField("job_id", job.ID)

		// Parse payload
		var payload FetchRegistrarCodePayload
		if err := json.Unmarshal(job.Payload, &payload); err != nil {
			logger.WithError(err).Error("Failed to parse fetch_registrar_code payload")
			return fmt.Errorf("parse payload: %w", err)
		}

		logger = logger.WithFields(logrus.Fields{
			"ipo_id":               payload.IPOID.String(),
			"registrar_short_code": payload.RegistrarShortCode,
			"ipo_name":             payload.IPOName,
		})

		// Get IPO from database
		ipo, err := ipoService.GetIPOByID(ctx, payload.IPOID.String())
		if err != nil {
			logger.WithError(err).Error("Failed to get IPO from database")
			return fmt.Errorf("get IPO: %w", err)
		}

		// Load IST timezone
		istLocation, err := time.LoadLocation("Asia/Kolkata")
		if err != nil {
			logger.WithError(err).Error("Failed to load IST timezone")
			return fmt.Errorf("load IST location: %w", err)
		}

		// Get current time in IST
		istNow := time.Now().In(istLocation)

		// Check if result_date is set (nil check)
		if ipo.ResultDate == nil {
			logger.WithField("ipo_name", payload.IPOName).Info("Skipping IPO: no result_date set")
			return nil
		}

		// Check if result_date is today
		resultDateIST := ipo.ResultDate.In(istLocation)
		todayIST := istNow.Truncate(24 * time.Hour)
		resultDateTruncated := resultDateIST.Truncate(24 * time.Hour)

		if resultDateTruncated != todayIST {
			logger.WithFields(logrus.Fields{
				"result_date": resultDateIST.Format(time.RFC3339),
				"today_ist":   todayIST.Format(time.RFC3339),
			}).Info("Skipping: result_date is not today")
			return nil
		}

		// Check if current time is >= 13:00 IST (1 PM)
		if istNow.Hour() < 13 {
			logger.WithField("current_hour_ist", istNow.Hour()).Info("Skipping: before 1 PM IST")
			return nil
		}

		// Call registrar code service to resolve code
		_, err = registrarCodeService.ResolveCode(ctx, payload.IPOID, payload.RegistrarShortCode, payload.IPOName)
		if err != nil {
			logger.WithError(err).Error("Failed to resolve registrar code")
			return fmt.Errorf("resolve code: %w", err)
		}

		logger.Info("Successfully resolved registrar code")
		return nil
	}
}
