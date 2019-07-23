package server

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/gorilla/csrf"
	"github.com/gorilla/handlers"
	"github.com/gorilla/securecookie"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/hlog"
	"github.com/spf13/viper"
	errors "golang.org/x/xerrors"
)

// Key to use when setting the DB.
type ctxRequestKey int

// RequestDBKey is the key that holds the DB for this request.
const RequestDBKey ctxRequestKey = 0
const RequestSessionKey ctxRequestKey = 1

var sc *securecookie.SecureCookie

type queryExecer interface {
	Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, optionsAndArgs ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, optionsAndArgs ...interface{}) pgx.Row
}

type Session struct {
	ID              [16]byte
	Username        string
	IsAuthenticated bool
}

func Serve(listenAddress string, csrfKey []byte, insecureDevMode bool, cookieHashKey []byte, cookieBlockKey []byte) {
	log := zerolog.New(os.Stdout).With().
		Timestamp().
		Logger()

	sc = securecookie.New(cookieHashKey, cookieBlockKey)

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)

	r.Use(handlers.HTTPMethodOverrideHandler)

	r.Use(middleware.Logger)

	r.Use(hlog.NewHandler(log))
	r.Use(hlog.URLHandler("url"))

	r.Use(middleware.Recoverer)

	CSRF := csrf.Protect(csrfKey, csrf.Secure(!insecureDevMode))
	r.Use(CSRF)

	err := LoadTemplates("html")
	if err != nil {
		panic(err)
	}

	dbpool, err := pgxpool.Connect(context.Background(), viper.GetString("database_url"))
	if err != nil {
		panic(err)
	}
	r.Use(func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			ctx = context.WithValue(ctx, RequestDBKey, dbpool)
			next.ServeHTTP(w, r.WithContext(ctx))
		}

		return http.HandlerFunc(fn)
	})

	r.Use(func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			session := &Session{}
			ctx = context.WithValue(ctx, RequestSessionKey, session)

			cookie, err := r.Cookie("booklog-session-id")
			if err != nil {
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			var sessionID [16]byte
			err = sc.Decode("booklog-session-id", cookie.Value, &sessionID)
			if err != nil {
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			db := ctx.Value(RequestDBKey).(queryExecer)
			err = db.QueryRow(ctx,
				"select user_sessions.id, users.username from user_sessions join users on user_sessions.user_id=users.id where user_sessions.id=$1",
				sessionID,
			).Scan(&session.ID, &session.Username)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					// invalid session ID
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				} else {
					panic("unknown error finding session")
				}
			}
			session.IsAuthenticated = true

			next.ServeHTTP(w, r.WithContext(ctx))
		}

		return http.HandlerFunc(fn)
	})

	// r.Method("GET", "/", &BookIndex{})
	r.Method("GET", "/user_registration/new", &Endpoint{DB: dbpool, Handler: UserRegistrationNew})
	r.Method("POST", "/user_registration", &Endpoint{DB: dbpool, Handler: UserRegistrationCreate})

	r.Method("GET", "/login", &Endpoint{DB: dbpool, Handler: UserLoginForm})
	r.Method("POST", "/login/handle", &Endpoint{DB: dbpool, Handler: UserLogin})

	r.Method("POST", "/logout", &Endpoint{DB: dbpool, Handler: UserLogout})

	r.Method("GET", "/users/{username}/books", &Endpoint{DB: dbpool, Handler: BookIndex})
	r.Method("GET", "/users/{username}/books/new", &Endpoint{DB: dbpool, Handler: BookNew})
	r.Method("POST", "/users/{username}/books", &Endpoint{DB: dbpool, Handler: BookCreate})
	r.Method("GET", "/users/{username}/books/{id}/edit", &Endpoint{DB: dbpool, Handler: BookEdit})
	r.Method("PATCH", "/users/{username}/books/{id}", &Endpoint{DB: dbpool, Handler: BookUpdate})
	r.Method("DELETE", "/users/{username}/books/{id}", &Endpoint{DB: dbpool, Handler: BookDelete})
	r.Method("GET", "/users/{username}/books/import_csv/form", &Endpoint{DB: dbpool, Handler: BookImportCSVForm})
	r.Method("POST", "/users/{username}/books/import_csv", &Endpoint{DB: dbpool, Handler: BookImportCSV})

	fileServer(r, "/static", http.Dir("build/static"))

	http.ListenAndServe(listenAddress, r)
}

func fileServer(r chi.Router, path string, root http.FileSystem) {
	if strings.ContainsAny(path, "{}*") {
		panic("FileServer does not permit URL parameters.")
	}

	fs := http.StripPrefix(path, http.FileServer(root))

	if path != "/" && path[len(path)-1] != '/' {
		r.Get(path, http.RedirectHandler(path+"/", 301).ServeHTTP)
		path += "/"
	}
	path += "*"

	r.Get(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fs.ServeHTTP(w, r)
	}))
}
