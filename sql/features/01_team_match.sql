-- 01_team_match.sql: long view, one row per team per match (played) or fixture (upcoming)
--  Most feature files read from this, so training rows and upcoming fixtures share the same code path

CREATE OR REPLACE VIEW team_match AS 
WITH games AS (
    SELECT match_id, season, date, home_team_id, away_team_id,
           home_goals, away_goals, home_shots, away_shots,
           home_sot, away_sot, home_xg, away_xg,
           TRUE AS played
    FROM matches
    UNION ALL
    -- Fixtures have no stats yets, so they get NULLS. Skip any fixture that's already in matches
    SELECT match_id, season, date, home_team_id, away_team_id,
          NULL, NULL, NULL, NULL,
          NULL, NULL, NULL, NULL,
          FALSE
    FROM fixtures
    WHERE match_id NOT IN (SELECT match_id FROM matches)
),
long AS (
    -- Home team's pov
    SELECT match_id, season, date, played,
           home_team_id AS team_id, away_team_id AS opponent_id, TRUE AS is_home,
           home_goals AS goals_for, away_goals AS goals_against,
           home_shots AS shots_for, away_shots AS shots_against,
           home_sot AS sot_for, away_sot AS sot_against,
           home_xg AS xg_for, away_xg AS xg_against
    FROM games
    UNION ALL
    -- Away team's point of view: same columns, sides swapped
    SELECT match_id, season, date, played,
           away_team_id, home_team_id, FALSE,
           away_goals, home_goals,
           away_shots, home_shots,
           away_sot, home_sot,
           away_xg, home_xg
    FROM games
)
SELECT *,
    -- Comparisons with NULL are NULL, so fictures will fall through on every branch and get points = NULL
    CASE WHEN goals_for > goals_against THEN 3
         WHEN goals_for = goals_against THEN 1
         WHEN goals_for < goals_against THEN 0
    END AS points
FROM long;