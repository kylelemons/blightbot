package gonuts

import (
	"net/http"
	"net/url"

	"github.com/kylelemons/blightbot/commander"
)

var ThirdPartyHost = "godoc.org"
var ThirdPartyIndex = "http://godoc.org/-/index"

/* TODO cache?
var tpCacheLock sync.Mutex
var tpCache map[string]string
*/

func tpdoc(src *commander.Source, resp *commander.Response, cmd string, args []string) {
	if len(args) == 0 {
		resp.Public()
		u := &url.URL{
			Scheme: "http",
			Host:   ThirdPartyHost,
			Path:   "/",
		}
		resp.Printf("%s: %s", cmd, u)
		return
	}

	pkg := args[0]
	for _, ch := range pkg {
		switch ch {
		case 'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z':
		case 'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z':
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		case '-', '.', '_', '/':
		default:
			resp.Private()
			resp.Printf("Hmm, that doesn't look like a package name...")
			return
		}
	}

	uri := url.URL{
		Scheme:   "http",
		Host:     ThirdPartyHost,
		Path:     "/",
		RawQuery: url.Values{"q": {pkg}}.Encode(),
	}

	r, err := http.Head(uri.String())

	// If the query did not redirect, then GoPkgDoc could not find the package.
	if err != nil || r.StatusCode != http.StatusOK || r.Request.URL.Path == "/" {
		resp.Public()
		resp.Printf("Hmm, I can't find %q.  You can look for it on %s", pkg, ThirdPartyIndex)
		return
	}
	resp.Public()
	resp.Printf("3pkg: %s", r.Request.URL.String())
}

var TPDoc = commander.Cmd("3pkg", tpdoc).Help(`Retrieve the URL for a third-party package
Usage: 3PKG <pkgname>

Thanks to Gary Burd for his awesome gopkgdoc site!
http://gopkgdoc.appspot.com/`)
