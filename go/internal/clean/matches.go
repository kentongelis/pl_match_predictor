package clean

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// Match is one played game, shaped like a row of the matches table.
type Match struct {
	ID	string // {season}_{date}_{home}_{away}
	Season string // "2014-15"
	Date time.Time
	HomeTeamID	string
	AwayTeamID	string
	HomeGoals	int
	AwayGoals	int
	Result	string // "H", "D" or "A"
	HomeShots *int
	AwayShots *int
	HomeSOT *int
	AwaySOT *int
	HomeXG *float64
	AwayXG *float64
	OddsH *float64
	OddsD *float64
	OddsA *float64
}

// closingOdds list football-data's closing-odds columns, best first. 
// Uses Pinnacle because it's the sharpest but falls back to market average due to missing recent seasons
var closingOdds = [][3]string{
	{"PSCH", "PSCD", "PSCA"},
	{"AvgCH", "AvgCD", "AvgCA"},
}

var requiredColumns = []string{"Date", "HomeTeam", "AwayTeam", "FTHG", "FTAG", "FTR"}

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// SeasonLabel turns a start year into the label stored in the database: 2014 -> "2014-15"
func SeasonLabel(start int) string {
        return fmt.Sprintf("%d-%02d", start, (start+1)%100)
}

// ParseFootballData reads one football-data.co.uk season CSV. Columns are found by header name because thier 
// positions change between seasons. It checks every row and reports all bad rows together, so one run shows 
// every unmapped team name
func ParseFootballData(r io.Reader, season int, teams *TeamMap) ([]Match, error) {
	br := bufio.NewReader(r)
	if b, err := br.Peek(len(utf8BOM)); err == nil && bytes.Equal(b, utf8BOM) {
		br.Discard(len(utf8BOM)) // some files start with a byte-order mark
	}

	cr := csv.NewReader(br)
	cr.FieldsPerRecord = -1 // every row might not have the same number of fields

	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	col := make(map[string]int, len(header))
	for i, name := range header {
		col[strings.TrimSpace(name)] = i
	}
	for _, name := range requiredColumns {
		if _, ok := col[name]; !ok {
			return nil, fmt.Errorf("missing column %q", name)
		}
	}

	label := SeasonLabel(season)
	var matches []Match
	var errs []error
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		row := &rowReader{rec: rec, col: col}
		if row.str("HomeTeam") == "" {
			continue // blank padding row
		}

		m, err := parseMatch(row, label, teams)
		if err != nil {
			line, _ := cr.FieldPos(0)
			errs = append(errs, fmt.Errorf("line %d: %w", line, err))
			continue
		}
		matches = append(matches, m)
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return matches, nil
}

func parseMatch(row *rowReader, season string, teams *TeamMap) (Match, error) {
	home, homeErr := teams.Lookup(row.str("HomeTeam"))
	away, awayErr := teams.Lookup(row.str("AwayTeam"))
	if err := errors.Join(homeErr, awayErr); err != nil {
		return Match{}, err
	}
	if home.ID == away.ID {
		return Match{}, fmt.Errorf("%s plays itself", home.ID)
	}

	m := Match{
		Season: season,
		Date: row.date("Date"),
		HomeTeamID: home.ID,
		AwayTeamID: away.ID,
		HomeGoals: row.int("FTHG"),
		AwayGoals: row.int("FTAG"),
		Result: row.str("FTR"),
		HomeShots: row.optInt("HS"),
		AwayShots: row.optInt("AS"),
		HomeSOT: row.optInt("HST"),
		AwaySOT: row.optInt("AST"),
		HomeXG: row.optFloat("HxG"),
		AwayXG: row.optFloat("AxG"),
	}
	for _, cols := range closingOdds {
		if m.OddsH, m.OddsD, m.OddsA = row.odds(cols); m.OddsH != nil {
			break
		}
	}
	if row.err != nil {
		return Match{}, row.err
	}

	if want := resultFor(m.HomeGoals, m.AwayGoals); m.Result != want {
		return Match{}, fmt.Errorf("FTR is %q but the score %d-%d means %q", m.Result, m.HomeGoals, m.AwayGoals, want)
	}
	m.ID = fmt.Sprintf("%s_%s_%s_%s", season, m.Date.Format("2006-01-02"), home.ID, away.ID)
	return m, nil
}

func resultFor(homeGoals, awayGoals int) string {
	switch {
	case homeGoals > awayGoals:
		return "H"
	case homeGoals < awayGoals:
		return "A"
	default:
		return "D"
	}
}

// rowReader reads typed values from one CSV record by column name. The first pause failure is saved in err 
// and later calls return zero values, so parseMatch can read every field and check for an error once at the end
type rowReader struct {
	rec []string
	col map[string]int
	err error
}

// str returns the trimmed value, or "" if the column is absent from this file
func (r *rowReader) str(name string) string {
	i, ok := r.col[name]
	if !ok || i >= len(r.rec) {
		return ""
	}
	return strings.TrimSpace(r.rec[i])
}

func (r *rowReader) fail(name, value string, err error) {
	if r.err == nil {
		r.err = fmt.Errorf("column %s = %q: %w", name, value, err)
	}
}

func (r *rowReader) int(name string) int {
	s := r.str(name)
	n, err := strconv.Atoi(s)
	if err != nil {
		r.fail(name, s, err)
	}
	return n
}

// date accepts both formats football-data has used: 16/08/14 and 16/08/2014
func (r *rowReader) date(name string) time.Time {
	s := r.str(name)
	for _, layout := range []string{"02/01/2006", "02/01/06"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	r.fail(name, s, errors.New("not a dd/mm/yy or dd/mm/yyyy date"))
	return time.Time{}
}

// optInt returns nil when the value is empty or the column doesn't exist
func (r *rowReader) optInt(name string) *int {
	s := r.str(name)
	if s == "" {
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		r.fail(name, s, err)
		return nil
	}
	return &n
}

func (r *rowReader) optFloat(name string) *float64 {
	s := r.str(name)
	if s == "" {
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		r.fail(name, s, err)
		return nil
	}
	return &f
}

// odds returns all three prices, or three nils if any of them is missing
func (r *rowReader) odds(cols [3]string) (h, d, a *float64) {
	h, d, a = r.optFloat(cols[0]), r.optFloat(cols[1]), r.optFloat(cols[2])
	if h == nil || d == nil || a == nil {
		return nil, nil, nil
	}
	return h, d, a
}