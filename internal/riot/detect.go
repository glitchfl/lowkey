package riot

import (
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

var versionPattern = regexp.MustCompile(`^release-\d{2}\.\d{2}-shipping-\d+-\d+$`)

// ValidVersion rejects lookalikes such as the external-sessions manifest version
func ValidVersion(v string) bool { return versionPattern.MatchString(v) }

func RegionFromArgs(args []string) string {
	for _, a := range args {
		if v, ok := strings.CutPrefix(a, "-ares-deployment="); ok {
			return strings.ToLower(strings.TrimSpace(v))
		}
	}
	return ""
}

var shards = map[string]string{
	"na": "na", "latam": "na", "br": "na",
	"eu": "eu", "ap": "ap", "kr": "kr", "pbe": "pbe",
}

func Shard(region string) (string, bool) {
	s, ok := shards[region]
	return s, ok
}

// Expiry reads a JWT's exp claim without checking the signature - lowkey only uses it to refresh early
func Expiry(token string) (time.Time, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return time.Time{}, false
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if json.Unmarshal(payload, &claims) != nil || claims.Exp == 0 {
		return time.Time{}, false
	}
	return time.Unix(claims.Exp, 0), true
}
