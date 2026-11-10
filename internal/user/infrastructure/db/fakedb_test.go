package db

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeDB = Querier tiruan yang menjalankan "naskah" panggilan berurutan.
// Dipakai sebagai pengganti pgxmock (v3.4.0 belum cocok dengan pgx v5.11 yang kita pakai).
// Tiap panggilan harus: jenis sama, SQL memuat potongan yang diharapkan, argumen sama persis.
type fakeDB struct {
	t     *testing.T
	steps []step
}

type step struct {
	kind     string // exec | query | queryrow
	sqlLike  string
	args     []any
	affected int64
	rows     [][]any
	rowErr   error // dikembalikan Rows.Err() setelah iterasi
	err      error
}

func newFake(t *testing.T) *fakeDB {
	t.Helper()
	f := &fakeDB{t: t}
	t.Cleanup(func() { assert.Empty(t, f.steps, "ada panggilan DB yang diharapkan tapi tidak terjadi") })
	return f
}

func (f *fakeDB) add(s step) *fakeDB { f.steps = append(f.steps, s); return f }

func (f *fakeDB) expectExec(like string, args []any, affected int64) *fakeDB {
	return f.add(step{kind: "exec", sqlLike: like, args: args, affected: affected})
}
func (f *fakeDB) expectExecErr(like string, args []any, err error) *fakeDB {
	return f.add(step{kind: "exec", sqlLike: like, args: args, err: err})
}
func (f *fakeDB) expectQuery(like string, args []any, rows [][]any) *fakeDB {
	return f.add(step{kind: "query", sqlLike: like, args: args, rows: rows})
}
func (f *fakeDB) expectQueryErr(like string, args []any, err error) *fakeDB {
	return f.add(step{kind: "query", sqlLike: like, args: args, err: err})
}
func (f *fakeDB) expectQueryRowErrAfter(like string, args []any, rows [][]any, rowErr error) *fakeDB {
	return f.add(step{kind: "query", sqlLike: like, args: args, rows: rows, rowErr: rowErr})
}
func (f *fakeDB) expectQueryRow(like string, args []any, vals ...any) *fakeDB {
	return f.add(step{kind: "queryrow", sqlLike: like, args: args, rows: [][]any{vals}})
}
func (f *fakeDB) expectQueryRowErr(like string, args []any, err error) *fakeDB {
	return f.add(step{kind: "queryrow", sqlLike: like, args: args, err: err})
}

func (f *fakeDB) next(kind, sql string, args []any) step {
	f.t.Helper()
	require.NotEmpty(f.t, f.steps, "panggilan DB tak terduga: %s %q", kind, sql)
	s := f.steps[0]
	f.steps = f.steps[1:]
	require.Equal(f.t, s.kind, kind, "jenis panggilan DB")
	require.Contains(f.t, sql, s.sqlLike, "SQL")
	require.Equal(f.t, s.args, args, "argumen SQL")
	return s
}

func (f *fakeDB) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	s := f.next("exec", sql, args)
	if s.err != nil {
		return pgconn.CommandTag{}, s.err
	}
	return pgconn.NewCommandTag(fmt.Sprintf("UPDATE %d", s.affected)), nil
}

func (f *fakeDB) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	s := f.next("query", sql, args)
	if s.err != nil {
		return nil, s.err
	}
	return &fakeRows{rows: s.rows, err: s.rowErr, idx: -1}, nil
}

func (f *fakeDB) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	s := f.next("queryrow", sql, args)
	if s.err != nil {
		return fakeRow{err: s.err}
	}
	return fakeRow{vals: s.rows[0]}
}

// ------------------------------------------------------------------ row(s)

type fakeRow struct {
	vals []any
	err  error
}

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return assign(dest, r.vals)
}

type fakeRows struct {
	rows [][]any
	err  error
	idx  int
}

func (r *fakeRows) Close()                                       {}
func (r *fakeRows) Err() error                                   { return r.err }
func (r *fakeRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeRows) Values() ([]any, error)                       { return r.rows[r.idx], nil }
func (r *fakeRows) RawValues() [][]byte                          { return nil }
func (r *fakeRows) Conn() *pgx.Conn                              { return nil }
func (r *fakeRows) TypeMap() *pgtype.Map                         { return nil }
func (r *fakeRows) Next() bool {
	r.idx++
	return r.idx < len(r.rows)
}
func (r *fakeRows) Scan(dest ...any) error { return assign(dest, r.rows[r.idx]) }

// assign meniru pgx Scan: tipe nilai harus bisa ditaruh di tujuan, kalau tidak → error.
func assign(dest, vals []any) error {
	if len(dest) != len(vals) {
		return fmt.Errorf("scan: %d tujuan tapi %d kolom", len(dest), len(vals))
	}
	for i, d := range dest {
		dv := reflect.ValueOf(d).Elem()
		sv := reflect.ValueOf(vals[i])
		if !sv.Type().AssignableTo(dv.Type()) {
			return fmt.Errorf("scan kolom %d: %s tidak bisa ke %s", i, sv.Type(), dv.Type())
		}
		dv.Set(sv)
	}
	return nil
}

var _ = strings.Contains // dipakai bersama require.Contains di atas
