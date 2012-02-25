package gonuts

import (
	"path"
	"net/http"
	"net/url"
	"time"

	"github.com/kylelemons/blightbot/commander"
)

var ThirdPartyHost = "gopkgdoc.appspot.com"
var ThirdPartyPath = "/pkg/"
var ThirdPartyPackages = "http://gopkgdoc.appspot.com/packages"

/* TODO cache?
var tpCacheLock sync.Mutex
var tpCache map[string]string
*/

func tpdoc(src *commander.Source, resp *commander.Response, cmd string, args []string) {
	if len(args) == 0 {
		resp.Private()
		resp.Printf("What package do you want?")
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
		Scheme: "http",
		Host:   ThirdPartyHost,
		Path:   path.Join(ThirdPartyPath, pkg),
	}

	r, err := http.Head(uri.String())
	if err != nil || r.StatusCode != http.StatusOK {
		resp.Public()
		resp.Printf("Hmm, I can't find %q.  You can look for it on %s .", pkg, ThirdPartyPackages)
		return
	}
	resp.Public()
	resp.Printf("3pkg: %s", uri.String())
}

var TPDoc = commander.Cmd("3pkg", tpdoc).Help(`Retrieve the URL for a third-party package
Usage: 3PKG <pkgname>

Thanks to Gary Burd for his awesome gopkgdoc site!
http://gopkgdoc.appspot.com/`)

func init() {
	go func() {
		for {
			generate()
			time.Sleep(1 * time.Hour)
		}
	}()
}