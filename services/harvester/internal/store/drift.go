package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/lib/pq"
)

// DriftStore persists schema/DOM drift alerts produced by the drift detector
// (parse.DriftDetector) so a breaking change in an upstream source survives
// restarts and stays visible on the monitoring panel until it is resolved.
//
// It implements parse.DriftLogger. Only BREAKING drift is persisted: benign,
// additive evolution is informational (the detector adapts its baseline
// automatically) and would only add noise to an operator's alert list. The
// detector still logs benign drift at INFO via the slog-backed logger when the
// two are composed with parse.MultiLogger.
type DriftStore struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewDriftStore creates a DriftStore. logger may be nil (failures are then
// silent — persistence is strictly best-effort and must never disrupt the
// ingestion loop that observes drift).
func NewDriftStore(db *sql.DB, logger *slog.Logger) *DriftStore {
	return &DriftStore{db: db, logger: logger}
}

// DriftSignature returns a stable, order-independent fingerprint of a drift
// event. Identical drift (same source, severity, and field diffs) always
// produces the same signature, which the open-alert dedup index uses to
// collapse repeats into one row. Pure and exported so it is unit-testable
// without a database.
func DriftSignature(source, severity string, added, removed, retyped []string) string {
	norm := func(xs []string) string {
		cp := append([]string(nil), xs...)
		sort.Strings(cp)
		return strings.Join(cp, ",")
	}
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00%s\x00%s\x00%s\x00%s",
		source, severity, norm(added), norm(removed), norm(retyped))
	return hex.EncodeToString(h.Sum(nil))
}

// DriftSummary builds a compact human-readable description of a drift event,
// mirroring the detector's own summary vocabulary. Pure and exported so it is
// unit-testable.
func DriftSummary(severity string, added, removed, retyped []string) string {
	var parts []string
	if len(retyped) > 0 {
		parts = append(parts, "retyped=["+strings.Join(retyped, ",")+"]")
	}
	if len(removed) > 0 {
		parts = append(parts, "removed=["+strings.Join(removed, ",")+"]")
	}
	if len(added) > 0 {
		parts = append(parts, "added=["+strings.Join(added, ",")+"]")
	}
	return severity + ": " + strings.Join(parts, "; ")
}

// LogDrift implements parse.DriftLogger. It persists a BREAKING drift alert,
// deduplicating against any existing OPEN alert with the same signature
// (bumping its occurrence count and last_seen_at). Benign drift is ignored
// here. Persistence is best-effort: any error is logged and swallowed so a
// transient DB problem never breaks the fetch loop that observed the drift.
func (s *DriftStore) LogDrift(source, severity string, added, removed, retyped []string) {
	if severity != "breaking" {
		return
	}
	added, removed, retyped = orEmpty(added), orEmpty(removed), orEmpty(retyped)
	sig := DriftSignature(source, severity, added, removed, retyped)
	summary := DriftSummary(severity, added, removed, retyped)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const q = `
		INSERT INTO drift_alert
			(source, severity, signature, summary, added, removed, retyped,
			 status, occurrences, first_seen_at, last_seen_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'open', 1, now(), now())
		ON CONFLICT (signature) WHERE status = 'open'
		DO UPDATE SET
			occurrences  = drift_alert.occurrences + 1,
			last_seen_at = now(),
			summary      = EXCLUDED.summary`
	if _, err := s.db.ExecContext(ctx, q,
		source, severity, sig, summary,
		pq.Array(added), pq.Array(removed), pq.Array(retyped),
	); err != nil && s.logger != nil {
		s.logger.Warn("drift alert persistence failed",
			slog.String("source", source),
			slog.String("severity", severity),
			slog.String("error", err.Error()))
	}
}

// Resolve marks a single OPEN drift alert resolved (stamping resolved_at).
// Returns true when a row was actually transitioned (false when the id was
// unknown or already resolved). Agents/operators call this once the upstream
// drift is fixed so the alert leaves the panel.
func (s *DriftStore) Resolve(ctx context.Context, id int64) (bool, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE drift_alert SET status = 'resolved', resolved_at = now()
		 WHERE id = $1 AND status = 'open'`, id)
	if err != nil {
		return false, fmt.Errorf("resolve drift alert %d: %w", id, err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// ResolveBySource marks every OPEN alert for a source resolved and returns the
// count. Useful when a single fix addresses an entire source's contract.
func (s *DriftStore) ResolveBySource(ctx context.Context, source string) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE drift_alert SET status = 'resolved', resolved_at = now()
		 WHERE source = $1 AND status = 'open'`, source)
	if err != nil {
		return 0, fmt.Errorf("resolve drift alerts for %s: %w", source, err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// orEmpty normalises a nil slice to a non-nil empty slice so the stored array
// column is '{}' rather than NULL and the signature is stable.
func orEmpty(xs []string) []string {
	if xs == nil {
		return []string{}
	}
	return xs
}
