package services

import "testing"

func TestNormalizeChittorgarhLogoURL(t *testing.T) {
	t.Parallel()

	service := NewUtilityService()

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty input",
			input:    "",
			expected: "",
		},
		{
			name:     "filename only",
			input:    "yaap-ipo-logo.jpg",
			expected: "https://www.chittorgarh.net/images/ipo/yaap-ipo-logo.jpg",
		},
		{
			name:     "relative path with images prefix",
			input:    "images/ipo/yaap-ipo-logo.jpg",
			expected: "https://www.chittorgarh.net/images/ipo/yaap-ipo-logo.jpg",
		},
		{
			name:     "relative path with leading slash",
			input:    "/images/ipo/yaap-ipo-logo.jpg",
			expected: "https://www.chittorgarh.net/images/ipo/yaap-ipo-logo.jpg",
		},
		{
			name:     "absolute url",
			input:    "https://www.chittorgarh.net/images/ipo/yaap-ipo-logo.jpg",
			expected: "https://www.chittorgarh.net/images/ipo/yaap-ipo-logo.jpg",
		},
		{
			name:     "protocol relative",
			input:    "//www.chittorgarh.net/images/ipo/yaap-ipo-logo.jpg",
			expected: "https://www.chittorgarh.net/images/ipo/yaap-ipo-logo.jpg",
		},
		{
			name:     "raw domain path",
			input:    "www.chittorgarh.net/images/ipo/yaap-ipo-logo.jpg",
			expected: "https://www.chittorgarh.net/images/ipo/yaap-ipo-logo.jpg",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			actual := service.NormalizeChittorgarhLogoURL(testCase.input)
			if actual != testCase.expected {
				t.Fatalf("NormalizeChittorgarhLogoURL(%q) = %q, expected %q", testCase.input, actual, testCase.expected)
			}
		})
	}
}
