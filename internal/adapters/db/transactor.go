package db

import (
	"context"

	"github.com/jmoiron/sqlx"
)

type Transactor struct {
	db *sqlx.DB
}

func NewTransactor(db *sqlx.DB) *Transactor {
	return &Transactor{db: db}
}

func (t *Transactor) WithTx(ctx context.Context, fn func(context.Context) error) (err error) {
	if _, ok := txFromContext(ctx); ok {
		// If there's already a transaction in the context, just call the function
		return fn(ctx)
	}

	tx, err := t.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}

	ctxWithTx := contextWithTx(ctx, tx)

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p) // re-throw panic after Rollback
		} else if err != nil {
			tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()

	return fn(ctxWithTx)
}

type txContextKey struct{}

func txFromContext(ctx context.Context) (*sqlx.Tx, bool) {
	tx, ok := ctx.Value(txContextKey{}).(*sqlx.Tx)
	return tx, ok
}

func contextWithTx(ctx context.Context, tx *sqlx.Tx) context.Context {
	return context.WithValue(ctx, txContextKey{}, tx)
}
