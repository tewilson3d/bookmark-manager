-- name: CreateBookmark :one
INSERT INTO bookmarks (url, title, description, summary, source_type, favicon_url, image_url, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
RETURNING *;

-- name: GetBookmark :one
SELECT * FROM bookmarks WHERE id = ?;

-- name: GetBookmarkByURL :one
SELECT * FROM bookmarks WHERE url = ?;

-- name: ListBookmarks :many
SELECT * FROM bookmarks ORDER BY created_at DESC LIMIT ? OFFSET ?;

-- name: ListBookmarksBySource :many
SELECT * FROM bookmarks WHERE source_type = ? ORDER BY created_at DESC LIMIT ? OFFSET ?;

-- name: UpdateBookmark :one
UPDATE bookmarks SET
    title = ?,
    description = ?,
    summary = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: UpdateBookmarkAnalysis :one
UPDATE bookmarks SET
    summary = ?,
    keywords = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: DeleteBookmark :exec
DELETE FROM bookmarks WHERE id = ?;

-- name: SearchBookmarksFTS :many
-- Note: FTS search is done via raw SQL in the server code
SELECT * FROM bookmarks 
WHERE title LIKE ? OR description LIKE ? OR summary LIKE ?
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: CountBookmarks :one
SELECT COUNT(*) FROM bookmarks;

-- name: CountBookmarksBySource :one
SELECT COUNT(*) FROM bookmarks WHERE source_type = ?;

-- Tags
-- name: CreateTag :one
INSERT INTO tags (name, color) VALUES (?, ?)
ON CONFLICT(name) DO UPDATE SET name = excluded.name
RETURNING *;

-- name: GetTag :one
SELECT * FROM tags WHERE id = ?;

-- name: GetTagByName :one
SELECT * FROM tags WHERE name = ?;

-- name: ListTags :many
SELECT * FROM tags ORDER BY name;

-- name: DeleteTag :exec
DELETE FROM tags WHERE id = ?;

-- name: AddTagToBookmark :exec
INSERT OR IGNORE INTO bookmark_tags (bookmark_id, tag_id) VALUES (?, ?);

-- name: RemoveTagFromBookmark :exec
DELETE FROM bookmark_tags WHERE bookmark_id = ? AND tag_id = ?;

-- name: GetBookmarkTags :many
SELECT t.* FROM tags t
JOIN bookmark_tags bt ON t.id = bt.tag_id
WHERE bt.bookmark_id = ?;

-- name: GetBookmarksByTag :many
SELECT b.* FROM bookmarks b
JOIN bookmark_tags bt ON b.id = bt.bookmark_id
WHERE bt.tag_id = ?
ORDER BY b.created_at DESC
LIMIT ? OFFSET ?;

-- Collections
-- name: CreateCollection :one
INSERT INTO collections (name, description, icon) VALUES (?, ?, ?)
RETURNING *;

-- name: GetCollection :one
SELECT * FROM collections WHERE id = ?;

-- name: ListCollections :many
SELECT * FROM collections ORDER BY name;

-- name: UpdateCollection :one
UPDATE collections SET name = ?, description = ?, icon = ? WHERE id = ?
RETURNING *;

-- name: DeleteCollection :exec
DELETE FROM collections WHERE id = ?;

-- name: AddBookmarkToCollection :exec
INSERT OR IGNORE INTO bookmark_collections (bookmark_id, collection_id) VALUES (?, ?);

-- name: RemoveBookmarkFromCollection :exec
DELETE FROM bookmark_collections WHERE bookmark_id = ? AND collection_id = ?;

-- name: GetBookmarkCollections :many
SELECT c.* FROM collections c
JOIN bookmark_collections bc ON c.id = bc.collection_id
WHERE bc.bookmark_id = ?;

-- name: GetBookmarksInCollection :many
SELECT b.* FROM bookmarks b
JOIN bookmark_collections bc ON b.id = bc.bookmark_id
WHERE bc.collection_id = ?
ORDER BY b.created_at DESC
LIMIT ? OFFSET ?;

-- name: CountBookmarksInCollection :one
SELECT COUNT(*) FROM bookmark_collections WHERE collection_id = ?;
