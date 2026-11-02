package riot

import "strings"

var podNames = map[string]string{
	"eu-gp-frankfurt-1":     "Frankfurt 1",
	"eu-gp-frankfurt-2":     "Frankfurt 2",
	"eu-gp-frankfurt-aws-2": "Frankfurt 2",
	"eu-gp-london-1":        "London",
	"eu-gp-madrid-1":        "Madrid",
	"eu-gp-paris-1":         "Paris",
	"eu-gp-warsaw-1":        "Warsaw",
	"eu-gp-stockholm-1":     "Stockholm",
	"eu-gp-stockholm-aws-1": "Stockholm",
	"eu-gp-istanbul-1":      "Istanbul",
	"us-gp-virginia-1":      "Virginia 1",
	"us-gp-virginia-2":      "Virginia 2",
	"us-gp-california-1":    "California",
	"us-gp-texas-1":         "Texas",
	"us-gp-chicago-1":       "Chicago",
	"us-gp-oregon-1":        "Oregon",
	"us-gp-atlanta-1":       "Atlanta",
	"ap-gp-tokyo-1":         "Tokyo",
	"ap-gp-singapore-1":     "Singapore",
	"ap-gp-singapore-2":     "Singapore 2",
	"ap-gp-sydney-1":        "Sydney",
	"ap-gp-sydney-2":        "Sydney 2",
	"ap-gp-mumbai-1":        "Mumbai",
	"ap-gp-seoul-1":         "Seoul",
	"ap-gp-hongkong-1":      "Hong Kong",
	"latam-gp-santiago-1":   "Santiago",
	"latam-gp-mexico-1":     "Mexico City",
	"latam-gp-bogota-1":     "Bogota",
	"br-gp-saopaulo-1":      "Sao Paulo",
	"kr-gp-seoul-1":         "Seoul",
}

// PodName turns a game pod ID like aresriot.aws-euc1-prod.eu-gp-frankfurt-1 into the city Riot calls it -
// a pod missing from the list is named after the city in its ID
func PodName(id string) string {
	slug := strings.TrimSpace(id)
	if i := strings.LastIndex(slug, "."); i >= 0 {
		slug = slug[i+1:]
	}
	if name, ok := podNames[slug]; ok {
		return name
	}
	parts := strings.Split(slug, "-")
	if len(parts) < 3 || parts[1] != "gp" {
		return slug
	}
	city, n := parts[2:], ""
	if last := city[len(city)-1]; strings.Trim(last, "0123456789") == "" {
		city, n = city[:len(city)-1], last
	}
	var words []string
	for _, w := range city {
		if w != "" && w != "aws" && w != "awsedge" {
			words = append(words, strings.ToUpper(w[:1])+w[1:])
		}
	}
	if len(words) == 0 {
		return slug
	}
	name := strings.Join(words, " ")
	if n != "" && n != "1" {
		name += " " + n
	}
	return name
}
