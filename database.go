package main

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

type TodoItem struct {
	Id   int64
	Text string
}

type TodoList []TodoItem

var db *sql.DB

func createDatabase() error {
	sqlStatement := `create table if not exists items(id integer primary key autoincrement, value TEXT)`

	_, err := db.Exec(sqlStatement)

	return err
}

func getTodos() (TodoList, error) {
	rows, err := db.Query("select id, value from items")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ret TodoList
	for rows.Next() {
		r := TodoItem{}

		err = rows.Scan(&r.Id, &r.Text)
		if err != nil {
			return nil, err
		}

		ret = append(ret, r)
	}

	return ret, rows.Err()
}

func addTodo(text string) (int64, error) {
	res, err := db.Exec("insert into items (value) VALUES (?)", text)
	if err != nil {
		return 0, err
	}

	id, err := res.LastInsertId()
	return id, err
}

func updateTodo(t TodoItem) error {
	_, err := db.Exec("update items set value=? where id=?", t.Text, t.Id)
	return err
}

func deleteTodo(tid int64) error {
	_, err := db.Exec("delete from items where id=?", tid)
	return err
}