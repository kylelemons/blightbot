package gonuts

import (
	"exp/html"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/kylelemons/blightbot/commander"
)

const MaxPublicResults = 5
const RefreshDocEvery = 6 * time.Hour

var DocSites = map[string]string{
	"release": "golang.org",
	"weekly":  "weekly.golang.org",
	//"tip":     "tip.golang.org",
}

var DocPages = map[string]string{
	"pkg":    "/pkg",
	"cmd":    "/cmd",
	"ego":    "/doc/effective_go.html",
	"faq":    "/doc/go_faq.html",
	"spec":   "/ref/spec",
	"go1":    "/doc/go1.html",
	"compat": "/doc/go1compat.html",
}

type Index struct {
	Pages map[string]map[string]*PageIndex // Pages[site][page]
}

type PageIndex struct {
	SectionURLs map[string][]string
}

func (p *PageIndex) ParseFrom(uri url.URL, root *html.Node) error {
	if p.SectionURLs == nil {
		p.SectionURLs = make(map[string][]string, 100)
	}

	var text func(*html.Node) string
	text = func(n *html.Node) string {
		pieces := []string{}
		if n.Type == html.TextNode {
			pieces = append(pieces, n.Data)
		}
		for _, child := range n.Child {
			t := strings.TrimSpace(text(child))
			if t != "" {
				pieces = append(pieces, t)
			}
		}
		return strings.Join(pieces, " ")
	}

	var index func(*html.Node)
	index = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if len(n.Data) == 2 && n.Data[0] == 'h' {
				sectionurl := uri
				sectionname := ""
				for _, attr := range n.Attr {
					if attr.Key == "id" {
						sectionurl.Fragment = attr.Val
					}
				}
				sectionname = strings.Replace(text(n), "\n", " ", -1)
				if sectionname == "" || sectionname == ".." || sectionurl.Fragment == "" {
					return
				}
				sectionname = strings.TrimSpace(sectionname)
				sectionname = strings.Title(sectionname)

				log.Printf("Found %q %q", sectionname, sectionurl)
				p.SectionURLs[sectionname] = append(p.SectionURLs[sectionname], sectionurl.String())
				return
			} else if n.Data == "a" {
				pkgpath := ""
				pkgname := ""
				for _, attr := range n.Attr {
					if attr.Key == "href" {
						pkgpath = attr.Val
					}
				}
				for _, child := range n.Child {
					if child.Type == html.TextNode {
						pkgname = child.Data
					}
				}
	
				// All packages have a name and a href
				if pkgname == "" || pkgpath == "" {
					return
				}
				// Package names can't have spaces
				if strings.IndexRune(pkgname, ' ') >= 0 {
					return
				}
				// Packages don't have absolute URIs
				if pkgpath[0] == '/' || strings.IndexRune(pkgpath, ':') >= 0 {
					return
				}
				// Package names are all lower-case
				if strings.ToLower(pkgname) != pkgname {
					return
				}
				// Package names are either the same as or the last entry of the path
				if pkgname != pkgpath && !strings.HasSuffix(pkgpath, "/"+pkgname) {
					return
				}

				pkgurl := uri
				pkgurl.Path = path.Join(pkgurl.Path, pkgpath)

				p.SectionURLs[pkgpath] = append(p.SectionURLs[pkgpath], pkgurl.String())
				return
			}
		}
		for _, c := range n.Child {
			index(c)
		}
	}
	index(root)
	return nil
}

func generate() {
	type data struct {
		site, page string
		parsed     *PageIndex
	}

	start := time.Now()
	index := new(Index)
	done := make(chan data, len(DocSites)*len(DocPages))

	index.Pages = map[string]map[string]*PageIndex{}
	for site, host := range DocSites {
		index.Pages[site] = map[string]*PageIndex{}
		for page, path := range DocPages {
			d := data{
				site: site,
				page: page,
			}
			uri := url.URL{
				Scheme: "http",
				Host:   host,
				Path:   path,
			}
			go func() {
				defer func() {
					done <- d
				}()
				resp, err := http.Get(uri.String())
				if err != nil {
					log.Printf("[GoDoc] %s:%s (%s) failed: %s", d.site, d.page, uri, err)
					return
				}
				defer resp.Body.Close()

				node, err := html.Parse(resp.Body)
				if err != nil {
					log.Printf("[GoDoc] %s:%s (%s) failed to parse: %s", d.site, d.page, uri, err)
					return
				}

				pageIndex := new(PageIndex)
				if err := pageIndex.ParseFrom(uri, node); err != nil {
					log.Printf("[GoDoc] %s:%s (%s) failed to index: %s", d.site, d.page, uri, err)
					return
				}

				if d.page == "pkg" || d.page == "cmd" {
					uris := make([]string, 0, len(pageIndex.SectionURLs))
					pkgs := make([]string, 0, len(pageIndex.SectionURLs))
					need := "/" + d.page + "/"

					for pkg := range pageIndex.SectionURLs {
						for _, uri := range pageIndex.SectionURLs[pkg] {
							if !strings.Contains(uri, need) {
								continue
							}
							uris = append(uris, uri)
							pkgs = append(pkgs, pkg)
						}
					}

					for i, uri := range uris {
						pkg := pkgs[i]
						log.Printf("[GoDoc] Pulling package %q at %q", pkg, uri)

						u, err := url.Parse(uri)
						if err != nil {
							log.Printf("[GoDoc] %s:%s:%s failed to parse URL %q: %s", d.site, d.page, pkg, uri, err)
							continue
						}

						resp, err := http.Get(uri)
						if err != nil {
							log.Printf("[GoDoc] bad package URL %q", uri)
							continue
						}
						defer resp.Body.Close()

						node, err := html.Parse(resp.Body)
						if err != nil {
							log.Printf("[GoDoc] %s:%s:%s (%s) failed to parse package: %s", d.site, d.page, pkg, uri, err)
							continue
						}

						if err := pageIndex.ParseFrom(*u, node); err != nil {
							log.Printf("[GoDoc] %s:%s:%s (%s) failed to index: %s", d.site, d.page, pkg, uri, err)
							continue
						}
					}
				}

				d.parsed = pageIndex
			}()
		}
	}
	for i := 0; i < cap(done); i++ {
		d := <-done
		log.Printf("[GoDoc] %s:%s complete", d.site, d.page)
		index.Pages[d.site][d.page] = d.parsed
	}
	godocIndex = index
	log.Printf("Generate took %s", time.Since(start))
}

