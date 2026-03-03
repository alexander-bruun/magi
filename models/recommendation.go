package models

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// tagFrequencyCache holds precomputed IDF weights for tags/genres.
// Populated once via ComputeTagIDF and reused across recommendation calls.
var tagIDF map[string]float64

// ComputeTagIDF precomputes inverse document frequency weights for all tags/genres
// in the catalog. Call this once at startup or after bulk indexing.
//
// IDF(tag) = log(1 + totalMedia / (1 + mediaWithTag))
//
// Common tags like "action" get low weight; rare tags like "isekai-reincarnation"
// get high weight, making tag overlap a much stronger signal when it matters.
func ComputeTagIDF() error {
	rows, err := db.Query(`
		SELECT tag, COUNT(DISTINCT media_slug) as freq
		FROM media_tags
		GROUP BY tag
	`)
	if err != nil {
		return fmt.Errorf("ComputeTagIDF query: %w", err)
	}
	defer rows.Close()

	freqMap := make(map[string]int)
	for rows.Next() {
		var tag string
		var freq int
		if err := rows.Scan(&tag, &freq); err != nil {
			continue
		}
		freqMap[strings.ToLower(tag)] = freq
	}

	var totalMedia int
	if err := db.QueryRow(`SELECT COUNT(*) FROM media`).Scan(&totalMedia); err != nil {
		return fmt.Errorf("ComputeTagIDF count: %w", err)
	}

	tagIDF = make(map[string]float64, len(freqMap))
	for tag, freq := range freqMap {
		tagIDF[tag] = math.Log1p(float64(totalMedia) / float64(1+freq))
	}
	return nil
}

// idfWeight returns the IDF weight for a tag, falling back to a default if
// ComputeTagIDF hasn't been run yet.
func idfWeight(tag string) float64 {
	if tagIDF == nil {
		return 1.0
	}
	if w, ok := tagIDF[strings.ToLower(tag)]; ok {
		return w
	}
	return 1.0 // unknown tag gets neutral weight
}

// (removed tableExists) Authors are stored as JSON in `media.authors`.
// Recommendation queries should use that JSON column rather than a
// non-existent auxiliary table.

// cosineSimilarity computes the cosine similarity between two TF-IDF tag vectors.
// Each media is represented as a weighted set of tags+genres, and similarity is
// the dot product divided by the product of magnitudes.
func cosineSimilarity(aWeights, bWeights map[string]float64) float64 {
	var dot, magA, magB float64
	for tag, wa := range aWeights {
		magA += wa * wa
		if wb, ok := bWeights[tag]; ok {
			dot += wa * wb
		}
	}
	for _, wb := range bWeights {
		magB += wb * wb
	}
	if magA == 0 || magB == 0 {
		return 0
	}
	return dot / (math.Sqrt(magA) * math.Sqrt(magB))
}

// buildTagVector returns a TF-IDF-style weight map for a media's tags and genres.
// Tags and genres are combined; IDF down-weights ubiquitous labels.
func buildTagVector(tags, genres []string) map[string]float64 {
	vec := make(map[string]float64)
	for _, t := range tags {
		key := strings.ToLower(t)
		vec[key] += idfWeight(key) // additive in case of duplicates
	}
	for _, g := range genres {
		key := strings.ToLower(g)
		// Genres are broader than tags so give them a small boost
		vec[key] += idfWeight(key) * 1.2
	}
	return vec
}

// wordTokens splits text into lowercase word tokens, filtering stopwords and
// single-character tokens. Used for title/description overlap scoring.
func wordTokens(s string) map[string]struct{} {
	stopwords := map[string]struct{}{
		"the": {}, "a": {}, "an": {}, "and": {}, "or": {}, "but": {}, "in": {},
		"on": {}, "at": {}, "to": {}, "for": {}, "of": {}, "with": {}, "is": {},
		"it": {}, "as": {}, "by": {}, "from": {}, "this": {}, "that": {},
		"his": {}, "her": {}, "he": {}, "she": {}, "they": {}, "their": {},
		"are": {}, "was": {}, "be": {}, "has": {}, "had": {},
	}
	out := make(map[string]struct{})
	lower := strings.ToLower(s)
	// Split on any non-alphanumeric character
	start := -1
	for i, r := range lower {
		isAlnum := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if isAlnum && start == -1 {
			start = i
		} else if !isAlnum && start != -1 {
			tok := lower[start:i]
			if len(tok) > 1 {
				if _, stop := stopwords[tok]; !stop {
					out[tok] = struct{}{}
				}
			}
			start = -1
		}
	}
	if start != -1 {
		tok := lower[start:]
		if len(tok) > 1 {
			if _, stop := stopwords[tok]; !stop {
				out[tok] = struct{}{}
			}
		}
	}
	return out
}

