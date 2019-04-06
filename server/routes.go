package server

import "fmt"

func BooksPath() string {
	return "/books"
}

func BookPath(id string) string {
	return fmt.Sprintf("/books/%s", id)
}

func NewBookPath() string {
	return "/books/new"
}
