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
	_ "github.com/mattn/go-sqlite3" // Import sqlite3 driver
)

//go:embed templates/*.html
var templateFiles embed.FS

//go:embed static
var staticFiles embed.FS

func main() {
	var err error
	var bind string

	// Determine the bind address from command line arguments.
	switch len(os.Args) {
	case 1:
		// If no arguments are provided, bind to port 8080.
		bind = ":8080"
	case 2:
		// If one argument is provided, use it as the bind address.
		bind = os.Args[1]
	default:
		// If more than one argument is provided, print usage and exit.
		log.Fatalf("Usage: %s [bind]", os.Args[0])
	}

	// Create a new Flow router.
	mux := flow.New()
	// Add a logging middleware to the router.
	mux.Use(loggingHandler)

	// Open a connection to the SQLite database.
	db, err := sql.Open("sqlite3", "./todo.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Check if the database connection is working.
	if err = db.Ping(); err != nil {
		log.Fatal(err)
	}

	// Create the database table if it doesn't exist.
	if err = createDatabase(db); err != nil {
		log.Fatal(err)
	}

	// Parse the HTML templates.
	templates := template.Must(template.New("web").ParseFS(templateFiles, "templates/*"))
	// Create a new environment with the database connection and templates.
	handlers := &Env{db, templates}

	// HTML and API methods (if Accept is set to application/json)
	mux.HandleFunc("/", handlers.indexHandlerFunc, "GET")
	mux.HandleFunc("/delete/:id|^[0-9]+$", handlers.deleteHandlerFunc, "DELETE")
	mux.HandleFunc("/do/:id|^[0-9]+$", handlers.doHandlerFunc, "GET")
	mux.HandleFunc("/undo/:id|^[0-9]+$", handlers.undoHandlerFunc, "GET")
	mux.HandleFunc("/add", handlers.addHandlerFunc, "POST")

	// Static files
	mux.Handle("/static/...", http.FileServer(http.FS(staticFiles)))

	// Create a new HTTP server.
	server := &http.Server{
		Addr:    bind,
		Handler: mux,
	}

	// Start the server.
	log.Printf("Server starting on %s", bind)
	log.Fatal(server.ListenAndServe())
}

// Env holds the database connection and templates.
type Env struct {
	db        *sql.DB
	templates *template.Template
}

// indexHandlerFunc handles the index page.
func (e *Env) indexHandlerFunc(w http.ResponseWriter, r *http.Request) {
	// Get the list of todos from the database.
	dtl, err := getTodos(e.db)
	if err != nil {
		log.Printf("Error getting todo list: %s", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Respond with JSON if the Accept header is set to application/json.
	switch r.Header.Get("Accept") {
	case "application/json":
		encoder := json.NewEncoder(w)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(dtl); err != nil {
			log.Printf("Error encoding response to json: %s", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
	default:
		// Otherwise, render the index template.
		indexTemplate := e.templates.Lookup("index.html")
		if err := indexTemplate.Execute(w, dtl); err != nil {
			log.Printf("Error rendering template: %s", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
	}
}

// updateTodoStatusAndGetItem updates the done status of a todo item and retrieves the updated item.
func (e *Env) updateTodoStatusAndGetItem(id int64, done bool) (TodoItem, error) {
	var err error
	if done {
		err = markTodoDone(e.db, id)
	} else {
		err = markTodoNotDone(e.db, id)
	}
	if err != nil {
		return TodoItem{}, fmt.Errorf("unable to update entry %d status to %t: %w", id, done, err)
	}

	tdi, err := getOneTodo(e.db, id)
	if err != nil {
		return TodoItem{}, fmt.Errorf("unable to retrieve updated todo item %d: %w", id, err)
	}
	return tdi, nil
}

// renderTodoItemResponse renders a single todo item as JSON or HTML.
func (e *Env) renderTodoItemResponse(w http.ResponseWriter, r *http.Request, tdi TodoItem) {
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
		if err := respTemplate.Execute(w, tdi); err != nil {
			log.Printf("Error rendering template: %s", err)
			http.Error(w, "Unable to render response", http.StatusInternalServerError)
		}
	}
}

// doHandlerFunc handles marking a todo as done.
func (e *Env) doHandlerFunc(w http.ResponseWriter, r *http.Request) {
	param := flow.Param(r.Context(), "id")
	val, err := strconv.ParseInt(param, 10, 64)
	if err != nil {
		log.Printf("Unable to convert %s to integer: %s", param, err)
		http.Error(w, "Markdone expects an integer path element", http.StatusBadRequest)
		return
	}

	tdi, err := e.updateTodoStatusAndGetItem(val, true)
	if err != nil {
		log.Printf("Error marking entry %d done: %s", val, err)
		http.Error(w, "Unable to mark entry as done", http.StatusInternalServerError)
		return
	}

	e.renderTodoItemResponse(w, r, tdi)
}

// undoHandlerFunc handles marking a todo as not done.
func (e *Env) undoHandlerFunc(w http.ResponseWriter, r *http.Request) {
	param := flow.Param(r.Context(), "id")
	val, err := strconv.ParseInt(param, 10, 64)
	if err != nil {
		log.Printf("Unable to convert %s to integer: %s", param, err)
		http.Error(w, "Markundone expects an integer path element", http.StatusBadRequest)
		return
	}

	tdi, err := e.updateTodoStatusAndGetItem(val, false)
	if err != nil {
		log.Printf("Error marking entry %d not done: %s", val, err)
		http.Error(w, "Unable to mark entry as not done", http.StatusInternalServerError)
		return
	}

	e.renderTodoItemResponse(w, r, tdi)
}

// deleteHandlerFunc handles deleting a todo.
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

	// Respond with a 204 No Content status code if the Accept header is set to application/json.
	switch r.Header.Get("Accept") {
	case "application/json":
		w.WriteHeader(http.StatusNoContent)
	default:
		// Otherwise, respond with an empty string.
		fmt.Fprintf(w, "")
	}
}

// addHandlerFunc handles adding a new todo.
func (e *Env) addHandlerFunc(w http.ResponseWriter, r *http.Request) {
	// Parse the form data.
	err := r.ParseForm()
	if err != nil {
		log.Printf("Error parsing form: %s", err)
		http.Error(w, "Error parsing add request", http.StatusBadRequest)
		return
	}

	// Get the text of the new todo from the form.
	text := r.FormValue("newTodo")
	if text == "" {
		log.Printf("Entry Empty")
		http.Error(w, "Empty todo items are not accepted", http.StatusBadRequest)
		return
	}

	// Add the todo to the database.
	tdid, err := addTodo(e.db, text)
	if err != nil {
		log.Printf("Error writing todo item: %s", err)
		http.Error(w, "Unable to write new entry", http.StatusInternalServerError)
		return
	}

	// Create a new TodoItem.
	tdi := TodoItem{tdid, text, false}
	// Respond with JSON if the Accept header is set to application/json.
	switch r.Header.Get("Accept") {
	case "application/json":
		encoder := json.NewEncoder(w)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(tdi); err != nil {
			log.Printf("Error encoding response to json: %s", err)
			http.Error(w, "Unable to render response", http.StatusInternalServerError)
		}
	default:
		// Otherwise, render the todoitem template.
		respTemplate := e.templates.Lookup("todoitem.html")
		if err := respTemplate.Execute(w, tdi); err != nil {
			log.Printf("Error rendering template: %s", err)
			http.Error(w, "Unable to render response", http.StatusInternalServerError)
		}
	}
}
