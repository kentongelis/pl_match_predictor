// Package clean turns raw source files into rows in DuckDB. Every team name goes through TeamMap, so a name
// nobody has mapped stops the load instead of quietly creating a duplicate team
package clean

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// ErrUnknownTeam measn a source used a team name that isn't in team_names.csv
var ErrUnknownTeam = errors.New("unknown team name")

// Team is a canonical club. ID is the slug used in match_ids; Name is for display
type Team struct {
	ID string
	Name string
}

// TeamMap resolves each source's spelling ("Man City, "Nott'm Forrest") to one canonical Team
type TeamMap struct {
	byAlias map[string]Team
	byID map[string]Team
}

// LoadTeamMap reads team_names.csv, whose columns are alias,team_id,name
func LoadTeamMap(path string) (*TeamMap, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("load team map: %w", err)
	}
	defer f.Close()

	m, err := parseTeamMap(f)
	if err != nil {
		return nil, fmt.Errorf("load teamp map %s: %w", path, err)
	}
	return m, nil
}

// parseTeamMap takes an io.Reader so tests can pass a string
func parseTeamMap(r io.Reader) (*TeamMap, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = 3 // every row must have exactly alias,team_id,name

	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	if strings.Join(header, ",") != "alias,team_id,name" {
		return nil, fmt.Errorf("header is %q, want alias,team_id,name", header)
	}

	m := &TeamMap{byAlias: map[string]Team{}, byID: map[string]Team{}}
	idByName := map[string]string{} // so two team_ids can't share a name
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err // csv errors already include this line number
		}
		line, _ := cr.FieldPos(0)
		alias := strings.TrimSpace(rec[0])
		id := strings.TrimSpace(rec[1])
		name := strings.TrimSpace(rec[2])

		if alias == "" || id == "" || name == "" {
			return nil, fmt.Errorf("line %d: empty field", line)
		}
		if _, dup := m.byAlias[alias]; dup {
			return nil, fmt.Errorf("line %d: alias %q is listed twice", line, alias)		
		}
		if t, ok := m.byID[id]; ok && t.Name != name {
			return nil, fmt.Errorf("line %d: team_id %q has two name, %q and %q", line, id, t.Name, name)
		}
		if other, ok := idByName[name]; ok && other != id {
			return nil, fmt.Errorf("line %d: name %q belongs to both %q and %q", line, name, other, id)
		}

		t := Team{ID: id, Name: name}
		m.byAlias[alias] = t
		m.byID[id] = t
		idByName[name] = id
	}
	return m, nil
}

// Lookup returns the canonical team for alias. An unmapped name is an error, never a guess
func (m *TeamMap) Lookup(alias string) (Team, error) {
	t, ok := m.byAlias[strings.TrimSpace(alias)]
	if !ok {
		return Team{}, fmt.Errorf("%w: %q (add it to data/mappings/team_names.csv)", ErrUnknownTeam, alias)
	}
	return t, nil
}

// Teams returns each canonical team once, sorted by ID, for loading the teams table
func (m *TeamMap) Teams() []Team {
	teams := make([]Team, 0, len(m.byID))
	for _, t := range m.byID {
		teams = append(teams, t)
	}
	sort.Slice(teams, func(i, j int) bool { return teams[i].ID < teams[j].ID})
	return teams
}