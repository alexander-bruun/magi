-- Drop triggers
DROP TRIGGER IF EXISTS update_media_vote_score_delete;
DROP TRIGGER IF EXISTS update_media_vote_score;

-- Drop the votes table
DROP TABLE IF EXISTS votes;