package search

import (
	"context"
	"fmt"
	"strings"

	"github.com/moov-io/base/log"
	"github.com/moov-io/rail-msg-sql/internal/storage"

	"vitess.io/vitess/go/vt/sqlparser"
)

type Service interface {
	Search(ctx context.Context, query string) (*Results, error)
}

func NewService(logger log.Logger, fileStorage storage.Repository) (Service, error) {
	return &service{
		logger:      logger,
		fileStorage: fileStorage,
	}, nil
}

type service struct {
	logger      log.Logger
	fileStorage storage.Repository
}

func (s *service) Search(ctx context.Context, query string) (*Results, error) {
	return s.startQuery(ctx, query)
}

func (s *service) startQuery(ctx context.Context, query string) (*Results, error) {
	var opts sqlparser.Options
	p, err := sqlparser.New(opts)
	if err != nil {
		return nil, fmt.Errorf("creating sqlparser: %w", err)
	}

	stmt, err := p.Parse(query)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SQL: %v", err)
	}

	sel, ok := stmt.(*sqlparser.Select)
	if !ok {
		return nil, fmt.Errorf("only SELECT queries are supported")
	}

	if len(sel.From) != 1 {
		return nil, fmt.Errorf("exactly one table expected")
	}

	tableName := sqlparser.String(sel.From[0].(*sqlparser.AliasedTableExpr).Expr)

	switch strings.ToLower(tableName) {
	case "ach_files":
		return s.executeAchSelect(ctx, sel)
	}

	return nil, fmt.Errorf("unknown table: %s", tableName)
}

func (s *service) executeAchSelect(ctx context.Context, sel *sqlparser.Select) (*Results, error) {
	var out Results

	return &out, nil
}
