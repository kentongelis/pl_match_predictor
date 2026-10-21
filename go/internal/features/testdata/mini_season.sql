-- Hand-built fixture for the feature SQL test. Expected values in feature_sql_test.go are worked out from this
-- by hand, so if you change a row here, redo the arithmetic there.
--
-- ars plays 7 matches, alternating home and away, then has one upcoming fixture:
--   #  date        venue  opponent  score  pts
--   1  2020-08-01  H      b         2-0    3
--   2  2020-08-08  A      c         1-1    1
--   3  2020-08-12  H      d         0-1    0   (4 days rest)
--   4  2020-08-22  A      b         3-1    3   (10 days rest)
--   5  2020-08-29  H      c         0-2    0
--   6  2020-09-05  A      d         1-1    1
--   7  2020-09-12  H      b         2-2    1
--   F  2020-09-19  A      c         (fixture, not played)

INSERT INTO matches (match_id, season, date, home_team_id, away_team_id, home_goals, away_goals, result,
                     home_shots, away_shots, home_sot, away_sot) VALUES
    ('2020-21_2020-08-01_ars_b', '2020-21', '2020-08-01', 'ars', 'b',   2, 0, 'H', 10, 5, 4, 1),
    ('2020-21_2020-08-08_c_ars', '2020-21', '2020-08-08', 'c',   'ars', 1, 1, 'D', 8,  9, 3, 3),
    ('2020-21_2020-08-12_ars_d', '2020-21', '2020-08-12', 'ars', 'd',   0, 1, 'A', 12, 6, 2, 2),
    ('2020-21_2020-08-22_b_ars', '2020-21', '2020-08-22', 'b',   'ars', 1, 3, 'A', 7, 11, 2, 5),
    ('2020-21_2020-08-29_ars_c', '2020-21', '2020-08-29', 'ars', 'c',   0, 2, 'A', 9,  8, 1, 4),
    ('2020-21_2020-09-05_d_ars', '2020-21', '2020-09-05', 'd',   'ars', 1, 1, 'D', 6, 10, 2, 3),
    ('2020-21_2020-09-12_ars_b', '2020-21', '2020-09-12', 'ars', 'b',   2, 2, 'D', 14, 7, 6, 3);

INSERT INTO fixtures (match_id, season, date, home_team_id, away_team_id) VALUES
    ('2020-21_2020-09-19_c_ars', '2020-21', '2020-09-19', 'c', 'ars');
