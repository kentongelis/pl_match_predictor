-- 02_rolling_form.sql: pre-match form for every team_match row
-- Every window ends at 1 PRECEDING, so a row never sees its own result (no leakage)
-- xG is left out until there's an xG source; averaging an all-NULL column would just give NULL

CREATE OR REPLACE VIEW team_form AS
SELECT match_id, team_id, is_home,
       AVG(points)        OVER last5  AS pts_l5,
       AVG(goals_for)     OVER last5  AS gf_l5,
       AVG(goals_against) OVER last5  AS ga_l5,
       AVG(shots_for)     OVER last5  AS shots_for_l5,
       AVG(shots_against) OVER last5  AS shots_against_l5,
       AVG(sot_for)       OVER last5  AS sot_for_l5,
       AVG(sot_against)   OVER last5  AS sot_against_l5,
       AVG(points)        OVER last10 AS pts_l10,
       AVG(goals_for)     OVER last10 AS gf_l10,
       AVG(goals_against) OVER last10 AS ga_l10,
       AVG(shots_for)     OVER last10 AS shots_for_l10,
       AVG(shots_against) OVER last10 AS shots_against_l10,
       AVG(sot_for)       OVER last10 AS sot_for_l10,
       AVG(sot_against)   OVER last10 AS sot_against_l10,
       -- Venue form: home games only for the home team, away games only for the away team
       AVG(points)        OVER venue5 AS venue_pts_l5,
       AVG(goals_for)     OVER venue5 AS venue_gf_l5,
       AVG(goals_against) OVER venue5 AS venue_ga_l5,
       -- Subtracting two DATEs gives whole days. NULL for a team's first game in the data
       date - LAG(date) OVER (PARTITION BY team_id ORDER BY date) AS rest_days
FROM team_match
WINDOW
    last5  AS (PARTITION BY team_id ORDER BY date ROWS BETWEEN 5 PRECEDING AND 1 PRECEDING),
    last10 AS (PARTITION BY team_id ORDER BY date ROWS BETWEEN 10 PRECEDING AND 1 PRECEDING),
    venue5 AS (PARTITION BY team_id, is_home ORDER BY date ROWS BETWEEN 5 PRECEDING AND 1 PRECEDING);