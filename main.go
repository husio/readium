package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"

	"golang.org/x/net/html"
)

func main() {
	http.Handle("/", &readium{})
	http.ListenAndServe(":8000", nil)
}

type readium struct {
	client http.Client
}

func (rd *readium) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	resp, err := rd.client.Get("https://medium.com" + r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(resp.StatusCode)
	io.WriteString(w, `<!doctype html><body>
<meta name="viewport" content="width=device-width, initial-scale=1">
<style type="text/css">
body{ margin:40px auto; max-width:800px; line-height:1.6; font-size:18px; padding:0 10px; }
h1,h2,h3 { line-height:1.2 }
img { max-height: 400px; max-width: 400px; }
</style>
	`)
	out, _ := extract(resp.Body)
	w.Write(out)
}

func extract(body io.Reader) ([]byte, error) {
	var out bytes.Buffer

	var (
		inArticle    bool
		discardStack []string
	)

	z := html.NewTokenizer(body)
	for {
		switch z.Next() {
		case html.ErrorToken:
			return out.Bytes(), z.Err()
		case html.TextToken:
			if inArticle && len(discardStack) == 0 {
				if _, err := out.Write(z.Text()); err != nil {
					return out.Bytes(), fmt.Errorf("cannot write text: %w", err)
				}
			}
		case html.SelfClosingTagToken:
			t, _ := z.TagName()
			switch tag := string(t); tag {
			case "br", "img":
				if inArticle && len(discardStack) == 0 {
					var attrs []byte
					for {
						k, v, more := z.TagAttr()
						if !more {
							break
						}
						if _, ok := allowedTags[string(k)]; ok {
							attrs = append(attrs, fmt.Sprintf(`%s="%s"`, k, v)...)
						}
					}
					fmt.Fprintf(&out, "<%s %s>\n", tag, attrs)
				}
			default:
			}
		case html.StartTagToken:
			t, _ := z.TagName()
			switch tag := string(t); tag {
			case "article":
				inArticle = true
			case "title", "p", "a", "em", "strong", "div", "span", "section", "h1", "h2", "h3", "blockquote", "figure", "figcaption", "pre", "code":
				if inArticle && len(discardStack) == 0 {
					var attrs []byte
					for {
						k, v, more := z.TagAttr()
						if !more {
							break
						}
						if _, ok := allowedTags[string(k)]; ok {
							attrs = append(attrs, fmt.Sprintf(`%s="%s"`, k, v)...)
						}
					}
					fmt.Fprintf(&out, "<%s %s>\n", tag, attrs)
				}
			default:
				if inArticle {
					discardStack = append(discardStack, string(tag))
				}
			}
		case html.EndTagToken:
			switch tag, _ := z.TagName(); string(tag) {
			case "article":
				inArticle = false
			case "title", "p", "a", "em", "strong", "div", "span", "section", "h1", "h2", "h3", "blockquote", "figure", "figcaption", "pre", "code":
				if inArticle && len(discardStack) == 0 {
					fmt.Fprintf(&out, "</%s>\n", tag)
				}
			default:
				if inArticle {
					if len(discardStack) == 0 {
						log.Printf("cannot discard %q: empty stack", tag)
					} else {
						if last := discardStack[len(discardStack)-1]; last != string(tag) {
							log.Printf("cannot discard %q: stack is %q", tag, discardStack)
						} else {
							discardStack = discardStack[:len(discardStack)-1]
						}
					}
				}
			}
		}
	}
}

var allowedTags = map[string]struct{}{
	"src":   struct{}{},
	"title": struct{}{},
	"role":  struct{}{},
	"href":  struct{}{},
}
