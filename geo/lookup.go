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

// ResolvedLocation is the location, country, and timezone stored for a maintainer.
type ResolvedLocation struct {
	Location *string
	Country  *string
	Timezone *string
}

// ResolveLocation decides the location/country/timezone that should be stored for a new raw
// location observation (from a GitHub profile or a staff edit), given the maintainer's
// previously-stored values:
//
//   - rawLocation is empty: everything is cleared. This is the clearest signal available that
//     a maintainer wants their location removed, so nothing stale should be left behind.
//   - rawLocation derives a country/timezone: all three fields are set to the new values.
//   - rawLocation doesn't derive and is unchanged from previous.Location: location is set (a
//     no-op) and country/timezone carry over unchanged. A parser gap must not destroy data
//     that's still valid.
//   - rawLocation doesn't derive and differs from previous.Location: location is set and
//     country/timezone are cleared. The location changed to something we can't resolve (often
//     a deliberate privacy choice like "Earth"), so a stale derived value must not be kept.
func ResolveLocation(rawLocation string, previous ResolvedLocation) ResolvedLocation {
	trimmed := strings.TrimSpace(rawLocation)
	if trimmed == "" {
		return ResolvedLocation{}
	}
	resolved := ResolvedLocation{Location: &trimmed}
	if country, tz, ok := DeriveCountryAndTimezone(trimmed); ok {
		resolved.Country = &country
		resolved.Timezone = &tz
		return resolved
	}
	if previous.Location != nil && *previous.Location == trimmed {
		resolved.Country = previous.Country
		resolved.Timezone = previous.Timezone
	}
	return resolved
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
