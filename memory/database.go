package memory

import (
	"github.com/eturella/go-mysql-test/sql"
)

// Database is an in-memory database.
type Database struct {
	name   string
	tables map[string]sql.Table
}

// NewDatabase creates a new database with the given name.
func NewDatabase(name string) *Database {
	return &Database{
		name:   name,
		tables: map[string]sql.Table{},
	}
}

// Name returns the database name.
func (d *Database) Name() string {
	return d.name
}

// Tables returns all tables in the database.
func (d *Database) Tables() map[string]sql.Table {
	return d.tables
}

// AddTable adds a new table to the database.
func (d *Database) AddTable(name string, t sql.Table) {
	d.tables[name] = t
}

// CreateTable creates a table with the given name and schema
func (d *Database) CreateTable(ctx *sql.Context, name string, schema sql.Schema) error {
	_, ok := d.tables[name]
	if ok {
		return sql.ErrTableAlreadyExists.New(name)
	}

	d.tables[name] = NewTable(name, schema)
	return nil
}

// DropTable drops the table with the given name
func (d *Database) DropTable(ctx *sql.Context, name string) error {
	_, ok := d.tables[name]
	if !ok {
		return sql.ErrTableNotFound.New(name)
	}

	delete(d.tables, name)
	return nil
}
