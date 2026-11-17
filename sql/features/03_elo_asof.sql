-- 03_elo_asof.sql: each team's Elo going into every team_match row (played or upcoming)
-- An elo row dated on match day already includes that day's result, so the join takes the latest
-- row dated strictly BEFORE the match. Changing > to >= here would leak the result into the feature

CREATE OR REPLACE VIEW team_elo as
SELECT tm.match_id, tm.team_id, e.elo
FROM team_match tm
ASOF LEFT JOIN elo e
    ON tm.team_id = e.team_id
    AND tm.date > e.date_from;