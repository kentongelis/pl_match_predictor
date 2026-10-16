package clean

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

// The files in testdata/ are unedited excerpts of real football-data CSVs:
//   - footballdata_1415.csv: 2-digit years, no Time column, Pinnacle odds, a blank padding row
//   - footballdata_2526.csv: BOM, Time column, one row without Pinnacle closing odds
//   - footballdata_2627.csv: BOM, xG columns, no Pinnacle columns at all

func testTeams(t *testing.T) *TeamMap {
	t.Helper()
	m, err := LoadTeamMap("../../../data/mappings/team_names.csv")
	if err != nil {
		t.Fatalf("LoadTeamMap: %v", err)
	}
	return m
}

func parseFile(t *testing.T, path string, season int) []Match {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()

	matches, err := ParseFootballData(f, season, testTeams(t))
	if err != nil {
		t.Fatalf("ParseFootballData(%s): %v", path, err)
	}
	return matches
}

func TestSeasonLabel(t *testing.T) {
	cases := map[int]string{2014: "2014-15", 2009: "2009-10", 2099: "2099-00"}
	for start, want := range cases {
		if got := SeasonLabel(start); got != want {
			t.Errorf("SeasonLabel(%d) = %q, want %q", start, got, want)
		}
	}
}

func TestParseOldFormat(t *testing.T) {
	matches := parseFile(t, "testdata/footballdata_1415.csv", 2014)

	if len(matches) != 3 {
		t.Fatalf("got %d matches, want 3 (blank row skipped)", len(matches))
	}
	m := matches[0]
	if m.ID != "2014-15_2014-08-16_arsenal_crystal_palace" {
		t.Errorf("ID = %q", m.ID)
	}
	if m.Season != "2014-15" || !m.Date.Equal(time.Date(2014, 8, 16, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("Season, Date = %q, %v", m.Season, m.Date)
	}
	if m.HomeGoals != 2 || m.AwayGoals != 1 || m.Result != "H" {
		t.Errorf("score = %d-%d %s, want 2-1 H", m.HomeGoals, m.AwayGoals, m.Result)
	}
	checkInt(t, "HomeShots", m.HomeShots, 14)
	checkInt(t, "AwayShots", m.AwayShots, 4)
	checkInt(t, "HomeSOT", m.HomeSOT, 6)
	checkInt(t, "AwaySOT", m.AwaySOT, 2)
	checkOdds(t, m, 1.29, 5.9, 12.75) // Pinnacle closing
	if m.HomeXG != nil || m.AwayXG != nil {
		t.Error("xG should be nil: 2014-15 has no xG columns")
	}

	if matches[1].Result != "D" || matches[2].Result != "A" {
		t.Errorf("results = %s, %s, want D, A", matches[1].Result, matches[2].Result)
	}
}

func TestParseOddsFallback(t *testing.T) {
	matches := parseFile(t, "testdata/footballdata_2526.csv", 2025)

	if len(matches) != 2 {
		t.Fatalf("got %d matches, want 2", len(matches))
	}
	if matches[0].ID != "2025-26_2025-08-15_liverpool_bournemouth" {
		t.Errorf("ID = %q (BOM not stripped?)", matches[0].ID)
	}
	checkOdds(t, matches[0], 1.29, 6.55, 9.75) // Pinnacle closing present
	checkOdds(t, matches[1], 3.01, 3.63, 2.23) // Pinnacle blank -> market average closing
}

func TestParseNewFormatWithXG(t *testing.T) {
	matches := parseFile(t, "testdata/footballdata_2627.csv", 2026)

	if len(matches) != 3 {
		t.Fatalf("got %d matches, want 3", len(matches))
	}
	m := matches[0]
	if m.ID != "2026-27_2026-08-21_arsenal_coventry" {
		t.Errorf("ID = %q", m.ID)
	}
	checkFloat(t, "HomeXG", m.HomeXG, 1.88)
	checkFloat(t, "AwayXG", m.AwayXG, 0.2)
	checkInt(t, "HomeShots", m.HomeShots, 20)
	checkOdds(t, m, 1.2, 6.72, 14) // no Pinnacle columns -> market average closing
}

// header is a minimal football-data header for building bad rows by hand.
const header = "Date,HomeTeam,AwayTeam,FTHG,FTAG,FTR\n"

func TestParseReportsAllUnknownTeams(t *testing.T) {
	input := header +
		"16/08/14,Arsenal,Man Utd,1,0,H\n" +
		"17/08/14,Spurs,Chelsea,0,0,D\n"

	_, err := ParseFootballData(strings.NewReader(input), 2014, testTeams(t))
	if !errors.Is(err, ErrUnknownTeam) {
		t.Fatalf("error = %v, want ErrUnknownTeam", err)
	}
	// Both bad names in one error, so one run shows everything to add to team_names.csv.
	for _, name := range []string{"Man Utd", "Spurs"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error doesn't mention %q: %v", name, err)
		}
	}
}

func TestParseRejectsBadRows(t *testing.T) {
	cases := map[string]string{
		"result disagrees with score": "16/08/14,Arsenal,Chelsea,2,1,A\n",
		"team plays itself":           "16/08/14,Arsenal,Arsenal,1,0,H\n",
		"bad date":                    "2014-08-16,Arsenal,Chelsea,1,0,H\n",
		"missing goals":               "16/08/14,Arsenal,Chelsea,,0,H\n",
		"non-numeric shots":           "16/08/14,Arsenal,Chelsea,1,0,H,lots\n",
	}
	for name, row := range cases {
		t.Run(name, func(t *testing.T) {
			h := header
			if name == "non-numeric shots" {
				h = "Date,HomeTeam,AwayTeam,FTHG,FTAG,FTR,HS\n"
			}
			if _, err := ParseFootballData(strings.NewReader(h+row), 2014, testTeams(t)); err == nil {
				t.Error("expected an error, got nil")
			}
		})
	}
}

func TestParseMissingColumn(t *testing.T) {
	input := "Date,HomeTeam,AwayTeam,FTHG,FTAG\n16/08/14,Arsenal,Chelsea,1,0\n"
	if _, err := ParseFootballData(strings.NewReader(input), 2014, testTeams(t)); err == nil {
		t.Error("expected an error for missing FTR column, got nil")
	}
}

func checkInt(t *testing.T, field string, got *int, want int) {
	t.Helper()
	if got == nil || *got != want {
		t.Errorf("%s = %v, want %d", field, deref(got), want)
	}
}

func checkFloat(t *testing.T, field string, got *float64, want float64) {
	t.Helper()
	if got == nil || *got != want {
		t.Errorf("%s = %v, want %v", field, deref(got), want)
	}
}

func checkOdds(t *testing.T, m Match, h, d, a float64) {
	t.Helper()
	checkFloat(t, "OddsH", m.OddsH, h)
	checkFloat(t, "OddsD", m.OddsD, d)
	checkFloat(t, "OddsA", m.OddsA, a)
}

// deref prints nil pointers as "nil" instead of an address.
func deref[T any](p *T) any {
	if p == nil {
		return "nil"
	}
	return *p
}
