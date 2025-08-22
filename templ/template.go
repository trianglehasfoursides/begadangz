package templ

import (
	"html/template"
	"net/http"
)

var (
	Template *template.Template
	err      error
)

func init() {
	Template, err = template.ParseGlob("html/*.html")
	if err != nil {
		panic(err)
	}
}

func View(w http.ResponseWriter, name string, data map[string]any) error {
	return Template.ExecuteTemplate(w, name+".html", data)
}
