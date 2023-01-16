package help

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/ast"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	"github.com/gorilla/mux"
	"gopkg.in/yaml.v2"
)

var cachedTmpl *template.Template

var templateFuncMap = template.FuncMap{
	"noescape": func(str string) template.HTML {
		return template.HTML(str)
	},
	"queryString": func(vals url.Values) template.URL {
		return "?" + template.URL(vals.Encode())
	},
}

var blankline = regexp.MustCompile("(?m)^$")

type ErrorHandler func(w http.ResponseWriter, r *http.Request, code int, message string, err error)

var defaultErrHandler = func(w http.ResponseWriter, r *http.Request, code int, message string, err error) {
	log.Println("WARN", fmt.Sprintf("Sending Error Response: %+v, %+v, %+v, %+v", code, message, r.URL.String(), err))
	w.WriteHeader(code)
	w.Write([]byte(message))
}

var rootDir = "./"

func relativePath(target string) string {
	return rootDir + target
}

func SetPath(path string) {
	rootDir = path
}

type Index []HelpTopic

func (this Index) Paginate(pagination *Pagination) Index {
	if len(this) > pagination.Skip+pagination.Limit {
		pagination.MoreAvailable = true
		return this[pagination.Skip:pagination.Limit]
	} else if len(this) < pagination.Skip {
		return Index{}
	} else {
		return this[pagination.Skip:]
	}
}

var cachedIndex *Index

func index() (Index, error) {
	if cachedIndex != nil {
		return *cachedIndex, nil
	}
	return BuildIndex()
}

func tmpl() *template.Template {
	if cachedTmpl == nil {
		cachedTmpl = template.Must(template.New("main").Funcs(templateFuncMap).ParseGlob(relativePath("/views/*")))
	}
	return cachedTmpl
}

func ServeTopicPage(w http.ResponseWriter, r *http.Request) error {
	name := path.Base(r.URL.Path)

	topic := HelpTopic{
		Name: name,
	}
	if err := topic.Hydrate(); err != nil {
		return err
	}

	if err := tmpl().ExecuteTemplate(w, "topic.html", topicPageData{
		Context:   r.Context(),
		HelpTopic: topic,
	}); err != nil {
		return err
	}

	return nil
}

func ServeHelpPageAsset(w http.ResponseWriter, r *http.Request) error {
	topic, asset := path.Split(r.URL.Path)

	p := relativePath(fmt.Sprintf("pages/%s/%s", topic, asset))

	_, err := os.Stat(p)
	if err != nil {
		return err
	}

	http.ServeFile(w, r, p)

	return nil
}

type topicPageData struct {
	Context context.Context
	HelpTopic
}

type HelpTopic struct {
	Name        string
	Title       string
	Content     string
	Markup      string
	Description string
	Tags        []string
}

func (topic *HelpTopic) StubOut() {
	topic.Content = ""
	topic.Markup = ""
}

func (topic *HelpTopic) Hydrate() error {
	dat, err := ioutil.ReadFile(relativePath("/pages/" + topic.Name + "/page.md"))
	if err != nil {
		return err
	}

	parts := blankline.Split(string(dat), 2)

	if len(parts) != 2 {
		return fmt.Errorf("Invalid topic data")
	}

	if err := yaml.Unmarshal([]byte(parts[0]), &topic); err != nil {
		return err
	}

	topic.Content = parts[1]

	extensions := parser.CommonExtensions | parser.AutoHeadingIDs
	parser := parser.NewWithExtensions(extensions)

	opts := html.RendererOptions{
		Flags:          html.CommonFlags,
		RenderNodeHook: imgUrlPrefixer(topic.Name),
	}
	renderer := html.NewRenderer(opts)

	markup := markdown.ToHTML([]byte(topic.Content), parser, renderer)
	topic.Markup = string(markup)

	return nil
}

