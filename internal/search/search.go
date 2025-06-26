package search

import (
	"context"
	"fmt"
	"strings"

	"github.com/moov-io/rail-msg-sql/internal/storage"

	"github.com/moov-io/base/log"
	"vitess.io/vitess/go/vt/sqlparser"
)

type Service interface {
	Search(ctx context.Context, query string, params storage.FilterParams) (*Results, error)
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

func (s *service) Search(ctx context.Context, query string, params storage.FilterParams) (*Results, error) {
	return s.startQuery(ctx, query, params)
}

func (s *service) startQuery(ctx context.Context, query string, params storage.FilterParams) (*Results, error) {
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

	tableExpr, ok := sel.From[0].(*sqlparser.AliasedTableExpr)
	if !ok {
		return nil, fmt.Errorf("unexpected %T - wanted AliasedTableExpr", sel.From[0])
	}
	tableName := sqlparser.String(tableExpr.Expr)

	switch strings.ToLower(tableName) {
	case "ach_files":
		return s.executeAchFileSelect(ctx, sel, params)
	}

	return nil, fmt.Errorf("unknown table: %s", tableName)
}
