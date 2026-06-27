-- Migration: Drop review_maps table (ReviewMap feature removed)
-- Run manually against the database before deploying the updated code.

DROP TABLE IF EXISTS review_maps;
