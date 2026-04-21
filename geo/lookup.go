package geo

import (
	_ "embed"
	"encoding/json"
	"sort"
	"strings"
	"unicode"
)

//go:embed countries.json
var countriesJSON []byte

type countryEntry struct {
	Name    string   `json:"name"`
	Aliases []string `json:"aliases"`
	ISO     string   `json:"iso"`
	TZ      string   `json:"tz"`
}

var countries []countryEntry

func init() {
	if err := json.Unmarshal(countriesJSON, &countries); err != nil {
		panic("geo: failed to parse countries.json: " + err.Error())
	}
	// Sort: entries with longer aliases first so we match "United States" before "US".
	sort.SliceStable(countries, func(i, j int) bool {
		return maxAliasLen(countries[i].Aliases) > maxAliasLen(countries[j].Aliases)
	})
}

func maxAliasLen(aliases []string) int {
	max := 0
	for _, a := range aliases {
		if len(a) > max {
			max = len(a)
		}
	}
	return max
}

// DeriveCountryAndTimezone returns the ISO-3166 alpha-2 country code and IANA
// timezone for rawLocation using a word-boundary substring match against the
// curated alias list. Returns ok=false when no match is found.
func DeriveCountryAndTimezone(rawLocation string) (country, tz string, ok bool) {
	if strings.TrimSpace(rawLocation) == "" {
		return
	}
	lower := strings.ToLower(rawLocation)
	for _, entry := range countries {
		for _, alias := range entry.Aliases {
			if matchesWord(lower, strings.ToLower(alias)) {
				return entry.ISO, entry.TZ, true
			}
		}
	}
	return
}

// matchesWord checks that word appears in s at a word boundary (i.e. not
// adjacent to a letter or digit on either side).
func matchesWord(s, word string) bool {
	if word == "" {
		return false
	}
	idx := 0
	for {
		pos := strings.Index(s[idx:], word)
		if pos < 0 {
			return false
		}
		abs := idx + pos
		// left boundary
		if abs > 0 {
			r := rune(s[abs-1])
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				idx = abs + 1
				continue
			}
		}
		// right boundary
		end := abs + len(word)
		if end < len(s) {
			r := rune(s[end])
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				idx = abs + 1
				continue
			}
		}
		return true
	}
}
