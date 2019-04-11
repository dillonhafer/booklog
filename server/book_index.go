package server

import (
	"context"
	"html/template"
	"net/http"
	"time"

	"github.com/gorilla/csrf"
)

type BookIndex struct {
	templates *template.Template
}

func (action *BookIndex) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	db := ctx.Value(RequestDBKey).(queryExecer)

	var books []BookRow001
	rows, _ := db.Query(context.Background(), "select id, title, author, date_finished, media from book order by date_finished asc")
	for rows.Next() {
		var b BookRow001
		rows.Scan(&b.ID, &b.Title, &b.Author, &b.DateFinished, &b.Media)
		books = append(books, b)
	}
	if rows.Err() != nil {
		panic(rows.Err())
	}

	tmpl := action.templates.Lookup("book_index")
	err := tmpl.Execute(w, map[string]interface{}{"Books": books, csrf.TemplateTag: csrf.TemplateField(r)})
	if err != nil {
		panic(err)
	}
}

type BookRow001 struct {
	ID           string
	Title        string
	Author       string
	DateFinished time.Time
	Media        string
}
