package geo

import "testing"

func TestDeriveCountryAndTimezone(t *testing.T) {
	tests := []struct {
		rawLocation string
		wantCountry string
		wantTZ      string
		wantOK      bool
	}{
		{"Yerevan, Armenia", "AM", "Asia/Yerevan", true},
		{"Madrid, Spain", "ES", "Europe/Madrid", true},
		{"Bengaluru, India", "IN", "Asia/Kolkata", true},
		{"Tokyo, Japan", "JP", "Asia/Tokyo", true},
		{"London, UK", "GB", "Europe/London", true},
		{"United Kingdom", "GB", "Europe/London", true},
		{"São Paulo, Brazil", "BR", "America/Sao_Paulo", true},
		{"Portland, Oregon, USA", "US", "America/New_York", true},
		{"New York, United States", "US", "America/New_York", true},
		{"Berlin, Germany", "DE", "Europe/Berlin", true},
		{"Paris, France", "FR", "Europe/Paris", true},
		{"Amsterdam, Netherlands", "NL", "Europe/Amsterdam", true},
		{"Seoul, South Korea", "KR", "Asia/Seoul", true},
		{"Singapore", "SG", "Asia/Singapore", true},
		{"Dublin, Ireland", "IE", "Europe/Dublin", true},
		{"Warsaw, Poland", "PL", "Europe/Warsaw", true},
		{"Amsterdam, Holland", "NL", "Europe/Amsterdam", true},
		// Should NOT match "IN" inside "China" or "Istanbul"
		{"China", "CN", "Asia/Shanghai", true},
		{"Istanbul, Turkey", "TR", "Europe/Istanbul", true},
		// Should NOT match "US" inside "Russia"
		{"Russia", "RU", "Europe/Moscow", true},
		// Unresolvable inputs
		{"SF", "", "", false},
		{"🌍", "", "", false},
		{"remote", "", "", false},
		{"", "", "", false},
		{"Earth", "", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.rawLocation, func(t *testing.T) {
			country, tz, ok := DeriveCountryAndTimezone(tc.rawLocation)
			if ok != tc.wantOK {
				t.Fatalf("DeriveCountryAndTimezone(%q) ok=%v want %v", tc.rawLocation, ok, tc.wantOK)
			}
			if country != tc.wantCountry {
				t.Errorf("DeriveCountryAndTimezone(%q) country=%q want %q", tc.rawLocation, country, tc.wantCountry)
			}
			if tz != tc.wantTZ {
				t.Errorf("DeriveCountryAndTimezone(%q) tz=%q want %q", tc.rawLocation, tz, tc.wantTZ)
			}
		})
	}
}