// jaccardWords computes word-level Jaccard similarity between two token sets.
func jaccardWords(a, b map[string]struct{}) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	inter := 0
	for t := range a {
		if _, ok := b[t]; ok {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

// popularityScore normalises a raw popularity value to [0, 1] using a soft cap.
// Uses a logistic-style curve so mega-popular titles don't dominate.
func popularityScore(pop, fav int) float64 {
	// Treat popularity and favorites equally, soft-cap at 100k combined
	combined := float64(pop + fav)
	return combined / (combined + 50000)
}

// ComputeAndStoreRecommendations calculates content-based recommendations for a
// media item and persists them. Designed to be called at index time.
//
// Scoring uses four normalised sub-scores combined with fixed weights:
//   - Tag/genre TF-IDF cosine similarity  (weight 0.40)
//   - Author match                        (weight 0.25)
//   - Description word Jaccard similarity (weight 0.20)
//   - Popularity                          (weight 0.15)
//
// Recommendations are post-processed with Maximal Marginal Relevance (MMR) to
// ensure diversity: the final 12 results cover varied authors and genre clusters
// rather than returning 12 near-identical titles.
func ComputeAndStoreRecommendations(base *Media) error {
	if base == nil {
		return fmt.Errorf("base media is nil")
	}

	// --- Build base feature vectors ----------------------------------------

	baseTagVec := buildTagVector(base.Tags, base.Genres)
	baseDescTokens := wordTokens(base.Description)

	baseAuthorSet := make(map[string]struct{})
	for _, a := range base.Authors {
		if a.Name != "" {
			baseAuthorSet[strings.ToLower(a.Name)] = struct{}{}
		}
	}

	// Collect all tags from the base for the SQL candidate query
	tagSet := make(map[string]struct{})
	for t := range baseTagVec {
		tagSet[t] = struct{}{}
	}
	tagList := make([]string, 0, len(tagSet))
	for t := range tagSet {
		tagList = append(tagList, t)
	}

	// --- Fetch candidates from DB -------------------------------------------
	// Broad query: match on any overlapping tag/genre, same author, type, or
	// demographic. We fetch up to 300 candidates and re-rank in Go.

	tagPlaceholders := make([]string, len(tagList))
	for i := range tagList {
		tagPlaceholders[i] = "?"
	}

	query := `
		SELECT m.slug, m.name, m.author, m.description, m.year, m.original_language,
		       m.type, m.status, m.content_rating, m.cover_art_url, m.file_count,
		       m.created_at, m.updated_at,
		       COALESCE(m.start_date, '') as start_date,
		       COALESCE(m.end_date, '') as end_date,
		       COALESCE(m.chapter_count, 0) as chapter_count,
		       COALESCE(m.volume_count, 0) as volume_count,
		       COALESCE(m.average_score, 0.0) as average_score,
		       COALESCE(m.popularity, 0) as popularity,
		       COALESCE(m.favorites, 0) as favorites,
		       COALESCE(m.demographic, '') as demographic,
		       COALESCE(m.publisher, '') as publisher,
		       COALESCE(m.magazine, '') as magazine,
		       COALESCE(m.serialization, '') as serialization,
		       COALESCE(m.authors, '[]') as authors,
		       COALESCE(m.artists, '[]') as artists,
		       COALESCE(m.genres, '[]') as genres,
		       COALESCE(m.characters, '[]') as characters,
		       COALESCE(m.alternative_titles, '[]') as alternative_titles,
		       COALESCE(m.attribution_links, '[]') as attribution_links,
		       COALESCE(m.potential_poster_urls, '[]') as potential_poster_urls
		FROM media m
		WHERE m.slug != ?`

	args := []any{base.Slug}

	if len(tagList) > 0 {
		query += ` AND (
			EXISTS (
				SELECT 1 FROM media_tags mt
				WHERE mt.media_slug = m.slug
				  AND mt.tag IN (` + strings.Join(tagPlaceholders, ",") + `)
			)`
		for _, t := range tagList {
			args = append(args, t)
		}
		// Author match using the JSON `m.authors` column.
		if len(baseAuthorSet) > 0 {
			authorList := make([]string, 0, len(baseAuthorSet))
			for a := range baseAuthorSet {
				authorList = append(authorList, a)
			}

			// Match authors using the JSON `m.authors` column via SQLite JSON
			// functions. Tests whether any author name in the array matches
			// one of the base authors.
			authorPlaceholders := make([]string, len(authorList))
			for i := range authorList {
				authorPlaceholders[i] = "?"
			}
			query += ` OR EXISTS (
				SELECT 1 FROM json_each(m.authors) j
				WHERE LOWER(json_extract(j.value, '$.name')) IN (` + strings.Join(authorPlaceholders, ",") + `)
			)`
			for _, a := range authorList {
				args = append(args, a)
			}
		}
		if base.Type != "" {
			query += ` OR LOWER(m.type) = ?`
			args = append(args, strings.ToLower(base.Type))
		}
		query += `)`
	}

	query += ` ORDER BY m.popularity DESC LIMIT 300`

	rows, err := db.Query(query, args...)
	if err != nil {
		return fmt.Errorf("candidate query: %w", err)
	}
	defer rows.Close()

	type candidate struct {
		m           Media
		tagVec      map[string]float64
		descTokens  map[string]struct{}
		authorMatch bool
		// final combined score (0–1 scale)
		relevance float64
	}

	var candidates []candidate
	for rows.Next() {
		m, err := scanMediaRow(rows)
		if err != nil || m.Slug == base.Slug {
			continue
		}
		cTagVec := buildTagVector(m.Tags, m.Genres)
		cDescTokens := wordTokens(m.Description)

		// Check author overlap
		authorMatch := false
		for _, a := range m.Authors {
			if _, ok := baseAuthorSet[strings.ToLower(a.Name)]; ok && a.Name != "" {
				authorMatch = true
				break
			}
		}

		candidates = append(candidates, candidate{
			m:           m,
			tagVec:      cTagVec,
			descTokens:  cDescTokens,
			authorMatch: authorMatch,
		})
	}

	// --- Score candidates ---------------------------------------------------

	const (
		wTag    = 0.40
		wAuthor = 0.25
		wDesc   = 0.20
		wPop    = 0.15
	)

	for i := range candidates {
		c := &candidates[i]

		// 1. TF-IDF cosine similarity on tags+genres (0–1)
		tagSim := cosineSimilarity(baseTagVec, c.tagVec)

		// 2. Author match (binary, 0 or 1)
		var authorSim float64
		if c.authorMatch {
			authorSim = 1.0
		}

		// 3. Description word Jaccard (0–1)
		descSim := jaccardWords(baseDescTokens, c.descTokens)

		// 4. Popularity (soft-normalised 0–1)
		popSim := popularityScore(c.m.Popularity, c.m.Favorites)

		// Optional small bonus: same type / demographic keeps scores comparable
		// within a media type without dominating the ranking.
		var typeBonus float64
		if base.Type != "" && strings.EqualFold(base.Type, c.m.Type) {
			typeBonus = 0.03
		}
		if base.Demographic != "" && strings.EqualFold(base.Demographic, c.m.Demographic) {
			typeBonus += 0.02
		}

		c.relevance = wTag*tagSim + wAuthor*authorSim + wDesc*descSim + wPop*popSim + typeBonus
	}

	// Sort descending by relevance so MMR has the best pool to work from.
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].relevance > candidates[j].relevance
	})

	// --- Maximal Marginal Relevance (MMR) -----------------------------------
	// MMR iteratively selects the next candidate that maximises:
	//   MMR_score = λ * relevance − (1−λ) * max_similarity_to_selected
	// where similarity_to_selected is the tag-cosine similarity with every
	// already-selected item. This prevents the top-12 from all being sequels
	// or genre-clones of the same series.
	const (
		mmrLambda = 0.7 // 0 = pure diversity, 1 = pure relevance
		numRecs   = 12
	)

	selected := make([]candidate, 0, numRecs)
	remaining := make([]int, len(candidates)) // indices into candidates
	for i := range remaining {
		remaining[i] = i
	}

	for len(selected) < numRecs && len(remaining) > 0 {
		bestIdx := -1
		bestMMR := math.Inf(-1)

		for ri, ci := range remaining {
			c := candidates[ci]

			// Max similarity to any already-selected item
			maxSim := 0.0
			for _, sel := range selected {
				sim := cosineSimilarity(c.tagVec, sel.tagVec)
				if sim > maxSim {
					maxSim = sim
				}
			}

			mmrScore := mmrLambda*c.relevance - (1-mmrLambda)*maxSim
			if mmrScore > bestMMR {
				bestMMR = mmrScore
				bestIdx = ri
			}
		}

		if bestIdx == -1 {
			break
		}
		selected = append(selected, candidates[remaining[bestIdx]])
		// Remove chosen index from remaining
		remaining = append(remaining[:bestIdx], remaining[bestIdx+1:]...)
	}

	// --- Persist ------------------------------------------------------------

	recs := make([]MediaRecommendation, 0, len(selected))
	for rank, s := range selected {
		// Store score as an integer (basis points of the 0–1 float, × 10000)
		// so it fits existing integer schema while preserving ordering.
		recs = append(recs, MediaRecommendation{
			MediaSlug:       base.Slug,
			RecommendedSlug: s.m.Slug,
			Score:           int(s.relevance*10000) - rank, // rank tiebreak
		})
	}

	return SaveMediaRecommendations(base.Slug, recs)
}

// GetRecommendedMedia returns up to 12 precomputed recommendations for base.
// Falls back to an empty list (not an error) if none have been computed yet.
func GetRecommendedMedia(base *Media) ([]Media, error) {
	if base == nil {
		return nil, fmt.Errorf("base media is nil")
	}
	slugs, err := GetMediaRecommendations(base.Slug, 12)
	if err != nil || len(slugs) == 0 {
		return nil, fmt.Errorf("no recommendations found for %s: %w", base.Slug, err)
	}
	recs := make([]Media, 0, len(slugs))
	for _, slug := range slugs {
		m, err := GetMedia(slug)
		if err == nil && m != nil {
			recs = append(recs, *m)
		}
	}
	return recs, nil
}
