package server

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jackc/booklog/data"
	"github.com/jackc/booklog/route"
	"github.com/jackc/booklog/validate"
	"github.com/jackc/booklog/view"
	"github.com/jackc/pgx/v4"
	errors "golang.org/x/xerrors"
)

type BookEditForm struct {
	Title      string
	Author     string
	FinishDate string
	Media      string
}

func (f BookEditForm) Parse() (data.Book, validate.Errors) {
	var err error
	book := data.Book{
		Title:  f.Title,
		Author: f.Author,
		Media:  f.Media,
	}
	v := validate.New()

	book.FinishDate, err = time.Parse("2006-01-02", f.FinishDate)
	if err != nil {
		book.FinishDate, err = time.Parse("1/2/2006", f.FinishDate)
		if err != nil {
			v.Add("finishDate", errors.New("is not a date"))
		}
	}

	if v.Err() != nil {
		return book, v.Err().(validate.Errors)
	}

	return book, nil
}

func BookIndex(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	db := ctx.Value(RequestDBKey).(queryExecer)
	pathUser := ctx.Value(RequestPathUserKey).(*data.UserMin)

	books, err := data.GetAllBooks(ctx, db, pathUser.ID)
	if err != nil {
		InternalServerErrorHandler(w, r, err)
		return
	}

	yearBooksLists := make([]*view.YearBookList, 0)
	var ybl *view.YearBookList

	for _, book := range books {
		year := book.FinishDate.Year()
		if ybl == nil || year != ybl.Year {
			ybl = &view.YearBookList{Year: year}
			yearBooksLists = append(yearBooksLists, ybl)
		}

		ybl.Books = append(ybl.Books, book)
	}

	err = view.BookIndex(w, baseViewArgsFromRequest(r), yearBooksLists)
	if err != nil {
		InternalServerErrorHandler(w, r, err)
		return
	}
}

type BooksForYear struct {
	Year  int
	Books []BookRow001
}

type BookRow001 struct {
	ID           int64
	Title        string
	Author       string
	DateFinished time.Time
	Media        string
}

func BookNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pathUser := ctx.Value(RequestPathUserKey).(*data.UserMin)

	var form BookEditForm
	err := RenderBookNew(w, baseViewDataFromRequest(r), form, nil, pathUser.Username)
	if err != nil {
		InternalServerErrorHandler(w, r, err)
		return
	}
}

func BookCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	db := ctx.Value(RequestDBKey).(queryExecer)
	pathUser := ctx.Value(RequestPathUserKey).(*data.UserMin)

	form := BookEditForm{
		Title:      r.FormValue("title"),
		Author:     r.FormValue("author"),
		FinishDate: r.FormValue("finishDate"),
		Media:      r.FormValue("media"),
	}
	attrs, verr := form.Parse()
	if verr != nil {
		err := RenderBookNew(w, baseViewDataFromRequest(r), form, verr, pathUser.Username)
		if err != nil {
			InternalServerErrorHandler(w, r, err)
		}
		return
	}
	attrs.UserID = pathUser.ID

	book, err := data.CreateBook(ctx, db, attrs)
	if err != nil {
		var verr validate.Errors
		if errors.As(err, &verr) {
			err := RenderBookNew(w, baseViewDataFromRequest(r), form, verr, pathUser.Username)
			if err != nil {
				InternalServerErrorHandler(w, r, err)
			}
			return
		}

		InternalServerErrorHandler(w, r, err)
		return
	}

	http.Redirect(w, r, route.BookPath(pathUser.Username, book.ID), http.StatusSeeOther)
}

func BookConfirmDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	db := ctx.Value(RequestDBKey).(queryExecer)
	pathUser := ctx.Value(RequestPathUserKey).(*data.UserMin)
	bookID := int64URLParam(r, "id")

	book, err := data.GetBook(ctx, db, bookID)
	if err != nil {
		var nfErr data.NotFoundError
		if errors.As(err, nfErr) {
			NotFoundHandler(w, r)
		} else {
			InternalServerErrorHandler(w, r, err)
		}
		return
	}

	err = RenderBookConfirmDelete(w, baseViewDataFromRequest(r), book, pathUser.Username)
	if err != nil {
		InternalServerErrorHandler(w, r, err)
		return
	}
}

func BookDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	db := ctx.Value(RequestDBKey).(queryExecer)
	pathUser := ctx.Value(RequestPathUserKey).(*data.UserMin)
	bookID := int64URLParam(r, "id")

	err := data.DeleteBook(ctx, db, bookID)
	if err != nil {
		var nfErr data.NotFoundError
		if errors.As(err, nfErr) {
			NotFoundHandler(w, r)
		} else {
			InternalServerErrorHandler(w, r, err)
		}
		return
	}

	http.Redirect(w, r, route.BooksPath(pathUser.Username), http.StatusSeeOther)
}

func BookShow(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	db := ctx.Value(RequestDBKey).(queryExecer)
	pathUser := ctx.Value(RequestPathUserKey).(*data.UserMin)
	bookID := int64URLParam(r, "id")

	book, err := data.GetBook(ctx, db, bookID)
	if err != nil {
		var nfErr data.NotFoundError
		if errors.As(err, nfErr) {
			NotFoundHandler(w, r)
		} else {
			InternalServerErrorHandler(w, r, err)
		}
		return
	}

	err = RenderBookShow(w, baseViewDataFromRequest(r), book, pathUser.Username)
	if err != nil {
		InternalServerErrorHandler(w, r, err)
		return
	}
}

func BookEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	db := ctx.Value(RequestDBKey).(queryExecer)
	pathUser := ctx.Value(RequestPathUserKey).(*data.UserMin)
	bookID := int64URLParam(r, "id")

	var form BookEditForm
	var FinishDate time.Time
	err := db.QueryRow(ctx, "select title, author, finish_date, media from books where id=$1 and user_id=$2", bookID, pathUser.ID).
		Scan(&form.Title, &form.Author, &FinishDate, &form.Media)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			NotFoundHandler(w, r)
		} else {
			InternalServerErrorHandler(w, r, err)
		}
		return
	}
	form.FinishDate = FinishDate.Format("2006-01-02")

	err = RenderBookEdit(w, baseViewDataFromRequest(r), bookID, form, nil, pathUser.Username)
	if err != nil {
		InternalServerErrorHandler(w, r, err)
		return
	}
}

func BookUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	db := ctx.Value(RequestDBKey).(queryExecer)
	pathUser := ctx.Value(RequestPathUserKey).(*data.UserMin)
	bookID := int64URLParam(r, "id")

	form := BookEditForm{
		Title:      r.FormValue("title"),
		Author:     r.FormValue("author"),
		FinishDate: r.FormValue("finishDate"),
		Media:      r.FormValue("media"),
	}
	attrs, verr := form.Parse()
	if verr != nil {
		err := RenderBookEdit(w, baseViewDataFromRequest(r), bookID, form, verr, pathUser.Username)
		if err != nil {
			InternalServerErrorHandler(w, r, err)
		}
		return
	}
	attrs.ID = bookID

	err := data.UpdateBook(ctx, db, attrs)
	if err != nil {
		var verr validate.Errors
		if errors.As(err, &verr) {
			err := RenderBookEdit(w, baseViewDataFromRequest(r), bookID, form, verr, pathUser.Username)
			if err != nil {
				InternalServerErrorHandler(w, r, err)
			}
			return
		}

		var nfErr data.NotFoundError
		if errors.As(err, nfErr) {
			NotFoundHandler(w, r)
		} else {
			InternalServerErrorHandler(w, r, err)
		}
		return
	}

	http.Redirect(w, r, route.BookPath(pathUser.Username, bookID), http.StatusSeeOther)
}

func BookImportCSVForm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pathUser := ctx.Value(RequestPathUserKey).(*data.UserMin)

	err := RenderBookImportCSVForm(w, baseViewDataFromRequest(r), pathUser.Username)
	if err != nil {
		InternalServerErrorHandler(w, r, err)
		return
	}
}

func BookImportCSV(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	db := ctx.Value(RequestDBKey).(queryExecer)
	pathUser := ctx.Value(RequestPathUserKey).(*data.UserMin)

	r.ParseMultipartForm(10 << 20)

	file, _, err := r.FormFile("file")
	if err != nil {
		InternalServerErrorHandler(w, r, err)
		return
	}
	defer file.Close()

	err = importBooksFromCSV(ctx, db, pathUser.ID, file)
	if err != nil {
		InternalServerErrorHandler(w, r, err)
		return
	}

	http.Redirect(w, r, route.BooksPath(pathUser.Username), http.StatusSeeOther)
}

// TODO - need DB transaction control - so queryExecer is insufficient
func importBooksFromCSV(ctx context.Context, db queryExecer, ownerID int64, r io.Reader) error {
	records, err := csv.NewReader(r).ReadAll()
	if err != nil {
		return err
	}

	if len(records) < 2 {
		return errors.New("CSV must have at least 2 rows")
	}

	if len(records[0]) < 4 {
		return errors.New("CSV must have at least 4 columns")
	}

	for i, record := range records[1:] {
		form := BookEditForm{
			Title:      record[0],
			Author:     record[1],
			FinishDate: record[2],
			Media:      record[3],
		}
		if form.Media == "" {
			form.Media = "book"
		}

		attrs, verr := form.Parse()
		if verr != nil {
			return errors.Errorf("row %d: %w", i+1, verr)
		}
		attrs.UserID = ownerID

		_, err := data.CreateBook(ctx, db, attrs)
		if err != nil {
			return errors.Errorf("row %d: %w", i+1, err)
		}
	}

	return nil
}

func BookExportCSV(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	db := ctx.Value(RequestDBKey).(queryExecer)
	pathUser := ctx.Value(RequestPathUserKey).(*data.UserMin)

	buf := &bytes.Buffer{}
	csvWriter := csv.NewWriter(buf)
	csvWriter.Write([]string{"title", "author", "finish_date", "media"})

	rows, _ := db.Query(ctx, `select title, author, finish_date, media
from books
where user_id=$1
order by finish_date desc`, pathUser.ID)
	for rows.Next() {
		var title, author, media string
		var finishDate time.Time
		rows.Scan(&title, &author, &finishDate, &media)
		csvWriter.Write([]string{title, author, finishDate.Format("2006-01-02"), media})
	}
	if rows.Err() != nil {
		InternalServerErrorHandler(w, r, rows.Err())
		return
	}

	csvWriter.Flush()
	if csvWriter.Error() != nil {
		InternalServerErrorHandler(w, r, csvWriter.Error())
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=booklog-%s.csv", pathUser.Username))
	_, err := buf.WriteTo(w)
	if err != nil {
		InternalServerErrorHandler(w, r, err)
		return
	}
}
