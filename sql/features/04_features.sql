-- 04_features.sql: final wide table, one rwo per match (played or upcoming). This is what Python reads
-- A real Table that's rebuilt from scratch every run (CREATE OR REPLACE keeps it idempotent)
-- Elo columns join in here once 03_elo_asof.sql exists

CREATE OR REPLACE TABLE features AS
WITH seasons AS (
    -- '2014-15' < '2015-16' as strings, so ranking by season gives 1, 2, 3, ... in order
    SELECT season, DENSE_RANK() OVER (ORDER BY season) AS season_idx
    FROM (SELECT DISTINCT season FROM team_match)
),
team_seasons AS (
    -- Which teams were in the league each season (fixtures count, so the current season is known in August)
    SELECT DISTINCT tm.team_id, s.season_idx
    FROM team_match tm JOIN seasons s USING (season)
),
side AS (
    -- One row per team per match: its form plus whether it came up this season
    SELECT f.*,
           CASE WHEN s.season_idx = 1 THEN NULL          -- no earlier season in the data to compare with
                ELSE prev.team_id IS NULL                -- no match last season means promoted
           END AS is_promoted
    FROM team_form f
    JOIN team_match tm USING (match_id, team_id)
    JOIN seasons s USING (season)
    LEFT JOIN team_seasons prev
           ON prev.team_id = f.team_id AND prev.season_idx = s.season_idx - 1
),
-- Split into the two sides and prefix every feature column, e.g. pts_l5 becomes home_pts_l5
home AS (SELECT match_id, COLUMNS(* EXCLUDE (match_id, team_id, is_home)) AS 'home_\0' FROM side WHERE is_home),
away AS (SELECT match_id, COLUMNS(* EXCLUDE (match_id, team_id, is_home)) AS 'away_\0' FROM side WHERE NOT is_home),
games AS (
    SELECT match_id, season, date, played AS is_played,
           team_id AS home_team_id, opponent_id AS away_team_id
    FROM team_match WHERE is_home
)
SELECT games.*,
       home.* EXCLUDE (match_id),
       away.* EXCLUDE (match_id),
       home_pts_l5 - away_pts_l5 AS diff_pts_l5,
       home_gf_l5 - away_gf_l5 AS diff_gf_l5,
       home_ga_l5 - away_ga_l5 AS diff_ga_l5,
       home_shots_for_l5 - away_shots_for_l5 AS diff_shots_for_l5,
       home_shots_against_l5 - away_shots_against_l5 AS diff_shots_against_l5,
       home_sot_for_l5 - away_sot_for_l5 AS diff_sot_for_l5,
       home_sot_against_l5 - away_sot_against_l5 AS diff_sot_against_l5,
       home_pts_l10 - away_pts_l10 AS diff_pts_l10,
       home_gf_l10 - away_gf_l10 AS diff_gf_l10,
       home_ga_l10 - away_ga_l10 AS diff_ga_l10,
       home_shots_for_l10 - away_shots_for_l10 AS diff_shots_for_l10,
       home_shots_against_l10 - away_shots_against_l10 AS diff_shots_against_l10,
       home_sot_for_l10 - away_sot_for_l10 AS diff_sot_for_l10,
       home_sot_against_l10 - away_sot_against_l10 AS diff_sot_against_l10,
       home_venue_pts_l5 - away_venue_pts_l5 AS diff_venue_pts_l5,
       home_venue_gf_l5 - away_venue_gf_l5 AS diff_venue_gf_l5,
       home_venue_ga_l5 - away_venue_ga_l5 AS diff_venue_ga_l5,
       home_rest_days - away_rest_days AS diff_rest_days,
       m.result                                          -- the label. NULL for upcoming fixtures
FROM games
JOIN home USING (match_id)
JOIN away USING (match_id)
LEFT JOIN matches m USING (match_id);