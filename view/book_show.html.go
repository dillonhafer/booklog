package view

import (
	"html"
	"io"

	"github.com/jackc/booklog/data"
	"github.com/jackc/booklog/route"
)

func BookShow(w io.Writer, bva *BaseViewArgs, book *data.Book) error {
	LayoutHeader(w, bva)
	io.WriteString(w, `
<div class="card">
    <dl>
      <dt>Title</dt>
      <dd>`)
	io.WriteString(w, html.EscapeString(book.Title))
	io.WriteString(w, `</dd>
      <dt>Author</dt>
      <dd>`)
	io.WriteString(w, html.EscapeString(book.Author))
	io.WriteString(w, `</dd>
      <dt>Finish Date</dt>
      <dd>`)
	io.WriteString(w, html.EscapeString(book.FinishDate.Format("January 2, 2006")))
	io.WriteString(w, `</dd>
      <dt>Format</dt>
      <dd>`)
	io.WriteString(w, html.EscapeString(book.Format))
	io.WriteString(w, `</dd>
      <dt>Location</dt>
      `)
	if book.Location == "" {
		io.WriteString(w, `
        <dd class="empty">None</dd>
      `)
	} else {
		io.WriteString(w, `
        <dd>`)
		io.WriteString(w, html.EscapeString(book.Location))
		io.WriteString(w, `</dd>
      `)
	}
	io.WriteString(w, `
    </dl>

    <a class="title" href="`)
	io.WriteString(w, html.EscapeString(route.EditBookPath(bva.PathUser.Username, book.ID)))
	io.WriteString(w, `">Edit</a>
    <a class="title" href="`)
	io.WriteString(w, html.EscapeString(route.BookConfirmDeletePath(bva.PathUser.Username, book.ID)))
	io.WriteString(w, `">Delete</a>
  </div>
`)
	LayoutFooter(w, bva)
	io.WriteString(w, `
`)

	return nil
}
