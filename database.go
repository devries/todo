package main

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

type TodoItem struct {
	Id   int64
	Text string
	Done bool
}

type TodoList []TodoItem

func createDatabase(db *sql.DB) error {
	sqlStatement := `create table if not exists items(id integer primary key autoincrement, value TEXT, done INTEGER default 0)`

	_, err := db.Exec(sqlStatement)

	return err
}

func getTodos(db *sql.DB) (TodoList, error) {
	rows, err := db.Query("select id, value, done from items")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ret TodoList
	for rows.Next() {
		r := TodoItem{}

		err = rows.Scan(&r.Id, &r.Text, &r.Done)
		if err != nil {
			return nil, err
		}

		ret = append(ret, r)
	}

	return ret, rows.Err()
}

func getOneTodo(db *sql.DB, tid int64) (TodoItem, error) {
	ret := TodoItem{Id: tid}
	err := db.QueryRow("select value, done from items where id=?", tid).Scan(&ret.Text, &ret.Done)
	return ret, err
}

func addTodo(db *sql.DB, text string) (int64, error) {
	res, err := db.Exec("insert into items (value) VALUES (?)", text)
	if err != nil {
		return 0, err
	}

	id, err := res.LastInsertId()
	return id, err
}

func markTodoDone(db *sql.DB, tid int64) error {
	_, err := db.Exec("update items set done=? where id=?", true, tid)
	return err
}

func markTodoNotDone(db *sql.DB, tid int64) error {
	_, err := db.Exec("update items set done=? where id=?", false, tid)
	return err
}

func deleteTodo(db *sql.DB, tid int64) error {
	res, err := db.Exec("delete from items where id=?", tid)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		err = fmt.Errorf("%d rows were affected in delete, expected only 1 row to be deleted", n)
	}
	return err
}
