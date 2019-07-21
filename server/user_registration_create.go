package server

import (
	"net/http"

	"github.com/gorilla/csrf"
	"github.com/jackc/booklog/domain"
	"github.com/jackc/booklog/validate"
	errors "golang.org/x/xerrors"
)

func UserRegistrationCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	db := ctx.Value(RequestDBKey).(queryExecer)

	rua := domain.RegisterUserArgs{
		Username: r.FormValue("username"),
		Password: r.FormValue("password"),
	}

	err := domain.RegisterUser(ctx, db, rua)
	if err != nil {
		var verr validate.Errors
		if errors.As(err, &verr) {
			err := RenderUserRegistrationNew(w, csrf.TemplateField(r), rua, verr)
			if err != nil {
				panic(err)
			}

			return
		}

		panic(err)
	}

	http.Redirect(w, r, BooksPath(rua.Username), http.StatusSeeOther)
}
