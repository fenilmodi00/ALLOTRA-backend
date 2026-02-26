package services

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/fenilmodi00/ipo-backend/models"
)

var whitespaceRE = regexp.MustCompile(`\s+`)
var emailRE = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
var phoneRE = regexp.MustCompile(`\+?[0-9][0-9\-\s]{7,}[0-9]`)

// ParseCMSContent extracts structured fields from Groww CMS HTML blocks.
func ParseCMSContent(htmlContent string) (*models.GrowwParsedCMS, error) {
	result := &models.GrowwParsedCMS{}
	if strings.TrimSpace(htmlContent) == "" {
		return result, nil
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return nil, err
	}

	result.Objectives = parseObjectivesTable(doc)
	result.LeadManager = parseSectionParagraph(doc, "Lead Manager")
	result.RegistrarDetails = parseRegistrarDetails(doc)
	result.ContactDetails = parseContactDetails(doc)

	return result, nil
}

func parseObjectivesTable(doc *goquery.Document) []models.GrowwObjective {
	var objectives []models.GrowwObjective

	doc.Find("table").Each(func(_ int, table *goquery.Selection) {
		if len(objectives) > 0 {
			return
		}
		head := cleanText(table.Find("tr").First().Text())
		if !strings.Contains(strings.ToLower(head), "purpose") {
			return
		}

		table.Find("tr").Each(func(i int, row *goquery.Selection) {
			if i == 0 {
				return
			}
			cells := row.Find("td")
			if cells.Length() < 3 {
				return
			}
			objectives = append(objectives, models.GrowwObjective{
				Purpose:     cleanText(cells.Eq(0).Text()),
				Amount:      cleanText(cells.Eq(1).Text()),
				Description: cleanText(cells.Eq(2).Text()),
			})
		})
	})

	return objectives
}

func parseSectionParagraph(doc *goquery.Document, sectionKeyword string) string {
	var out string
	needle := strings.ToLower(sectionKeyword)
	doc.Find("h2").Each(func(_ int, h2 *goquery.Selection) {
		if out != "" {
			return
		}
		heading := strings.ToLower(cleanText(h2.Text()))
		if !strings.Contains(heading, needle) {
			return
		}
		next := h2.Next()
		for next.Length() > 0 {
			if goquery.NodeName(next) == "p" {
				out = cleanText(next.Text())
				return
			}
			next = next.Next()
		}
	})
	return out
}

func parseRegistrarDetails(doc *goquery.Document) *models.GrowwRegistrarInfo {
	name := parseSectionParagraph(doc, "Registrar")
	if name == "" {
		return nil
	}

	info := &models.GrowwRegistrarInfo{Name: name}

	doc.Find("h2").Each(func(_ int, h2 *goquery.Selection) {
		if !strings.Contains(strings.ToLower(cleanText(h2.Text())), "registrar") {
			return
		}
		next := h2.Next()
		for next.Length() > 0 {
			if goquery.NodeName(next) == "p" && cleanText(next.Text()) != name {
				full := cleanText(next.Text())
				if email := emailRE.FindString(full); email != "" {
					info.Email = email
				}
				if phone := cleanText(phoneRE.FindString(full)); phone != "" {
					info.Phone = phone
				}
				break
			}
			next = next.Next()
		}
	})

	return info
}

func parseContactDetails(doc *goquery.Document) *models.GrowwContactDetails {
	var details *models.GrowwContactDetails
	doc.Find("h2").Each(func(_ int, h2 *goquery.Selection) {
		if details != nil {
			return
		}
		if !strings.Contains(strings.ToLower(cleanText(h2.Text())), "contact details") {
			return
		}

		next := h2.Next()
		for next.Length() > 0 {
			if goquery.NodeName(next) == "p" {
				full := cleanText(next.Text())
				if full == "" {
					next = next.Next()
					continue
				}
				d := &models.GrowwContactDetails{}
				if email := emailRE.FindString(full); email != "" {
					d.Email = email
					full = cleanText(strings.ReplaceAll(full, email, ""))
				}
				if phone := cleanText(phoneRE.FindString(full)); phone != "" {
					d.Phone = phone
					full = cleanText(strings.ReplaceAll(full, phone, ""))
				}
				d.Address = full
				details = d
				return
			}
			next = next.Next()
		}
	})

	return details
}

func cleanText(input string) string {
	trimmed := strings.TrimSpace(input)
	return whitespaceRE.ReplaceAllString(trimmed, " ")
}
