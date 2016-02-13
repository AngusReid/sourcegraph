package traceutil

import (
	"database/sql"
	"fmt"
	"time"

	"gopkg.in/gorp.v1"

	"sourcegraph.com/sourcegraph/appdash"
	"sourcegraph.com/sourcegraph/appdash/sqltrace"
	"src.sourcegraph.com/sourcegraph/util/dbutil"
)

// SQLExecutor records the timings of SQL queries in appdash and
// associates them with the span that originated them.
type SQLExecutor struct {
	gorp.SqlExecutor
	*appdash.Recorder
}

func (x SQLExecutor) record(start time.Time, query string, args []interface{}) {
	rec := x.Recorder.Child()
	rec.Name("SQL")
	rec.Event(sqltrace.SQLEvent{
		SQL:        dbutil.UnbindQuery(query, args...),
		ClientSend: start,
		ClientRecv: time.Now(),
	})
}

func (x SQLExecutor) Get(i interface{}, keys ...interface{}) (interface{}, error) {
	defer x.record(time.Now(), fmt.Sprintf("Get %T", i), keys)
	return x.SqlExecutor.Get(i, keys...)
}

func (x SQLExecutor) Insert(list ...interface{}) error {
	defer x.record(time.Now(), "Insert", list)
	return x.SqlExecutor.Insert(list...)
}

func (x SQLExecutor) Update(list ...interface{}) (int64, error) {
	defer x.record(time.Now(), "Update", list)
	return x.SqlExecutor.Update(list...)
}

func (x SQLExecutor) Delete(list ...interface{}) (int64, error) {
	defer x.record(time.Now(), "Delete", list)
	return x.SqlExecutor.Delete(list...)
}

func (x SQLExecutor) Exec(query string, args ...interface{}) (sql.Result, error) {
	defer x.record(time.Now(), query, args)
	return x.SqlExecutor.Exec(query, args...)
}

func (x SQLExecutor) Select(i interface{}, query string, args ...interface{}) ([]interface{}, error) {
	defer x.record(time.Now(), query, args)
	return x.SqlExecutor.Select(i, query, args...)
}

func (x SQLExecutor) SelectInt(query string, args ...interface{}) (int64, error) {
	defer x.record(time.Now(), query, args)
	return x.SqlExecutor.SelectInt(query, args...)
}

func (x SQLExecutor) SelectNullInt(query string, args ...interface{}) (sql.NullInt64, error) {
	defer x.record(time.Now(), query, args)
	return x.SqlExecutor.SelectNullInt(query, args...)
}

func (x SQLExecutor) SelectFloat(query string, args ...interface{}) (float64, error) {
	defer x.record(time.Now(), query, args)
	return x.SqlExecutor.SelectFloat(query, args...)
}

func (x SQLExecutor) SelectNullFloat(query string, args ...interface{}) (sql.NullFloat64, error) {
	defer x.record(time.Now(), query, args)
	return x.SqlExecutor.SelectNullFloat(query, args...)
}

func (x SQLExecutor) SelectStr(query string, args ...interface{}) (string, error) {
	defer x.record(time.Now(), query, args)
	return x.SqlExecutor.SelectStr(query, args...)
}

func (x SQLExecutor) SelectNullStr(query string, args ...interface{}) (sql.NullString, error) {
	defer x.record(time.Now(), query, args)
	return x.SqlExecutor.SelectNullStr(query, args...)
}

func (x SQLExecutor) SelectOne(holder interface{}, query string, args ...interface{}) error {
	defer x.record(time.Now(), query, args)
	return x.SqlExecutor.SelectOne(holder, query, args...)
}

// UnderlyingSQLExecutor implements db.SQLExecutorWrapper.
func (x SQLExecutor) UnderlyingSQLExecutor() gorp.SqlExecutor { return x.SqlExecutor }
