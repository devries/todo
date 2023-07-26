package main

import (
	"database/sql"
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/alexedwards/flow"
)

//go:embed templates/*.html
var templateFiles embed.FS

//go:embed static
var staticFiles embed.FS

func main() {
	var err error
	var bind string

	switch len(os.Args) {
	case 1:
		bind = ":8080"
	case 2:
		bind = os.Args[1]
	default:
		log.Fatalf("Usage: %s [bind]", os.Args[0])
	}

	mux := flow.New()
	mux.Use(loggingHandler)

	db, err = sql.Open("sqlite3", "./todo.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err = db.Ping(); err != nil {
		log.Fatal(err)
	}

	if err = createDatabase(); err != nil {
		log.Fatal(err)
	}

	templates := template.Must(template.New("web").ParseFS(templateFiles, "templates/*"))
	for _, t := range templates.Templates() {
		log.Print(t.Name())
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		dtl, err := getTodos()
		if err != nil {
			log.Printf("Error getting todo list: %s", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		indexTemplate := templates.Lookup("index.html")
		err = indexTemplate.Execute(w, dtl)
		if err != nil {
			log.Printf("Error rendering template: %s", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
	}, "GET")

	mux.HandleFunc("/delete/:id|^[0-9]+$", func(w http.ResponseWriter, r *http.Request) {
		param := flow.Param(r.Context(), "id")
		val, err := strconv.ParseInt(param, 10, 64)
		if err != nil {
			log.Printf("Unable to convert %s to integer: %s", param, err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		err = deleteTodo(val)
		if err != nil {
			log.Printf("Unable to delete entry: %s", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "")
	}, "DELETE")

	mux.HandleFunc("/add", func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			log.Printf("Error parsing form: %s", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		text := r.FormValue("newTodo")
		if text == "" {
			log.Printf("Entry is empty")
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		tdId, err := addTodo(text)
		if err != nil {
			log.Printf("Error writing todo item: %s", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		tdi := TodoItem{tdId, text}
		respTemplate := templates.Lookup("todoitem.html")
		err = respTemplate.Execute(w, tdi)
		if err != nil {
			log.Printf("Error rendering template: %s", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
	}, "POST")

	mux.Handle("/static/...", http.FileServer(http.FS(staticFiles)))

	server := &http.Server{
		Addr:    bind,
		Handler: mux,
	}

	log.Printf("Server starting on %s", bind)
	log.Fatal(server.ListenAndServe())
}
