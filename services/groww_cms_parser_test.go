package services

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseCMSContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		html             string
		expectObjectives int
		expectLead       string
		expectRegName    string
		expectRegPhone   string
		expectRegEmail   string
		expectAddress    string
		expectPhone      string
		expectEmail      string
	}{
		{
			name: "extracts structured sections",
			html: `<h2>Objective of Demo IPO</h2>
<table>
  <tr><td><strong>Purpose / Objective</strong></td><td><strong>Approx. Amount (₹ Crore)</strong></td><td><strong>Description</strong></td></tr>
  <tr><td>Solar Plant</td><td>7.85</td><td>Set up solar plant.</td></tr>
</table>
<h2>Demo IPO Registrar</h2>
<p>Bigshare Services Pvt.Ltd.</p>
<p>+91-22-6263 8200 ipo@bigshareonline.com</p>
<h2>Demo IPO Lead Manager</h2>
<p>Interactive Financial Services Ltd.</p>
<h2>Demo IPO Contact Details</h2>
<p>566P1 Umwada Road Rajkot Gujarat 360311 91 75100 12200 cs@example.com</p>`,
			expectObjectives: 1,
			expectLead:       "Interactive Financial Services Ltd.",
			expectRegName:    "Bigshare Services Pvt.Ltd.",
			expectRegPhone:   "+91-22-6263 8200",
			expectRegEmail:   "ipo@bigshareonline.com",
			expectAddress:    "566P1 Umwada Road Rajkot Gujarat",
			expectPhone:      "360311 91 75100 12200",
			expectEmail:      "cs@example.com",
		},
		{
			name: "empty html",
			html: "",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			parsed, err := ParseCMSContent(tc.html)
			require.NoError(t, err)
			require.NotNil(t, parsed)

			require.Len(t, parsed.Objectives, tc.expectObjectives)
			require.Equal(t, tc.expectLead, parsed.LeadManager)

			if tc.expectRegName == "" {
				require.Nil(t, parsed.RegistrarDetails)
			} else {
				require.NotNil(t, parsed.RegistrarDetails)
				require.Equal(t, tc.expectRegName, parsed.RegistrarDetails.Name)
				require.Equal(t, tc.expectRegPhone, parsed.RegistrarDetails.Phone)
				require.Equal(t, tc.expectRegEmail, parsed.RegistrarDetails.Email)
			}

			if tc.expectAddress == "" {
				require.Nil(t, parsed.ContactDetails)
			} else {
				require.NotNil(t, parsed.ContactDetails)
				require.Equal(t, tc.expectAddress, parsed.ContactDetails.Address)
				require.Equal(t, tc.expectPhone, parsed.ContactDetails.Phone)
				require.Equal(t, tc.expectEmail, parsed.ContactDetails.Email)
			}
		})
	}
}
