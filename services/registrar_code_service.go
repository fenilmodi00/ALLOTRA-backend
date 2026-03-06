package services

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/fenilmodi00/ipo-backend/models"
	"github.com/fenilmodi00/ipo-backend/repositories"
	"github.com/fenilmodi00/ipo-backend/tools/registrars"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// RegistrarCodeService handles business logic for registrar code resolution
type RegistrarCodeService struct {
	db   *sql.DB
	repo repositories.RegistrarCodeRepository
}

// NewRegistrarCodeService creates a new registrar code service
func NewRegistrarCodeService(db *sql.DB, repo repositories.RegistrarCodeRepository) *RegistrarCodeService {
	return &RegistrarCodeService{
		db:   db,
		repo: repo,
	}
}

// ResolveCode resolves a registrar code by calling the registrar client and upserting if score >= 80.0
func (s *RegistrarCodeService) ResolveCode(ctx context.Context, ipoID uuid.UUID, registrarShortCode string, ipoName string) (*models.RegistrarCode, error) {
	logger := logrus.WithFields(logrus.Fields{
		"ipo_id":               ipoID.String(),
		"registrar_short_code": registrarShortCode,
		"ipo_name":             ipoName,
	})

	// Get registrar client
	client := registrars.GetClient(registrarShortCode)
	if client == nil {
		logger.Error("Registrar client not found")
		return nil, fmt.Errorf("registrar client not found: %s", registrarShortCode)
	}

	// Create context with timeout for external call
	resolveCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Get active IPOs from registrar
	activeIPOs, err := client.GetActiveIPOs(resolveCtx)
	if err != nil {
		logger.WithError(err).Warn("Failed to fetch active IPOs from registrar")
		// Create unresolved entry but don't fail
		code := &models.RegistrarCode{
			ID:                 uuid.New(),
			IPOID:              ipoID,
			RegistrarShortCode: registrarShortCode,
			IPOName:            &ipoName,
			MatchScore:         0,
			IsResolved:         false,
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
		}
		now := time.Now()
		code.LastAttemptedAt = &now

		if upsertErr := s.repo.Upsert(ctx, code); upsertErr != nil {
			logger.WithError(upsertErr).Error("Failed to upsert unresolved registrar code")
			return nil, fmt.Errorf("upsert unresolved code: %w", upsertErr)
		}
		return code, nil
	}

	// Match company name against active IPOs
	companyCode, score := client.MatchCompanyName(ipoName, activeIPOs)

	logger.WithFields(logrus.Fields{
		"match_score":  score,
		"company_code": companyCode,
		"threshold":    80.0,
	}).Info("Company name match result")

	// Create registrar code entry
	code := &models.RegistrarCode{
		ID:                   uuid.New(),
		IPOID:                ipoID,
		RegistrarShortCode:   registrarShortCode,
		RegistrarCompanyCode: &companyCode,
		IPOName:              &ipoName,
		MatchScore:           score,
		IsResolved:           float64(score) >= 80.0,
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}
	now := time.Now()
	code.LastAttemptedAt = &now

	// Upsert to database
	if err := s.repo.Upsert(ctx, code); err != nil {
		logger.WithError(err).Error("Failed to upsert registrar code")
		return nil, fmt.Errorf("upsert registrar code: %w", err)
	}

	// Write back resolved company_code to ipo_list
	if code.IsResolved && code.RegistrarCompanyCode != nil && *code.RegistrarCompanyCode != "" {
		updateQuery := `
			UPDATE ipo_list 
			SET company_code = $1, is_fetched = true, updated_at = $2
			WHERE id = $3
		`
		_, err := s.db.ExecContext(ctx, updateQuery, *code.RegistrarCompanyCode, time.Now(), ipoID)
		if err != nil {
			logger.WithError(err).Error("Failed to update ipo_list.company_code")
			// Don't fail - ipo_registrar_codes was already updated
		} else {
			logger.WithField("company_code", *code.RegistrarCompanyCode).Info("Updated ipo_list.company_code")
		}
	}

	logger.WithFields(logrus.Fields{
		"is_resolved":  code.IsResolved,
		"company_code": companyCode,
	}).Info("Registrar code resolved and saved")

	return code, nil
}

// GetResolvedCode retrieves a stored resolved code by IPO ID and registrar short code
func (s *RegistrarCodeService) GetResolvedCode(ctx context.Context, ipoID uuid.UUID, registrarShortCode string) (*models.RegistrarCode, error) {
	code, err := s.repo.GetByIPOAndRegistrar(ctx, ipoID.String(), registrarShortCode)
	if err != nil {
		if err.Error() == "not found" {
			logrus.WithFields(logrus.Fields{
				"ipo_id":               ipoID.String(),
				"registrar_short_code": registrarShortCode,
			}).Debug("Resolved code not found")
			return nil, err
		}
		logrus.WithError(err).Error("Failed to retrieve resolved code")
		return nil, fmt.Errorf("get resolved code: %w", err)
	}

	if !code.IsResolved {
		return nil, fmt.Errorf("code not resolved: %w", repositories.ErrNotFound)
	}

	return code, nil
}

// GetUnresolvedForToday retrieves unresolved registrar codes for today (for scheduler)
func (s *RegistrarCodeService) GetUnresolvedForToday(ctx context.Context) ([]*models.RegistrarCode, error) {
	today := time.Now()
	codes, err := s.repo.GetUnresolvedByResultDate(ctx, today)
	if err != nil {
		logrus.WithError(err).Error("Failed to retrieve unresolved codes for today")
		return nil, fmt.Errorf("get unresolved codes for today: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"count": len(codes),
		"date":  today.Format("2006-01-02"),
	}).Info("Retrieved unresolved codes for today")

	return codes, nil
}
