package services

import (
	"math"
	"strings"
)

type MatcherService struct {
	utilityService *UtilityService
}

func NewMatcherService(utilityService *UtilityService) *MatcherService {
	if utilityService == nil {
		panic("utilityService cannot be nil")
	}
	return &MatcherService{
		utilityService: utilityService,
	}
}

type MatchResult struct {
	IsMatch          bool
	Confidence       float64
	MatchType        string
	NormalizedSource string
	NormalizedTarget string
}

const (
	MatchTypeExact     = "exact"
	MatchTypeFuzzy     = "fuzzy"
	MatchTypeNoMatch   = "no_match"
	MinConfidenceExact = 100.0
	MinConfidenceFuzzy = 85.0
)

func (s *MatcherService) MatchIPOs(sourceName, targetName string) *MatchResult {
	if sourceName == "" || targetName == "" {
		return &MatchResult{IsMatch: false, Confidence: 0, MatchType: MatchTypeNoMatch}
	}

	normalizedSource := s.utilityService.NormalizeIPOName(sourceName)
	normalizedTarget := s.utilityService.NormalizeIPOName(targetName)

	if normalizedSource == normalizedTarget {
		return &MatchResult{
			IsMatch:          true,
			Confidence:       MinConfidenceExact,
			MatchType:        MatchTypeExact,
			NormalizedSource: normalizedSource,
			NormalizedTarget: normalizedTarget,
		}
	}

	confidence := s.calculateLevenshteinConfidence(normalizedSource, normalizedTarget)

	if confidence >= MinConfidenceFuzzy {
		return &MatchResult{
			IsMatch:          true,
			Confidence:       confidence,
			MatchType:        MatchTypeFuzzy,
			NormalizedSource: normalizedSource,
			NormalizedTarget: normalizedTarget,
		}
	}

	return &MatchResult{
		IsMatch:          false,
		Confidence:       confidence,
		MatchType:        MatchTypeNoMatch,
		NormalizedSource: normalizedSource,
		NormalizedTarget: normalizedTarget,
	}
}

func (s *MatcherService) calculateLevenshteinConfidence(s1, s2 string) float64 {
	distance := levenshteinDistance(s1, s2)
	maxLen := math.Max(float64(len(s1)), float64(len(s2)))

	if maxLen == 0 {
		return 100.0
	}

	similarity := (1.0 - float64(distance)/maxLen) * 100
	return similarity
}

func levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	matrix := make([][]int, len(s1)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(s2)+1)
	}

	for i := 0; i <= len(s1); i++ {
		matrix[i][0] = i
	}
	for j := 0; j <= len(s2); j++ {
		matrix[0][j] = j
	}

	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 1
			if s1[i-1] == s2[j-1] {
				cost = 0
			}
			matrix[i][j] = min(
				matrix[i-1][j]+1,
				min(matrix[i][j-1]+1, matrix[i-1][j-1]+cost),
			)
		}
	}

	return matrix[len(s1)][len(s2)]
}

func (s *MatcherService) FindBestMatch(targetName string, candidates []string) *MatchResult {
	var bestMatch *MatchResult

	for _, candidate := range candidates {
		result := s.MatchIPOs(targetName, candidate)
		if result.IsMatch && (bestMatch == nil || result.Confidence > bestMatch.Confidence) {
			bestMatch = result
		}
	}

	if bestMatch == nil {
		return &MatchResult{
			IsMatch:    false,
			Confidence: 0,
			MatchType:  MatchTypeNoMatch,
		}
	}

	return bestMatch
}

func (s *MatcherService) MatchBySlug(sourceSlug, targetSlug string) *MatchResult {
	if sourceSlug == "" || targetSlug == "" {
		return &MatchResult{IsMatch: false, Confidence: 0, MatchType: MatchTypeNoMatch}
	}

	sourceNormalized := s.normalizeSlug(sourceSlug)
	targetNormalized := s.normalizeSlug(targetSlug)

	if sourceNormalized == targetNormalized {
		return &MatchResult{
			IsMatch:          true,
			Confidence:       MinConfidenceExact,
			MatchType:        MatchTypeExact,
			NormalizedSource: sourceNormalized,
			NormalizedTarget: targetNormalized,
		}
	}

	confidence := s.calculateLevenshteinConfidence(sourceNormalized, targetNormalized)

	if confidence >= MinConfidenceFuzzy {
		return &MatchResult{
			IsMatch:          true,
			Confidence:       confidence,
			MatchType:        MatchTypeFuzzy,
			NormalizedSource: sourceNormalized,
			NormalizedTarget: targetNormalized,
		}
	}

	return &MatchResult{
		IsMatch:          false,
		Confidence:       confidence,
		MatchType:        MatchTypeNoMatch,
		NormalizedSource: sourceNormalized,
		NormalizedTarget: targetNormalized,
	}
}

func (s *MatcherService) normalizeSlug(slug string) string {
	normalized := strings.ToLower(slug)
	normalized = strings.ReplaceAll(normalized, "-", " ")
	normalized = strings.ReplaceAll(normalized, "_", " ")
	normalized = strings.TrimSpace(normalized)
	return normalized
}

func (s *MatcherService) NormalizeSlugForMatching(slug string) string {
	return s.normalizeSlug(slug)
}
