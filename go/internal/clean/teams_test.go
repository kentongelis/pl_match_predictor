package clean

import (
	"errors"
	"strings"
	"testing"
)

const sampleMap = `alias,team_id,name
Man City,man_city,Manchester City
Manchester City,man_city,Manchester City
Arsenal,arsenal,Arsenal
`

func TestLookup(t *testing.T) {
	m, err := parseTeamMap(strings.NewReader(sampleMap))
	if err != nil {
		t.Fatalf("parseTeamMap: %v", err)
	}

	// Two spellings, one team.
	for _, alias := range []string{"Man City", "Manchester City", " Man City "} {
		got, err := m.Lookup(alias)
		if err != nil {
			t.Fatalf("Lookup(%q): %v", alias, err)
		}
		if got != (Team{ID: "man_city", Name: "Manchester City"}) {
			t.Errorf("Lookup(%q) = %+v", alias, got)
		}
	}
}

func TestLookupUnknownFails(t *testing.T) {
	m, err := parseTeamMap(strings.NewReader(sampleMap))
	if err != nil {
		t.Fatalf("parseTeamMap: %v", err)
	}
	_, err = m.Lookup("Man Utd")
	if !errors.Is(err, ErrUnknownTeam) {
		t.Errorf("Lookup(unknown) error = %v, want ErrUnknownTeam", err)
	}
}

func TestTeamsDeduplicatedAndSorted(t *testing.T) {
	m, err := parseTeamMap(strings.NewReader(sampleMap))
	if err != nil {
		t.Fatalf("parseTeamMap: %v", err)
	}
	teams := m.Teams()
	if len(teams) != 2 || teams[0].ID != "arsenal" || teams[1].ID != "man_city" {
		t.Errorf("Teams() = %+v, want arsenal then man_city", teams)
	}
}

func TestParseTeamMapRejectsBadFiles(t *testing.T) {
	cases := map[string]string{
		"wrong header":    "name,id,alias\nArsenal,arsenal,Arsenal\n",
		"empty field":     "alias,team_id,name\nArsenal,,Arsenal\n",
		"missing column":  "alias,team_id,name\nArsenal,arsenal\n",
		"duplicate alias": "alias,team_id,name\nSpurs,tottenham,Tottenham\nSpurs,tottenham,Tottenham\n",
		"id with 2 names": "alias,team_id,name\nSpurs,tottenham,Tottenham\nTottenham,tottenham,Tottenham Hotspur\n",
		"name with 2 ids": "alias,team_id,name\nSpurs,spurs,Tottenham\nTottenham,tottenham,Tottenham\n",
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := parseTeamMap(strings.NewReader(input)); err == nil {
				t.Error("expected an error, got nil")
			}
		})
	}
}

// The committed mapping file must always parse.
func TestRealTeamNamesFile(t *testing.T) {
	m, err := LoadTeamMap("../../../data/mappings/team_names.csv")
	if err != nil {
		t.Fatalf("LoadTeamMap: %v", err)
	}
	if len(m.Teams()) == 0 {
		t.Error("team_names.csv has no teams")
	}
}
