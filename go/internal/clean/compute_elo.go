package clean

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/kentongelis/match_predictor/go/internal/config"
)

// openEnded is the DateTo of each team's latest rating
var openEnded = time.Date(9999, time.December, 31, 0, 0, 0, 0, time.UTC)

// ComputeElo replays matches in date order and returns every team's rating history. Each team
// gets a row the day before its season's first match (the start-of-season rating) and a row dated
// on each match day holding the rating after that match, so features must only use rows dated strictly
// before a match.
func ComputeElo(matches []Match, p config.Elo) ([]EloRating, error) {
	sorted := slices.Clone(matches)
	slices.SortFunc(sorted, func(a, b Match) int {
		if c := a.Date.Compare(b.Date); c != 0 {
			return c
		}
		return strings.Compare(a.HomeTeamID, b.HomeTeamID)
	})

	ratings := map[string]float64{} // current rating of every team in the current season
	history := map[string][]EloRating{}
	add := func(team string, date time.Time) error {
		rows := history[team]
		if n := len(rows); n > 0 && !rows[n-1].DateFrom.Before(date) {
			return fmt.Errorf("%s has two ratings dated %s", team, date.Format("2006-01-02"))
		}
		history[team] = append(rows, EloRating{TeamID: team, DateFrom: date, Elo: ratings[team]})
		return nil
	}

	for start := 0; start < len(sorted); {
		season := sorted[start].Season
		end := start
		for end < len(sorted) && sorted[end].Season == season {
			end++
		}
		block := sorted[start:end]
		if end < len(sorted) && sorted[end].Season < season {
			return nil, fmt.Errorf("season %s overlaps season %s", sorted[end].Season, season)
		}

		teams := seasonTeams(block)
		if err := startSeason(ratings, teams, p); err != nil {
			return nil, fmt.Errorf("season %s: %w", season, err)
		}
		opening := block[0].Date.AddDate(0, 0, -1)
		for _, team := range teams {
			if err := add(team, opening); err != nil {
				return nil, fmt.Errorf("season %s: %w", season, err)
			}
		}

		for _, m := range block {
			delta := eloDelta(ratings[m.HomeTeamID], ratings[m.AwayTeamID], m.HomeGoals, m.AwayGoals, p)
			ratings[m.HomeTeamID] += delta
			ratings[m.AwayTeamID] -= delta
			if err := add(m.HomeTeamID, m.Date); err != nil {
				return nil, fmt.Errorf("match %s: %w", m.ID, err)
			}
			if err := add(m.AwayTeamID, m.Date); err != nil {
				return nil, fmt.Errorf("match %s: %w", m.ID, err)
			}
		}
		start = end
	}

	var out []EloRating
	for _, team := range slices.Sorted(maps.Keys(history)) {
		rows := history[team]
		for i := range rows {
			if i+1 < len(rows) {
				rows[i].DateTo = rows[i+1].DateFrom.AddDate(0, 0, -1)
			} else {
				rows[i].DateTo = openEnded
			}
		}
		out = append(out, rows...)
	}
	return out, nil
}

// seasonsTeams returns the sorted IDs of every team that plays in block
func seasonTeams(block []Match) []string {
	seen := map[string]bool{}
	for _, m := range block {
		seen[m.HomeTeamID] = true
		seen[m.AwayTeamID] = true
	}
	return slices.Sorted(maps.Keys(seen))
}

// startSeason turns last season's final ratings into this season's starting ratings. Teams
// that left are removed, each arrival gets the leaver's mean, then everyone is pulled toward
// p.Initial. On the first season ratings is empty, so every team starts at p.Initial
func startSeason(ratings map[string]float64, teams []string, p config.Elo) error {
	if len(ratings) == 0 {
		for _, team := range teams {
			ratings[team] = p.Initial
		}
		return nil
	}

	var arrivals []string
	for _, team := range teams {
		if _, ok := ratings[team]; !ok {
			arrivals = append(arrivals, team)
		}
	}
	// Sorted so the floating point sum is the same on every run
	var leavers []string
	for _, team := range slices.Sorted(maps.Keys(ratings)) {
		if !slices.Contains(teams, team) {
			leavers = append(leavers, team)
		}
	}
	if len(leavers) != len(arrivals) {
		return fmt.Errorf("%d teams left but %d arrived", len(leavers), len(arrivals))
	}

	if len(arrivals) > 0 {
		var sum float64
		for _, team := range leavers {
			sum += ratings[team]
			delete(ratings, team)
		}
		mean := sum / float64(len(leavers))
		for _, team := range arrivals {
			ratings[team] = mean
		}
	}

	for team, r := range ratings {
		ratings[team] = p.Initial + (1-p.SeasonRegression)*(r-p.Initial)
	}
	return nil
}

// eloDelta is the points the home team gains from a match. The away team loses the saem amount
func eloDelta(home, away float64, homeGoals, awayGoals int, p config.Elo) float64 {
	expected := 1 / (1 + math.Pow(10, -(home+p.HomeAdvantage-away)/400))

	score := 0.5
	switch {
	case homeGoals > awayGoals:
		score = 1
	case homeGoals < awayGoals:
		score = 0
	}

	g := 1.0
	if p.GoalMargin {
		g = goalMultiplier(homeGoals - awayGoals)
	}
	return p.K * g * (score - expected)
}

// goalMultiplier is where wins by more goals move the ratings further
func goalMultiplier(margin int) float64 {
	if margin < 0 {
		margin = -margin
	}
	switch {
	case margin <= 1:
		return 1
	case margin == 2:
		return 1.5
	default:
		return (11 + float64(margin)) / 8
	}
}
