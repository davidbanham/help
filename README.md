# Knowledgebase

Serves a directory of directories that contain markdown pages and images. Example structure in ./pages and example instantiation below. Supports tagging and pagination on the index.

By default, uses its own vanilla-tailwind style. Can be customised if you want by passing your own template and/or css dir.

```markdown
description: A demo help page
tags:
- demo
- example
- greetings
title: Hello World

**Hello World!**

This is a totally great help page!

## More Things

There are _so_ many more things! You can bang on about them.

And you can have new lines.

Even [links](https://google.com)

* lists
* too

!["and images with alt text!"](./cat_caviar.jpg)
```

```golang
package routes

import (
	"log"

	"github.com/davidbanham/help"
	"github.com/gorilla/mux"
)

var errRes = func(w http.ResponseWriter, r *http.Request, code int, message string, err error) {
  log.Println("Oh no! An error!", code, message, err)
}

func init() {
	help.SetContentPath("./help/")

  // optional
	//help.UseCustomAssets("./assets/")
	//help.UseCustomTemplates("./views/*.html")

  r := mux.NewRouter()

	r.Path("/knowledgebase").
		Methods("GET").
		Handler(http.RedirectHandler("/knowledgebase/index", http.StatusFound))

	r.PathPrefix("/knowledgebase").
		Methods("GET").
		Handler(http.StripPrefix("/knowledgebase", help.Router(&errRes)))
}
```
