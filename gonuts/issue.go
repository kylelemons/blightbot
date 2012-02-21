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
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/kylelemons/blightbot/commander"
)

type Feed struct {
	Entry []Entry `xml:"entry"`
}

type Entry struct {
	ID        string    `xml:"id"`
	Title     string    `xml:"title"`
	Published time.Time `xml:"published"`
	Updated   time.Time `xml:"updated"`
	Links     []Link    `xml:"link"`
	Content   string    `xml:"content"`
	Updates   []Update  `xml:"updates"`
	Author    Person    `xml:"author"`
	Owner     Person    `xml:"owner"`
	Stars     int       `xml:"stars"`
	Status    string    `xml:"status"`
	Label     []string  `xml:"label"`
}

type Person struct {
	Name     string `xml:"name"`
	UserName string `xml:"username"`
}

type Link struct {
	Type string `xml:"type,attr"`
	URL  string `xml:"href,attr"`
}

type Update struct {
	Summary string `xml:"summary"`
	Owner   string `xml:"ownerUpdate"`
	Label   string `xml:"label"`
	Status  string `xml:"status"`
}

// ByLatest sorts a []Entry by (reverse) update time
type ByLatest []Entry

func (e ByLatest) Len() int           { return len(e) }
func (e ByLatest) Swap(i, j int)      { e[i], e[j] = e[j], e[i] }
func (e ByLatest) Less(i, j int) bool { return !e[i].Updated.Before(e[j].Updated) }

// ProjectID is the Google Code Project Identifier whose issues should be queried
var ProjectID = "go"

var ShortQueries = map[string]string{
	"go1":        "Priority=Go1",
	"triage":     "Priority=Triage",
	"later":      "Priority=Later",
	"someday":    "Priority=Someday",
	"new":        "Status=New",
	"accepted":   "Status=Accepted",
	"started":    "Status=Started",
	"waiting":    "Status=WaitingForReply",
	"thinking":   "Status=HelpWanted",
	"helpwanted": "Status=Thinking",
	"longterm":   "Status=LongTerm",
}

var funcs = template.FuncMap{
	"wrap":  wrap,
	"first": firstline,
	"link":  findlink,
}

var issuetemplates = map[string]*template.Template{
	"search": template.Must(template.New("search").Funcs(funcs).Parse("[issue {{.ID}}] {{.Title}}")),
	"id":     template.Must(template.New("id").Funcs(funcs).Parse("[issue {{.ID}}] {{.Label}} {{.Title}}\nDetail: {{.Links|link}} -- !issue detail {{.ID}}")),
	"detail": template.Must(template.New("detail").Funcs(funcs).Parse(`[issue {{.ID}}] {{.Title}}
Status:    {{range .Label}}{{.}} {{end}}{{.Status}} ({{.Stars}} stars)
{{if .Author}}Author:    {{.Author.Name}}
{{end}}{{if .Owner}}Owner:     {{.Owner.UserName}}
{{end}}Updated:   {{.Updated}}
{{.Content|wrap}}`)),
}

var Issue = commander.Cmd("issue", func(src *commander.Source, resp *commander.Response, cmd string, args []string) {
	if len(args) < 1 {
		args = append(args, "")
	}

	query := url.Values{}

	cmd = strings.ToLower(args[0])
	log.Printf("CMD %q, ARGS %v", cmd, args)
	switch cmd {
	case "search":
		if len(args) < 2 {
			resp.Private()
			resp.Printf("Usage: ISSUE search <query>")
			return
		}
		query.Set("can", "open")
		query.Set("max-results", "500")
		query.Set("q", strings.Join(args[1:], " "))
	case "detail":
		if len(args) < 2 {
			resp.Private()
			resp.Printf("Usage: ISSUE detail #####")
			return
		}
		query.Set("id", args[1])
		query.Set("max-results", "1")
	default:
		// First, try an ID
		if _, err := strconv.Atoi(cmd); err == nil {
			query.Set("id", cmd)
			cmd = "id"
			break
		}

		// Next, try an abbreviated query
		if q, ok := ShortQueries[cmd]; ok {
			query.Set("can", "open")
			query.Set("max-results", "500")
			query.Set("q", q)
			cmd = "search"
			break
		}

		// Well, we don't know what we're being asked to do...
		resp.Private()
		resp.Printf("Usage: ISSUE {#####|search|detail|<status>|<priority>} ...")
		return
	}

	u := "https://code.google.com/feeds/issues/p/" + ProjectID + "/issues/full?" + query.Encode()
	log.Printf("Issue search: %q", u)
	r, err := http.Get(u)
	if err != nil {
		resp.Public()
		resp.Printf("Sorry, `issue` seems to be having, well, issues...")
		log.Printf("http get: %s", err)
		return
	}
	defer r.Body.Close()

	var feed Feed
	if err := xml.NewDecoder(r.Body).Decode(&feed); err != nil {
		resp.Public()
		resp.Printf("Sorry, `issue` seems to be having, well, issues...")
		log.Printf("xml decode: %s", err)
		return
	}

	sort.Sort(ByLatest(feed.Entry))
	for entryIdx, e := range feed.Entry {
		if entryIdx >= 5 {
			break
		}

		b := new(bytes.Buffer)
		if t, ok := issuetemplates[cmd]; ok {
			if err := t.Execute(b, e); err != nil {
				resp.Printf("Sorry, `issue` seems to be having, well, issues...")
				log.Printf("template execute: %s", err)
				return
			}
			switch cmd {
			case "detail":
				resp.Private()
			default:
				resp.Public()
			}
			for lineno, line := range strings.Split(b.String(), "\n") {
				if lineno == 10 {
					resp.Printf("... (see online for more)")
					return
				}
				resp.Printf(line)
			}
		} else {
			resp.Printf("Sorry, `issue` seems to be having, well, issues...")
			log.Printf("couldn't find template for %q...", cmd)
			return
		}
	}
}).Help(`List or search Go issues

Usage:
	ISSUE <issue#>
		Retrieve the issue and print its URL and a short description

	ISSUE search <query>
		Search for recent (open) issues matching the query

	ISSUE detail <issue#>
		Query (privately) details about the issue

	ISSUE {<label>|<priority>}
		Query (privately) summaries of the latest 5 matching issue
`)

func firstline(t string) string {
	return strings.Split(t, "\n")[0]
}

func wrap(t string) string {
	t = strings.Replace(t, "\r\n", "\n", -1)
	lines := strings.Split(t, "\n")

	var out []string
	for _, s := range lines {
		for len(s) > 70 {
			i := strings.LastIndex(s[:70], " ")
			if i < 0 {
				i = 69
			}
			i++
			out = append(out, s[:i])
			s = s[i:]
		}
		if len(s) > 0 {
			out = append(out, s)
		}
	}
	return strings.Join(out, "\n")
}

func findlink(links []Link) string {
	for _, link := range links {
		if link.Type == "text/html" {
			return link.URL
		}
	}
	return "Link not found."
}
