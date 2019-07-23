package server

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/jackc/booklog/domain"
)

type BookEdit struct {
}

func (action *BookEdit) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	db := ctx.Value(RequestDBKey).(queryExecer)
	username := chi.URLParam(r, "username")
	bookID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		panic(err)
	}

	uba := domain.UpdateBookArgs{}
	err = db.QueryRow(ctx, "select title, author, finish_date::text, media from books where id=$1", bookID).Scan(&uba.Title, &uba.Author, &uba.DateFinished, &uba.Media)
	// TODO - handle not found error
	// if len(result.Rows) == 0 {
	// 	http.NotFound(w, r)
	// 	return
	// }
	if err != nil {
		panic(err)
	}

	err = RenderBookEdit(w, baseViewDataFromRequest(r), bookID, uba, nil, username)
	if err != nil {
		panic(err)
	}
}
