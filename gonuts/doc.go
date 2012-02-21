package gonuts

import (
	"exp/html"
	"log"
	"path"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kylelemons/blightbot/commander"
)

const MaxPublicResults = 5

var DocSites = map[string]string{
	"release": "golang.org",
	"weekly":  "weekly.golang.org",
	"tip":     "tip.golang.org",
}

var DocPages = map[string]string{
	"pkg":  "/pkg",
	"ego":  "/doc/effective_go.html",
	"faq":  "/doc/go_faq.html",
	"spec": "/doc/go_spec.html",
	"go1":  "/doc/go1.html",
}

type Index struct {
	Pages map[string]map[string]*PageIndex // Pages[site][page]
}

type PageIndex struct {
	SectionURLs map[string]string
}

func (p *PageIndex) ParseFrom(uri url.URL, root *html.Node) error {
	p.SectionURLs = make(map[string]string, 100)

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
				/*
					for _, child := range n.Child {
						if child.Type == html.TextNode {
							sectionname += " " + child.Data
						}
					}
				*/
				if sectionname == "" || sectionurl.Fragment == "" {
					return
				}
				sectionname = strings.TrimSpace(sectionname)
				sectionname = strings.Title(sectionname)
				log.Printf("[GoDoc] %s: found section %q (at %s)", &uri, sectionname, &sectionurl)
				p.SectionURLs[sectionname] = sectionurl.String()
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
				log.Printf("[GoDoc] %s: found pkg %q (at %s)", &uri, pkgname, &pkgurl)
				p.SectionURLs[pkgpath] = pkgurl.String()
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

	sites, pages := []string{"weekly"}, []string{cmd}

	switch cmd {
	case "pkg":
	case "faq":
	case "go1":
	case "spec":
	case "ego":
	case "doc":
		pages = make([]string, 0, len(DocPages))
		for page := range DocPages {
			pages = append(pages, page)
		}
	default:
		resp.Private()
		resp.Printf("Unrecognized godoc subcommand %q", cmd)
		return
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

	if index == nil {
		resp.Private()
		resp.Printf("The index isn't ready yet.")
		return
	}

	resp.Public()
	prefix := ""
	found := 0
	for _, site := range sites {
		if len(sites) > 1 {
			prefix = site + ": "
		}
		for _, page := range pages {
			search := search
			if page != "pkg" {
				search = strings.Title(search)
			}
			pageIndex := index.Pages[site][page]
			if url, ok := pageIndex.SectionURLs[search]; ok {
				found++
				resp.Printf("%s%s: %s - %s", prefix, page, search, url)
				continue
			}
			for section, url := range pageIndex.SectionURLs {
				if strings.Contains(section, search) {
					found++
					if found > MaxPublicResults {
						resp.Private()
						resp.Printf("Too many results to display them all.")
						return
					}
					resp.Printf("%s%s: %s - %s", prefix, page, section, url)
				}
			}
		}
	}
	if found == 0 {
		resp.Private()
		resp.Printf("Nothing found for %q in %v x %v", search, sites, pages)
	}

}

var Pkg = commander.Cmd("pkg", godoc).Help(`Retrieve the URLs for go packages
Usage: PKG [--release] [--weekly] [--tip] <pkgname>`)
var FAQ = commander.Cmd("faq", godoc).Help(`Retrieve the URLs for FAQ sections
Usage: FAQ [--release] [--weekly] [--tip] <search terms>`)
var Go1 = commander.Cmd("go1", godoc).Help(`Retrieve the URLs for Go1 sections
Usage: GO1 [--release] [--weekly] [--tip] <search terms>`)
var Spec = commander.Cmd("spec", godoc).Help(`Retrieve the URLs for Specification sections
Usage: SPEC [--release] [--weekly] [--tip] <search terms>`)
var EGo = commander.Cmd("ego", godoc).Help(`Retrieve the URLs for Effective Go sections
Usage: EGO [--release] [--weekly] [--tip] <search terms>`)
var Doc = commander.Cmd("doc", godoc).Help(`Search the (cached) online documents
Usage: DOC [--release] [--weekly] [--tip] <search terms>`)

func init() {
	go func() {
		for {
			generate()
			time.Sleep(1 * time.Hour)
		}
	}()
}
