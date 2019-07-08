package server

import (
	"context"
	"net/http"

	"github.com/gorilla/csrf"
	"github.com/jackc/booklog/validate"
)

type BookCreate struct {
}

func createBook(ctx context.Context, db queryExecer, bcr *BookCreateRequest) error {
	v := validate.New()
	v.Presence("title", bcr.Title)
	v.Presence("author", bcr.Author)
	v.Presence("dateFinished", bcr.DateFinished)
	v.Presence("media", bcr.Media)

	if v.Err() != nil {
		return v.Err()
	}

	_, err := db.Exec(ctx, "insert into book(title, author, date_finished, media) values($1, $2, $3, $4)", bcr.Title, bcr.Author, bcr.DateFinished, bcr.Media)
	if err != nil {
		return err
	}

	return nil
}

func (action *BookCreate) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	db := ctx.Value(RequestDBKey).(queryExecer)

	bcr := &BookCreateRequest{}
	bcr.Title = r.FormValue("title")
	bcr.Author = r.FormValue("author")
	bcr.DateFinished = r.FormValue("dateFinished")
	bcr.Media = r.FormValue("media")

	err := createBook(ctx, db, bcr)
	if err != nil {
		err := RenderBookNew(w, csrf.TemplateField(r), bcr, err)
		if err != nil {
			panic(err)
		}

		if err != nil {
			panic(err)
		}
		return
	}

	http.Redirect(w, r, BooksPath(), http.StatusSeeOther)

}
