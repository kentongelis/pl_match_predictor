package clean

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

// testdata/clubelo_forest.csv is hand-built in ClubElo's format (the API was down when this was written).
// It uses "Forest", ClubElo's spelling, and has rows on both sides of the since cutoff

func TestParseClubElo(t *testing.T) {
	f, err := os.Open("testdata/clubelo_forest.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	since := time.Date(2014, time.January, 1, 0, 0, 0, 0, time.UTC)
	ratings, err := ParseClubElo(f, testTeams(t), since)
	if err != nil {
		t.Fatalf("ParseClubElo: %v", err)
	}

	if len(ratings) != 3 {
		t.Fatalf("got %d ratings, want 3 (the two that ended before 2014 are dropped)", len(ratings))
	}
	got := ratings[0]
	want := EloRating{
		TeamID:   "nottm_forest",
		DateFrom: time.Date(2014, time.January, 1, 0, 0, 0, 0, time.UTC),
		DateTo:   time.Date(2014, time.August, 9, 0, 0, 0, 0, time.UTC),
		Elo:      1490.03,
	}
	if got != want {
		t.Errorf("first rating = %+v, want %+v", got, want)
	}
}

func TestParseClubEloUnknownTeam(t *testing.T) {
	csv := "Rank,Club,Country,Level,Elo,From,To\nNone,Atlantis,ENG,1,1500,2020-01-01,2020-01-05\n"
	_, err := ParseClubElo(strings.NewReader(csv), testTeams(t), time.Time{})
	if !errors.Is(err, ErrUnknownTeam) {
		t.Fatalf("err = %v, want ErrUnknownTeam", err)
	}
}
