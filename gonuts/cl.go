/*
Code based on `issue` command by Russ Cox:
http://code.google.com/p/rsc/source/browse/cmd/issue/issue.go

Google Code Issue Tracker API:
http://code.google.com/p/support/wiki/IssueTrackerAPI
*/

package gonuts

import (
	"bytes"
	"encoding/xml"
	"log"
	"io"
	"net/http"
	"strings"
	"text/template"
	"time"

	"github.com/kylelemons/blightbot/commander"
)

/*
// These are a subset of what's used by ISSUE, so we don't need separate ones.

type Feed struct {
	Entry []Entry `xml:"entry"`
}

type Entry struct {
	ID        string    `xml:"id"`
	Updated   time.Time `xml:"updated"`
	Title     string    `xml:"title"`
	Links     []Link    `xml:"link"`
	Author    Person    `xml:"author"`
}

type Person struct {
	Name     string `xml:"name"`
	UserName string `xml:"username"`
}

type Link struct {
	Type string `xml:"type,attr"`
	URL  string `xml:"href,attr"`
}
*/

// ProjectID is the Google Code Project Identifier whose issues should be queried
//var ProjectID = "go"
var ProjectVCS = "hg"

var cltemplates = map[string]*template.Template{
	"latest": template.Must(template.New("latest").Funcs(funcs).Parse(`{{with .Latest}}{{.Title}}
Detail: {{.Links|link}}{{end}}`)),
	"summary": template.Must(template.New("id").Funcs(funcs).Parse(`{{$comma := ""}}{{with .Summary}}Last 24 hours: {{.Daily}}{{.Plus}} changelists:
Authors{{range $auth, $cnt := .Authors}} | {{$auth}} ({{$cnt}} CLs){{end}}{{end}}`)),
}

var CL = commander.Cmd("cl", func(src *commander.Source, resp *commander.Response, cmd string, args []string) {
	// Reasonable default is private
	resp.Private()

	if len(args) < 1 {
		args = append(args, "")
	}

	cmd = strings.ToLower(args[0])
	log.Printf("CMD %q, ARGS %v", cmd, args)
	switch cmd {
	case "latest":
	case "summary":
	case "detail":
	default:
		// Well, we don't know what we're being asked to do...
		resp.Private()
		resp.Printf("Usage: CL {latest|summary}")
		return
	}

	u := "https://code.google.com/feeds/p/" + ProjectID + "/" + ProjectVCS + "changes/basic"
	r, err := http.Get(u)
	if err != nil {
		resp.Public()
		resp.Printf("Sorry, `cl` seems to be having issues...")
		log.Printf("http get: %s", err)
		return
	}
	defer r.Body.Close()

	x := new(bytes.Buffer)
	io.Copy(x, r.Body)

	var feed Feed
	if err := xml.NewDecoder(x).Decode(&feed); err != nil {
		resp.Public()
		resp.Printf("Sorry, `cl` seems to be having issues...")
		log.Printf("xml decode: %s", err)
		return
	}

	var data struct {
		Summary struct {
			Daily   int
			Plus    string
			Authors map[string]int
		}
		Latest Entry
	}

	data.Summary.Plus = "+"
	data.Summary.Authors = map[string]int{}

	for entryIdx, e := range feed.Entry {
		if entryIdx == 0 {
			title := e.Title

			// Split at translated newline
			if idx := strings.Index(title, "  "); idx >= 0 {
				title = title[:idx]
			}

			// Split at weird chars (usually a .)
			if idx := strings.IndexFunc(title, func(r rune) bool {
				switch {
				case r >= 'a' && r <= 'z':
					return false
				case r >= 'A' && r <= 'Z':
					return false
				case r >= '0' && r <= '9':
					return false
				case r == '-' || r == '_':
					return false
				case r == ':' || r == ' ':
					return false
				}
				return true
			}); idx >= 0 {
				title = title[:idx]
			}

			data.Latest = e
			data.Latest.Title = title
		}

		if time.Since(e.Updated) > 24*time.Hour {
			data.Summary.Plus = ""
			break
		}

		name := e.Author.Name
		if idx := strings.Index(name, " <"); idx >= 0 {
			name = name[:idx]
		}

		data.Summary.Authors[name]++
		data.Summary.Daily++
	}

	b := new(bytes.Buffer)
	if t, ok := cltemplates[cmd]; ok {
		if err := t.Execute(b, data); err != nil {
			resp.Public()
			resp.Printf("Sorry, `cl` seems to be having issues...")
			log.Printf("execute: %s", err)
			return
		}
		resp.Public()
		for lineno, line := range strings.Split(b.String(), "\n") {
			if lineno == 2 {
				resp.Printf("... (see online for more)")
				return
			}
			resp.Printf(line)
		}
	} else {
		resp.Printf("Sorry, `cl` seems to be having, well, issues...")
		log.Printf("couldn't find template for %q...", cmd)
		return
	}
}).Help(`List or summarize recent commits

Usage:
	CL latest
		Retrieve the latest issue and print its title

	CL summary
		Retrieve recent CLs and summarize them
`)