var godocIndex *Index

func godoc(src *commander.Source, resp *commander.Response, cmd string, args []string) {
	// The index is copy-on-write
	index := godocIndex

	sites, pages := []string{"release"}, []string{cmd}

	switch cmd {
	case "doc":
		pages = make([]string, 0, len(DocPages))
		for page := range DocPages {
			pages = append(pages, page)
		}
	default:
		if _, ok := DocPages[cmd]; !ok {
			resp.Private()
			resp.Printf("Unrecognized godoc subcommand %q", cmd)
			return
		}
	}

	var start int
	var overriddenSites bool
	for _, arg := range args {
		if len(arg) == 0 || arg[0] != '-' {
			continue
		}
		for len(arg) > 1 && arg[0] == '-' {
			arg = arg[1:]
		}
		start++
		if _, ok := DocSites[arg]; ok {
			if overriddenSites {
				sites = append(sites, arg)
				continue
			}
			sites = []string{arg}
			overriddenSites = true
		} else {
			resp.Private()
			resp.Printf("Unrecognized godoc option: --%s", arg)
		}
	}

	search := strings.Join(args[start:], " ")

	if search == "" {
		resp.Private()
		resp.Printf("What do you want to search for?")
		return
	}

	if index == nil {
		resp.Private()
		resp.Printf("The index isn't ready yet.")
		return
	}

	prefix := ""

	exact := make([]string, 0, MaxPublicResults)
	found := make([]string, 0, MaxPublicResults)

	for _, site := range sites {
		if len(sites) > 1 {
			prefix = site + ": "
		}
		for _, page := range pages {
			search := search
			switch page {
			case "pkg", "cmd": // leave the case alone
			default:
				search = strings.Title(search)
			}
			pageIndex := index.Pages[site][page]
			if urls, ok := pageIndex.SectionURLs[search]; ok {
				for _, url := range urls {
					exact = append(exact, fmt.Sprintf("%s%s: %s - %s", prefix, page, search, url))
				}
			}
			if len(exact) > 0 {
				continue
			}
			for section, urls := range pageIndex.SectionURLs {
				if strings.Contains(section, search) {
					for _, url := range urls {
						found = append(found, fmt.Sprintf("%s%s: %s - %s", prefix, page, section, url))
					}
				}
			}
		}
	}
	switch {
	case len(exact) > 0:
		resp.Public()
		for _, e := range exact {
			resp.Printf(e)
		}
	case len(found) > MaxPublicResults:
		resp.Private()
		resp.Printf("Found %d results; only showing %d", len(found), MaxPublicResults)
		sort.Sort(DashSorter(found))
		found = found[:MaxPublicResults]
		fallthrough
	case len(found) > 0:
		resp.Public()
		for _, f := range found {
			resp.Printf(f)
		}
	default:
		resp.Private()
		resp.Printf("Nothing found for %q in %v x %v", search, sites, pages)
	}

}

type DashSorter []string

func (ds DashSorter) Len() int { return len(ds) }
func (ds DashSorter) Less(i, j int) bool {
	return strings.IndexRune(ds[i], '-') < strings.IndexRune(ds[j], '-')
}
func (ds DashSorter) Swap(i, j int) { ds[i], ds[j] = ds[j], ds[i] }

func dochelp(cmd, text, searchFor string) string {
	opts := ""
	for site := range DocSites {
		opts += "[--" + site + "] "
	}
	return fmt.Sprintf("%s\nUsage: %s %s<%s>", text, strings.ToUpper(cmd), opts, searchFor)
}

var (
	Pkg    = commander.Cmd("pkg", godoc).Help(dochelp("pkg", "Retrieve the URLs for go packages", "package"))
	Cmd    = commander.Cmd("cmd", godoc).Help(dochelp("cmd", "Retrieve the URLs for go commands", "command"))
	FAQ    = commander.Cmd("faq", godoc).Help(dochelp("faq", "Retrieve the URLs for FAQ sections", "search terms"))
	Go1    = commander.Cmd("go1", godoc).Help(dochelp("go1", "Retrieve the URLs for Go1 Release Notes sections", "search terms"))
	EGo    = commander.Cmd("ego", godoc).Help(dochelp("ego", "Retrieve the URLs for Effective Go sections", "search terms"))
	Doc    = commander.Cmd("doc", godoc).Help(dochelp("doc", "Search the (cached) online documents", "search terms"))
	Spec   = commander.Cmd("spec", godoc).Help(dochelp("spec", "Retrieve the URLs for Specification sections", "search terms"))
	Compat = commander.Cmd("compat", godoc).Help(dochelp("compat", "Retrieve the URLs for Go1 Compatibility Notes sections", "search terms"))
)

func StartPolling() {
	go func() {
		for {
			generate()
			time.Sleep(RefreshDocEvery)
		}
	}()
}
