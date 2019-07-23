package server

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/jackc/booklog/domain"
	"github.com/jackc/booklog/validate"
	errors "golang.org/x/xerrors"
)

type BookCreate struct {
}

func (action *BookCreate) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	db := ctx.Value(RequestDBKey).(queryExecer)
	username := chi.URLParam(r, "username")
	var readerID int64
	err := db.QueryRow(ctx, "select id from users where username=$1", username).Scan(&readerID)
	if err != nil {
		panic(err)
	}

	cba := domain.CreateBookArgs{
		ReaderID:     readerID,
		Title:        r.FormValue("title"),
		Author:       r.FormValue("author"),
		DateFinished: r.FormValue("dateFinished"),
		Media:        r.FormValue("media"),
	}

	err = domain.CreateBook(ctx, db, cba)
	if err != nil {
		var verr validate.Errors
		if errors.As(err, &verr) {
			err := RenderBookNew(w, baseViewDataFromRequest(r), cba, verr, username)
			if err != nil {
				panic(err)
			}

			return
		}

		panic(err)
	}

	http.Redirect(w, r, BooksPath(username), http.StatusSeeOther)
}
