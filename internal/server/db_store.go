package server

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nhdewitt/spectra/internal/database"
)

// Store is the production DB implementation: the sql-c generated queries
// plus the pool needed to open a transaction.
//
// The pool lives here rather than on Server so Server keeps exactly one
// database dependency. Handing Server both a DB and a pool would give it
// two ways to reach the database, and would put transactional behavior
// outside what MockDB can stand in for.
type Store struct {
	*database.Queries
	pool *pgxpool.Pool
}

// NewStore wraps a pool and its generated queries.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{
		Queries: database.New(pool),
		pool:    pool,
	}
}

// WithMetricTx runs fn inside a transaction, committing only if it returns
// nil.
//
// The deferred Rollback is a no-op once Commit has succeeded, so it covers
// the error return and a panic partway through the batch alive.
func (s *Store) WithMetricTx(ctx context.Context, fn func(MetricWriter) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := fn(s.Queries.WithTx(tx)); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
