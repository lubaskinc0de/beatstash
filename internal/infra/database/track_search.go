package database

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

// similarWord keeps a word with a typo or two similar, and an unrelated
// one not; the <% operator, unlike word_similarity(), can use the trigram
// indexes.
const similarWord = "0.5"

// searchForm compares ignoring case, with ё read as е; the trigram indexes
// are built on it.
func searchForm(column string) string {
	return fmt.Sprintf("translate(lower(%s), 'ё', 'е')", column)
}

// searchWords are the words of the text in searchForm.
func searchWords(text string) []string {
	return strings.Fields(strings.ReplaceAll(strings.ToLower(text), "ё", "е"))
}

func trackFields(alias string) []string {
	return []string{alias + ".artist", alias + ".album_artist", alias + ".album", alias + ".title"}
}

func (r *TrackRepository) Search(
	ctx context.Context, libraryIDs []uint, text string, offset, limit int,
) ([]library.Track, error) {
	if len(libraryIDs) == 0 {
		return nil, nil
	}
	words := searchWords(text)
	var found []library.Track
	err := r.withSimilarity(ctx, func(db *gorm.DB) error {
		order := libraryOrder(libraryIDs)
		query := db.Table("tracks AS t").Select("t.*").
			Where("t.library_id = ANY(?::bigint[])", order).
			Where(`NOT EXISTS (
				SELECT 1 FROM tracks d
				WHERE d.library_id = ANY(?::bigint[])
				AND array_position(?::bigint[], d.library_id) < array_position(?::bigint[], t.library_id)
				AND t.artist <> '' AND t.title <> '' AND `+sameRecording("d", "t")+`
			)`, order, order, order)
		if len(words) == 0 {
			query = query.Clauses(orderBy("t.created_at DESC, t.id DESC"))
		} else {
			cond, args := matching(db, order, words, trackFields("t"))
			score, scoreArgs := score(words, []string{"t.title", "t.artist"}, trackFields("t"))
			query = query.Where(cond, args...).
				Clauses(orderBy(score+" DESC, lower(t.artist), lower(t.album), t.track_number, lower(t.title), t.id", scoreArgs...))
		}
		return query.Offset(offset).Limit(limit).Find(&found).Error
	})
	return found, err
}

func (r *TrackRepository) SearchAlbums(
	ctx context.Context, libraryIDs []uint, text string, indexedOnly bool, offset, limit int,
) ([]repositories.AlbumSummary, error) {
	if len(libraryIDs) == 0 {
		return nil, nil
	}
	words := searchWords(text)
	var rows []struct {
		LibraryID   uint
		AlbumArtist string
		Album       string
		TrackID     uint
		Tracks      int
	}
	err := r.withSimilarity(ctx, func(db *gorm.DB) error {
		order := libraryOrder(libraryIDs)
		albums := db.Table("tracks AS t").
			Select(`t.library_id, min(t.album_artist) AS album_artist, min(t.album) AS album, min(t.id) AS track_id,
				count(*) AS tracks, max(t.created_at) AS newest`).
			Where("t.library_id = ANY(?::bigint[]) AND t.album <> ''", order).
			Group("t.library_id, lower(t.album_artist), lower(t.album)")
		if len(words) > 0 {
			cond, args := matching(db, order, words, []string{"t.album_artist", "t.album"})
			albums = albums.Where(cond, args...)
		}
		if indexedOnly {
			albums = albums.Having("bool_or(t.song_id <> '')")
		}

		query := db.Table("(?) AS a", albums).Select("a.*").
			Where(`NOT EXISTS (
				SELECT 1 FROM tracks d
				WHERE d.library_id = ANY(?::bigint[])
				AND array_position(?::bigint[], d.library_id) < array_position(?::bigint[], a.library_id)
				AND lower(d.album_artist) = lower(a.album_artist) AND lower(d.album) = lower(a.album)
			)`, order, order, order)
		if len(words) == 0 {
			query = query.Clauses(orderBy("a.newest DESC, a.track_id DESC"))
		} else {
			score, args := score(words, []string{"a.album"}, []string{"a.album_artist", "a.album"})
			query = query.Clauses(orderBy(score+" DESC, lower(a.album_artist), lower(a.album), a.track_id", args...))
		}
		return query.Offset(offset).Limit(limit).Scan(&rows).Error
	})
	if err != nil {
		return nil, err
	}
	found := make([]repositories.AlbumSummary, 0, len(rows))
	for _, row := range rows {
		found = append(found, repositories.AlbumSummary{
			AlbumKey: library.AlbumKey{LibraryID: row.LibraryID, AlbumArtist: row.AlbumArtist, Album: row.Album},
			TrackID:  row.TrackID, Tracks: row.Tracks,
		})
	}
	return found, nil
}

func (r *TrackRepository) withSimilarity(ctx context.Context, fn func(db *gorm.DB) error) error {
	return dbForContext(ctx, r.DB).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT set_config('pg_trgm.word_similarity_threshold', ?, true)", similarWord).Error; err != nil {
			return err
		}
		return fn(tx)
	})
}

// matching takes the words as typed, or as similar only if no Track has
// them as typed: a query that matches brings nothing merely alike.
func matching(db *gorm.DB, order string, words, fields []string) (string, []any) {
	exact, exactArgs := allWords(words, trackFields("m"), false)
	matched := db.Table("tracks AS m").Select("1").Where("m.library_id = ANY(?::bigint[])", order).Where(exact, exactArgs...)
	typed, typedArgs := allWords(words, fields, false)
	similar, similarArgs := allWords(words, fields, true)
	args := append(append(append([]any{matched}, typedArgs...), matched), similarArgs...)
	return "(EXISTS (?) AND " + typed + ") OR (NOT EXISTS (?) AND " + similar + ")", args
}

func allWords(words, fields []string, similar bool) (string, []any) {
	conds := make([]string, 0, len(words))
	var args []any
	for _, word := range words {
		parts := make([]string, 0, len(fields))
		for _, field := range fields {
			if similar {
				parts = append(parts, "? <% "+searchForm(field))
				args = append(args, word)
			} else {
				parts = append(parts, searchForm(field)+" LIKE ?")
				args = append(args, "%"+likeEscaper.Replace(word)+"%")
			}
		}
		conds = append(conds, "("+strings.Join(parts, " OR ")+")")
	}
	return "(" + strings.Join(conds, " AND ") + ")", args
}

// score ranks the words found in the first fields above the rest.
func score(words, first, fields []string) (string, []any) {
	terms := make([]string, 0, 2*len(words))
	var args []any
	for _, word := range words {
		found, foundArgs := allWords([]string{word}, first, false)
		terms = append(terms, found+"::int")
		args = append(args, foundArgs...)
		similar := make([]string, 0, len(fields))
		for _, field := range fields {
			similar = append(similar, fmt.Sprintf("word_similarity(?, %s)", searchForm(field)))
			args = append(args, word)
		}
		terms = append(terms, "greatest("+strings.Join(similar, ", ")+")")
	}
	return "(" + strings.Join(terms, " + ") + ")", args
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// libraryOrder: the order of the libraries tells which copy of a
// Duplicate wins.
func libraryOrder(ids []uint) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, strconv.FormatUint(uint64(id), 10))
	}
	return "{" + strings.Join(parts, ",") + "}"
}
