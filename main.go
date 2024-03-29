package main

import (
	"database/sql"
	"embed"
	"encoding/json"
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

	db, err := sql.Open("sqlite3", "./todo.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err = db.Ping(); err != nil {
		log.Fatal(err)
	}

	if err = createDatabase(db); err != nil {
		log.Fatal(err)
	}

	templates := template.Must(template.New("web").ParseFS(templateFiles, "templates/*"))
	handlers := &Env{db, templates}

	// HTML and API methods (if Accept is set to application/json)
	mux.HandleFunc("/", handlers.indexHandlerFunc, "GET")
	mux.HandleFunc("/delete/:id|^[0-9]+$", handlers.deleteHandlerFunc, "DELETE")
	mux.HandleFunc("/do/:id|^[0-9]+$", handlers.doHandlerFunc, "GET")
	mux.HandleFunc("/undo/:id|^[0-9]+$", handlers.undoHandlerFunc, "GET")
	mux.HandleFunc("/add", handlers.addHandlerFunc, "POST")

	// Static files
	mux.Handle("/static/...", http.FileServer(http.FS(staticFiles)))

	server := &http.Server{
		Addr:    bind,
		Handler: mux,
	}

	log.Printf("Server starting on %s", bind)
	log.Fatal(server.ListenAndServe())
}

type Env struct {
	db        *sql.DB
	templates *template.Template
}

func (e *Env) indexHandlerFunc(w http.ResponseWriter, r *http.Request) {
	dtl, err := getTodos(e.db)
	if err != nil {
		log.Printf("Error getting todo list: %s", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	switch r.Header.Get("Accept") {
	case "application/json":
		encoder := json.NewEncoder(w)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(dtl); err != nil {
			log.Printf("Error encoding response to json: %s", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
	default:
		indexTemplate := e.templates.Lookup("index.html")
		err = indexTemplate.Execute(w, dtl)
		if err != nil {
			log.Printf("Error rendering template: %s", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
	}
}

func (e *Env) doHandlerFunc(w http.ResponseWriter, r *http.Request) {
	param := flow.Param(r.Context(), "id")
	val, err := strconv.ParseInt(param, 10, 64)
	if err != nil {
		log.Printf("Unable to convert %s to integer: %s", param, err)
		http.Error(w, "Markdone expects an integer path element", http.StatusBadRequest)
		return
	}
	err = markTodoDone(e.db, val)
	if err != nil {
		log.Printf("Unable to mark entry %d done: %s", val, err)
		http.Error(w, "Unable to mark entry as done", http.StatusInternalServerError)
		return
	}

	tdi, err := getOneTodo(e.db, val)
	if err != nil {
		log.Printf("Unable to retrieve updated todo item %d: %s", val, err)
		http.Error(w, "Unable to retrieve marked entry", http.StatusInternalServerError)
		return
	}

	switch r.Header.Get("Accept") {
	case "application/json":
		encoder := json.NewEncoder(w)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(tdi); err != nil {
			log.Printf("Error encoding response to json: %s", err)
			http.Error(w, "Unable to render response", http.StatusInternalServerError)
		}
	default:
		respTemplate := e.templates.Lookup("todoitem.html")
		err = respTemplate.Execute(w, tdi)
		if err != nil {
			log.Printf("Error rendering template: %s", err)
			http.Error(w, "Unable to render response", http.StatusInternalServerError)
		}
	}
}

func (e *Env) undoHandlerFunc(w http.ResponseWriter, r *http.Request) {
	param := flow.Param(r.Context(), "id")
	val, err := strconv.ParseInt(param, 10, 64)
	if err != nil {
		log.Printf("Unable to convert %s to integer: %s", param, err)
		http.Error(w, "Markundone expects an integer path element", http.StatusBadRequest)
		return
	}
	err = markTodoNotDone(e.db, val)
	if err != nil {
		log.Printf("Unable to mark entry %d not done: %s", val, err)
		http.Error(w, "Unable to mark entry as not done", http.StatusInternalServerError)
		return
	}

	tdi, err := getOneTodo(e.db, val)
	if err != nil {
		log.Printf("Unable to retrieve updated todo item %d: %s", val, err)
		http.Error(w, "Unable to retrieve marked entry", http.StatusInternalServerError)
		return
	}

	switch r.Header.Get("Accept") {
	case "application/json":
		encoder := json.NewEncoder(w)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(tdi); err != nil {
			log.Printf("Error encoding response to json: %s", err)
			http.Error(w, "Unable to render response", http.StatusInternalServerError)
		}
	default:
		respTemplate := e.templates.Lookup("todoitem.html")
		err = respTemplate.Execute(w, tdi)
		if err != nil {
			log.Printf("Error rendering template: %s", err)
			http.Error(w, "Unable to render response", http.StatusInternalServerError)
		}
	}
}

func (e *Env) deleteHandlerFunc(w http.ResponseWriter, r *http.Request) {
	param := flow.Param(r.Context(), "id")
	val, err := strconv.ParseInt(param, 10, 64)
	if err != nil {
		log.Printf("Unable to convert %s to integer: %s", param, err)
		http.Error(w, "Delete expects an integer path element", http.StatusBadRequest)
		return
	}
	err = deleteTodo(e.db, val)
	if err != nil {
		log.Printf("Unable to delete entry %d: %s", val, err)
		http.Error(w, "Unable to delete entry", http.StatusInternalServerError)
		return
	}

	switch r.Header.Get("Accept") {
	case "application/json":
		w.WriteHeader(http.StatusNoContent)
	default:
		fmt.Fprintf(w, "")
	}
}

func (e *Env) addHandlerFunc(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Printf("Error parsing form: %s", err)
		http.Error(w, "Error parsing add request", http.StatusBadRequest)
		return
	}

	text := r.FormValue("newTodo")
	if text == "" {
		log.Printf("Entry Empty")
		http.Error(w, "Empty todo items are not accepted", http.StatusBadRequest)
		return
	}

	tdid, err := addTodo(e.db, text)
	if err != nil {
		log.Printf("Error writing todo item: %s", err)
		http.Error(w, "Unable to write new entry", http.StatusInternalServerError)
		return
	}

	tdi := TodoItem{tdid, text, false}
	switch r.Header.Get("Accept") {
	case "application/json":
		encoder := json.NewEncoder(w)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(tdi); err != nil {
			log.Printf("Error encoding response to json: %s", err)
			http.Error(w, "Unable to render response", http.StatusInternalServerError)
		}
	default:
		respTemplate := e.templates.Lookup("todoitem.html")
		err = respTemplate.Execute(w, tdi)
		if err != nil {
			log.Printf("Error rendering template: %s", err)
			http.Error(w, "Unable to render response", http.StatusInternalServerError)
		}
	}
}
