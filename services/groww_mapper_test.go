package services

import (
	"encoding/json"
	"testing"

	"github.com/fenilmodi00/ipo-backend/models"
	"github.com/stretchr/testify/require"
)

func TestGrowwMapper_MapToIPO_MapsParsedCMSRichData(t *testing.T) {
	t.Parallel()

	mapper := NewGrowwMapper(NewUtilityService())

	groww := &models.GrowwScrapedIPO{
		Slug: "demo-ipo",
		Details: &models.GrowwIPODetailsResponse{
			CompanyName: "Demo IPO Ltd",
			Status:      "ACTIVE",
			Registrar:   "Bigshare",
		},
		ParsedCMS: &models.GrowwParsedCMS{
			LeadManager: "Lead Manager Pvt Ltd",
			Objectives: []models.GrowwObjective{{
				Purpose:     "Working Capital",
				Amount:      "10",
				Description: "For day-to-day operations",
			}},
		},
	}

	ipo := mapper.MapToIPO(groww, nil)
	require.NotNil(t, ipo)
	require.NotEmpty(t, ipo.RichData)
	require.NotEmpty(t, ipo.GrowwDetails)

	var decoded models.GrowwParsedCMS
	require.NoError(t, json.Unmarshal(ipo.RichData, &decoded))
	require.Equal(t, "Lead Manager Pvt Ltd", decoded.LeadManager)
	require.Len(t, decoded.Objectives, 1)
	require.Equal(t, "Working Capital", decoded.Objectives[0].Purpose)

	var decodedDetails models.GrowwIPODetailsResponse
	require.NoError(t, json.Unmarshal(ipo.GrowwDetails, &decodedDetails))
	require.Equal(t, "Demo IPO Ltd", decodedDetails.CompanyName)
}
