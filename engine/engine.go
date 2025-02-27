package engine

import (
	"fmt"
	"time"

	"github.com/eturella/go-mysql-test/auth"
	"github.com/eturella/go-mysql-test/sql"
	"github.com/eturella/go-mysql-test/sql/analyzer"
	"github.com/eturella/go-mysql-test/sql/parse"
	"github.com/go-kit/kit/metrics/discard"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/sirupsen/logrus"
)

// Config for the Engine.
type Config struct {
	// VersionPostfix to display with the `VERSION()` UDF.
	VersionPostfix string
	// Auth used for authentication and authorization.
	Auth auth.Auth
}

// Engine is a SQL engine.
type Engine struct {
	Catalog  *sql.Catalog
	Analyzer *analyzer.Analyzer
	Auth     auth.Auth
}

var (
	// QueryCounter describes a metric that accumulates number of queries monotonically.
	QueryCounter = discard.NewCounter()

	// QueryErrorCounter describes a metric that accumulates number of failed queries monotonically.
	QueryErrorCounter = discard.NewCounter()

	// QueryHistogram describes a queries latency.
	QueryHistogram = discard.NewHistogram()
)

func observeQuery(ctx *sql.Context, query string) func(err error) {
	logrus.WithField("query", query).Debug("executing query")
	span, _ := ctx.Span("query", opentracing.Tag{Key: "query", Value: query})

	t := time.Now()
	return func(err error) {
		if err != nil {
			QueryErrorCounter.With("query", query, "error", err.Error()).Add(1)
		} else {
			QueryCounter.With("query", query).Add(1)
			QueryHistogram.With("query", query, "duration", "seconds").Observe(time.Since(t).Seconds())
		}

		span.Finish()
	}
}

// New creates a new Engine with custom configuration. To create an Engine with
// the default settings use `NewDefault`.
func New(c *sql.Catalog, a *analyzer.Analyzer, cfg *Config) *Engine {
	// var versionPostfix string
	// if cfg != nil {
	// 	versionPostfix = cfg.VersionPostfix
	// }

	// c.MustRegister(
	// 	sql.FunctionN{
	// 		Name: "version",
	// 		Fn:   function.NewVersion(versionPostfix),
	// 	},
	// 	sql.Function0{
	// 		Name: "database",
	// 		Fn:   function.NewDatabase(c),
	// 	})
	// c.MustRegister(function.Defaults...)

	// use auth.None if auth is not specified
	var au auth.Auth
	if cfg == nil || cfg.Auth == nil {
		au = new(auth.None)
	} else {
		au = cfg.Auth
	}

	return &Engine{c, a, au}
}

// NewDefault creates a new default Engine.
func NewDefault() *Engine {
	c := sql.NewCatalog()
	a := analyzer.NewDefault(c)

	return New(c, a, nil)
}

// Query executes a query.
func (e *Engine) Query(
	ctx *sql.Context,
	query string,
) (sql.Schema, sql.RowIter, error) {
	var (
		parsed, analyzed sql.Node
		iter             sql.RowIter
		err              error
	)

	finish := observeQuery(ctx, query)
	defer finish(err)

	parsed, err = parse.Parse(ctx, query)
	fmt.Printf("%+v\n", parsed)
	if err != nil {
		return nil, nil, err
	}

	var perm = auth.ReadPerm
	var typ = sql.QueryProcess
	// switch parsed.(type) {
	// case *plan.CreateIndex:
	// 	typ = sql.CreateIndexProcess
	// 	perm = auth.ReadPerm | auth.WritePerm
	// case *plan.InsertInto, *plan.DeleteFrom, *plan.Update, *plan.DropIndex, *plan.UnlockTables, *plan.LockTables:
	// 	perm = auth.ReadPerm | auth.WritePerm
	// }

	err = e.Auth.Allowed(ctx, perm)
	if err != nil {
		return nil, nil, err
	}

	ctx, err = e.Catalog.AddProcess(ctx, typ, query)
	defer func() {
		if err != nil && ctx != nil {
			e.Catalog.Done(ctx.Pid())
		}
	}()

	if err != nil {
		return nil, nil, err
	}

	analyzed, err = e.Analyzer.Analyze(ctx, parsed)
	fmt.Printf("%+v\n", analyzed)
	if err != nil {
		return nil, nil, err
	}

	iter, err = analyzed.RowIter(ctx)
	if err != nil {
		return nil, nil, err
	}

	return analyzed.Schema(), iter, nil
}

// Async returns true if the query is async. If there are any errors with the
// query it returns false
func (e *Engine) Async(ctx *sql.Context, query string) bool {
	// parsed, err := parse.Parse(ctx, query)
	// if err != nil {
	// 	return false
	// }

	// asyncNode, ok := parsed.(sql.AsyncNode)
	// return ok && asyncNode.IsAsync()
	return false
}

// AddDatabase adds the given database to the catalog.
func (e *Engine) AddDatabase(db sql.Database) {
	e.Catalog.AddDatabase(db)
}

// Init performs all the initialization requirements for the engine to work.
func (e *Engine) Init() error {
	//return e.Catalog.LoadIndexes(e.Catalog.AllDatabases())
	return nil
}
