-- +goose Up
-- board_lists holds six compile-time constant rows from sync.DefaultBoardLists, so a
-- uniqueness invariant on position buys nothing. It did force ReplaceBoard to shift
-- every list by a +1000 offset before upserting, purely to dodge transient collisions
-- during the rewrite.
DROP INDEX board_lists_position_unique;
CREATE INDEX board_lists_position_idx ON board_lists (position, key);

-- +goose Down
DROP INDEX board_lists_position_idx;
CREATE UNIQUE INDEX board_lists_position_unique ON board_lists (position);