func imgUrlPrefixer(slug string) func(w io.Writer, node ast.Node, entering bool) (ast.WalkStatus, bool) {
	return func(w io.Writer, node ast.Node, entering bool) (ast.WalkStatus, bool) {
		// skip all nodes that are not CodeBlock nodes
		if _, ok := node.(*ast.Image); !ok {
			return ast.GoToNext, false
		}

		img := node.(*ast.Image)
		href := string(img.Destination)
		if strings.HasPrefix(href, "./") {
			trimmed := strings.TrimPrefix(href, "./")
			img.Destination = []byte(slug + "/" + trimmed)
		}
		return ast.GoToNext, false
	}
}

func ServeHelpAsset(w http.ResponseWriter, r *http.Request) error {
	p := relativePath("assets/" + r.URL.Path)

	_, err := os.Stat(p)
	if err != nil {
		return err
	}

	http.ServeFile(w, r, p)

	return nil
}

func BuildIndex() (Index, error) {
	ret := Index{}
	info, err := ioutil.ReadDir(relativePath("/pages"))
	if err != nil {
		return ret, err
	}
	for _, file := range info {
		if file.IsDir() {
			topic := HelpTopic{
				Name: file.Name(),
			}

			if err := topic.Hydrate(); err != nil {
				return ret, err
			}

			topic.StubOut()

			ret = append(ret, topic)
		}
	}
	return ret, err
}

func ServeHelpIndex(w http.ResponseWriter, r *http.Request) error {
	ind, err := index()
	if err != nil {
		return err
	}

	pagination := Pagination{
		DefaultPageSize: 10,
	}
	pagination.Paginate(r.Form)

	if r.FormValue("tagged") != "" {
		filtered := ind
		for _, target := range r.Form["tagged"] {
			filtered = filtered.FilterToTag(target)
		}
		return tmpl().ExecuteTemplate(w, "index.html", indexPageData{
			Context:       r.Context(),
			Index:         filtered.Paginate(&pagination),
			Description:   "Everything you need to know",
			ActiveFilters: r.Form["tagged"],
			Title:         "Knowledgebase",
			Pagination:    pagination,
		})
	} else {
		return tmpl().ExecuteTemplate(w, "index.html", indexPageData{
			Context:     r.Context(),
			Index:       ind.Paginate(&pagination),
			Description: "Everything you need to know",
			Title:       "Knowledgebase",
			Pagination:  pagination,
		})
	}
}

func (index Index) FilterToTag(target string) Index {
	filtered := Index{}
	for _, topic := range index {
		for _, tag := range topic.Tags {
			if tag == target {
				filtered = append(filtered, topic)
			}
		}
	}
	return filtered
}

type indexPageData struct {
	Context       context.Context
	Title         string
	Description   string
	Index         Index
	ActiveFilters []string
	Pagination    Pagination
}

func Router(errorHandler *func(w http.ResponseWriter, r *http.Request, code int, message string, err error)) http.Handler {
	var r = mux.NewRouter()

	errRes := defaultErrHandler
	if errorHandler != nil {
		errRes = *errorHandler
	}

	r.Path("/").
		Methods("GET").
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := ServeHelpIndex(w, r); err != nil {
				errRes(w, r, 500, "Problem serving help index", err)
				return
			}
		})

	r.Path("/index").
		Methods("GET").
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := ServeHelpIndex(w, r); err != nil {
				errRes(w, r, 500, "Problem serving help index", err)
				return
			}
		})

	r.PathPrefix("/css").
		Methods("GET").
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := ServeHelpAsset(w, r); err != nil {
				errRes(w, r, 500, "Problem serving help asset", err)
				return
			}
		})

	r.Path("/{name}").
		Methods("GET").
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := ServeTopicPage(w, r); err != nil {
				errRes(w, r, 500, "Problem serving help topic", err)
				return
			}
		})

	r.Path("/{topic}/{asset}").
		Methods("GET").
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := ServeHelpPageAsset(w, r); err != nil {
				if os.IsNotExist(err) {
					errRes(w, r, 404, "Not found", err)
					return
				}
				errRes(w, r, 500, "Problem serving help topic asset", err)
				return
			}
		})

	return r
}
