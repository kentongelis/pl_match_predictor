package clean

import (
	"fmt"
	"math"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kentongelis/match_predictor/go/internal/config"
)

var testElo = config.Elo{Initial: 1500, K: 20, HomeAdvantage: 60, SeasonRegression: 0.2, GoalMargin: true}

// eloMatch builds a match from a date like "2014-08-16" and a score like "2-0"
func eloMatch(t *testing.T, season, date, home, away, score string) Match {
	t.Helper()
	d, err := time.Parse("2006-01-02", date)
	if err != nil {
		t.Fatal(err)
	}
	var hg, ag int
	if _, err := fmt.Sscanf(score, "%d-%d", &hg, &ag); err != nil {
		t.Fatal(err)
	}
	return Match{ID: season + "_" + date + "_" + home + "_" + away, Season: season, Date: d,
		HomeTeamID: home, AwayTeamID: away, HomeGoals: hg, AwayGoals: ag}
}

// latest returns each team's open-ended rating
func latest(ratings []EloRating) map[string]float64 {
	out := map[string]float64{}
	for _, r := range ratings {
		if r.DateTo.Equal(openEnded) {
			out[r.TeamID] = r.Elo
		}
	}
	return out
}

func TestEloDeltaWorkedExample(t *testing.T) {
	// learn/06-elo.md section 3.3: Arsenal 1550 host Spurs 1520
	cases := []struct {
		hg, ag int
		want   float64
	}{
		{2, 0, 11.20},
		{1, 1, -2.53},
		{0, 1, -12.53},
	}
	for _, c := range cases {
		got := eloDelta(1550, 1520, c.hg, c.ag, testElo)
		if math.Abs(got-c.want) > 0.01 {
			t.Errorf("%d-%d: delta = %.4f, want %.2f", c.hg, c.ag, got, c.want)
		}
	}
}

func TestGoalMultiplier(t *testing.T) {
	want := map[int]float64{0: 1, 1: 1, -1: 1, 2: 1.5, -2: 1.5, 3: 14.0 / 8, 5: 16.0 / 8}
	for margin, w := range want {
		if got := goalMultiplier(margin); got != w {
			t.Errorf("goalMultiplier(%d) = %v, want %v", margin, got, w)
		}
	}
}

// twoSeasons is a 4-team league: d goes down after 2014-15 and e comes up
func twoSeasons(t *testing.T) []Match {
	return []Match{
		eloMatch(t, "2014-15", "2014-08-16", "a", "b", "2-0"),
		eloMatch(t, "2014-15", "2014-08-16", "c", "d", "1-1"),
		eloMatch(t, "2014-15", "2014-08-23", "b", "c", "0-3"),
		eloMatch(t, "2014-15", "2014-08-23", "d", "a", "1-0"),
		eloMatch(t, "2015-16", "2015-08-08", "a", "e", "4-0"),
		eloMatch(t, "2015-16", "2015-08-08", "b", "c", "1-2"),
	}
}

func TestComputeEloZeroSum(t *testing.T) {
	ratings, err := ComputeElo(twoSeasons(t)[:4], testElo)
	if err != nil {
		t.Fatal(err)
	}
	var sum float64
	for _, r := range latest(ratings) {
		sum += r
	}
	if math.Abs(sum-4*testElo.Initial) > 1e-9 {
		t.Errorf("ratings sum to %v, want %v", sum, 4*testElo.Initial)
	}
}

func TestComputeEloPromotion(t *testing.T) {
	ratings, err := ComputeElo(twoSeasons(t), testElo)
	if err != nil {
		t.Fatal(err)
	}

	var dFinal, eStart float64
	var startSum float64
	opening := time.Date(2015, time.August, 7, 0, 0, 0, 0, time.UTC)
	for _, r := range ratings {
		if r.TeamID == "d" && r.DateTo.Equal(openEnded) {
			dFinal = r.Elo
		}
		if r.DateFrom.Equal(opening) {
			startSum += r.Elo
			if r.TeamID == "e" {
				eStart = r.Elo
			}
		}
	}
	want := testElo.Initial + (1-testElo.SeasonRegression)*(dFinal-testElo.Initial)
	if math.Abs(eStart-want) > 1e-9 {
		t.Errorf("e starts at %v, want d's regressed final rating %v", eStart, want)
	}
	if math.Abs(startSum-4*testElo.Initial) > 1e-9 {
		t.Errorf("2015-16 starting ratings sum to %v, want %v", startSum, 4*testElo.Initial)
	}
}

func TestComputeEloOrderIndependent(t *testing.T) {
	matches := twoSeasons(t)
	want, err := ComputeElo(matches, testElo)
	if err != nil {
		t.Fatal(err)
	}
	r := rand.New(rand.NewPCG(1, 2))
	for range 10 {
		r.Shuffle(len(matches), func(i, j int) { matches[i], matches[j] = matches[j], matches[i] })
		got, err := ComputeElo(matches, testElo)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatal("shuffled input gave different ratings")
		}
	}
}

func TestComputeEloRanges(t *testing.T) {
	ratings, err := ComputeElo(twoSeasons(t), testElo)
	if err != nil {
		t.Fatal(err)
	}
	for i, r := range ratings {
		if r.DateTo.Before(r.DateFrom) {
			t.Errorf("%s %s: date_to before date_from", r.TeamID, r.DateFrom.Format("2006-01-02"))
		}
		last := i+1 == len(ratings) || ratings[i+1].TeamID != r.TeamID
		if last != r.DateTo.Equal(openEnded) {
			t.Errorf("%s %s: open-ended = %v, want %v", r.TeamID, r.DateFrom.Format("2006-01-02"), !last, last)
		}
		if !last && !ratings[i+1].DateFrom.Equal(r.DateTo.AddDate(0, 0, 1)) {
			t.Errorf("%s %s: next row doesn't start the day after date_to", r.TeamID, r.DateFrom.Format("2006-01-02"))
		}
	}
}

func TestComputeEloRejectsBadInput(t *testing.T) {
	cases := map[string]struct {
		matches []Match
		want    string
	}{
		"team plays twice in a day": {
			[]Match{
				eloMatch(t, "2014-15", "2014-08-16", "a", "b", "1-0"),
				eloMatch(t, "2014-15", "2014-08-16", "a", "c", "1-0"),
			},
			"two ratings dated",
		},
		"promoted count mismatch": {
			[]Match{
				eloMatch(t, "2014-15", "2014-08-16", "a", "b", "1-0"),
				eloMatch(t, "2015-16", "2015-08-08", "a", "c", "1-0"),
				eloMatch(t, "2015-16", "2015-08-08", "d", "e", "1-0"),
			},
			"1 teams left but 3 arrived",
		},
	}
	for name, c := range cases {
		_, err := ComputeElo(c.matches, testElo)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want it to contain %q", name, err, c.want)
		}
	}
}
