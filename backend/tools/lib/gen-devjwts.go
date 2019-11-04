// vim: sw=8

package main

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	docopt "github.com/docopt/docopt-go"
)

var (
	usage = qqBackticks(`Usage:
  gen-devjwts

Do not run directly, but only as as part of ''make devcerts''.
`)

	jwtdir = "/nog/jwt/tokens"

	issuer    = "nogapp"
	expiresIn = 63 * 24 * time.Hour // 2 months + eps.
	keyPath   = "/nog/ssl/certs/nogapp-iam/combined.pem"

	audienceRegd          = []string{"fso"}
	audienceStad          = []string{"fso"}
	audienceRstd          = []string{"fso"}
	audienceDomd          = []string{"fso"}
	audienceSchd          = []string{"fso"}
	audienceTard          = []string{"fso"}
	audienceSdwbakd3      = []string{"fso"}
	audienceSdwgctd       = []string{"fso"}
	audienceTchd3Registry = []string{"fso"}
	audienceTchd3Nogapp   = []string{"nogapp"}
	audienceAdmin         = []string{"fso", "nogapp"}

	version = "gen-devjwts-0.0.0"
)

type Scopes []map[string][]string

var scopeRegd = Scopes{
	map[string][]string{
		"aa": []string{"br"},        // bc/read
		"n":  []string{"allaggsig"}, // name
	},
	map[string][]string{
		"aa": []string{
			"frg",  // fso/read-registry
			"fcpr", // fso/exec-ping-registry
		},
		"n": []string{"exreg"}, // name
	},
	map[string][]string{
		"aa": []string{
			"frt",   // fso/read-root
			"fcsr",  // fso/exec-split-root
			"frr",   // fso/read-repo
			"fxfr",  // fso/exec-freeze-repo
			"fxufr", // fso/exec-unfreeze-repo
			"fxvr",  // fso/exec-archive-repo
			"fxuvr", // fso/exec-unarchive-repo
		},
		"p": []string{"/example/*"}, // path
	},
}

var scopeStad = Scopes{
	map[string][]string{
		"aa": []string{"bw"},  // bc/write
		"n":  []string{"all"}, // name
	},
	map[string][]string{
		"aa": []string{"br"},        // bc/read
		"n":  []string{"allaggsig"}, // name
	},
	map[string][]string{
		"aa": []string{"fs"},        // fso/session
		"n":  []string{"localhost"}, // name
	},
	map[string][]string{
		"aa": []string{
			"frg",  // fso/read-registry
			"fcpr", // fso/exec-ping-registry
		},
		"n": []string{"exreg"}, // name
	},
	map[string][]string{
		"aa": []string{
			"frt",   // fso/read-root
			"fcd",   // fso/exec-du
			"fxfr",  // fso/exec-freeze-repo
			"fxufr", // fso/exec-unfreeze-repo
			"fxvr",  // fso/exec-archive-repo
			"fxuvr", // fso/exec-unarchive-repo
		},
		"p": []string{"/example/*"}, // path
	},
	map[string][]string{
		"aa": []string{
			"fcr", // fso/confirm-repo
			"frr", // fso/read-repo
		},
		"p": []string{"/example*"}, // path
	},
}

var scopeRstd = Scopes{
	map[string][]string{
		"aa": []string{"br"},        // bc/read
		"n":  []string{"allaggsig"}, // name
	},
	map[string][]string{
		"aa": []string{
			"frg", // fso/read-registry
		},
		"n": []string{"exreg"}, // name
	},
	map[string][]string{
		"aa": []string{
			"frr",   // fso/read-repo
			"fxuvr", // fso/exec-unarchive-repo
		},
		"p": []string{"/example/*"}, // path
	},
}

var scopeDomd = Scopes{
	map[string][]string{
		"aa": []string{
			"xrd", // uxd/read-unix-domain
			"xwd", // uxd/write-unix-domain
		},
		"n": []string{"EXDOM"}, // name
	},
}

var scopeSchd = Scopes{
	map[string][]string{
		"aa": []string{"br"},  // bc/read
		"n":  []string{"all"}, // name
	},
	map[string][]string{
		"aa": []string{"frg"},   // fso/read-registry
		"n":  []string{"exreg"}, // name
	},
	map[string][]string{
		"aa": []string{"frr"},       // fso/read-repo
		"p":  []string{"/example*"}, // path
	},
}

var scopeTard = Scopes{
	map[string][]string{
		"aa": []string{"br"},  // bc/read
		"n":  []string{"all"}, // name
	},
	map[string][]string{
		"aa": []string{"frg"},   // fso/read-registry
		"n":  []string{"exreg"}, // name
	},
	map[string][]string{
		"aa": []string{
			"frr", // fso/read-repo
			"fia", // fso/init-repo-tartt
		},
		"p": []string{"/example*"}, // path
	},
}

var scopeSdwbakd3 = Scopes{
	map[string][]string{
		"aa": []string{"br"},  // bc/read
		"n":  []string{"all"}, // name
	},
	map[string][]string{
		"aa": []string{"frg"},   // fso/read-registry
		"n":  []string{"exreg"}, // name
	},
	map[string][]string{
		"aa": []string{
			"frr", // fso/read-repo
			"fib", // fso/init-repo-shadow-backup
		},
		"p": []string{"/example*"}, // path
	},
}

// `nogfsosdwgctd` does not watch the broadcast.  It uses only regular scans.
var scopeSdwgctd = Scopes{
	map[string][]string{
		"aa": []string{"frg"},   // fso/read-registry
		"n":  []string{"exreg"}, // name
	},
	map[string][]string{
		"aa": []string{"frr"},       // fso/read-repo
		"p":  []string{"/example*"}, // path
	},
}

var scopeTchd3Registry = Scopes{
	map[string][]string{
		"aa": []string{"frg"},   // fso/read-registry
		"n":  []string{"exreg"}, // name
	},
	map[string][]string{
		"aa": []string{"frr"},       // fso/read-repo
		"p":  []string{"/example*"}, // path
	},
}
var scopeTchd3Nogapp = Scopes{
	map[string][]string{
		"aa": []string{
			"a",   // api
			"ftu", // fso/issue-user-token
		},
		"p": []string{"/"},
	},
	map[string][]string{
		"aa": []string{"ffr"},       // fso/refresh-repo
		"p":  []string{"/example*"}, // path
	},
}

var scopeAdmin = Scopes{
	map[string][]string{
		"aa": []string{
			"a",  // api
			"b*", // broadcast wildcard `bc/*`
			"f*", // fso wildcard `fso/*`
			"x*", // Unix domain wildcard `uxd/*`
		},
		"n": []string{"*"},
		"p": []string{"/*"},
	},
}

func main() {
	const autoHelp = true
	const noOptionFirst = false
	_, err := docopt.Parse(usage, nil, autoHelp, version, noOptionFirst)
	must(err)

	must(os.MkdirAll(jwtdir, 0755))

	var f string
	var tok []byte

	f = filepath.Join(jwtdir, "nogfsoregd.jwt")
	tok = sysToken(
		"alovelace+nogfsoregd+dev",
		audienceRegd,
		[]string{"DNS:localhost"},
		scopeRegd,
	)
	must(ioutil.WriteFile(f, tok, 0644))
	fmt.Println(f)

	f = filepath.Join(jwtdir, "nogfsostad.jwt")
	tok = sysToken(
		"alovelace+nogfsostad+dev",
		audienceStad,
		[]string{"DNS:localhost"},
		scopeStad,
	)
	must(ioutil.WriteFile(f, tok, 0644))
	fmt.Println(f)

	f = filepath.Join(jwtdir, "nogfsorstd.jwt")
	tok = sysToken(
		"alovelace+nogfsorstd+dev",
		audienceRstd,
		[]string{"DNS:localhost"},
		scopeRstd,
	)
	must(ioutil.WriteFile(f, tok, 0644))
	fmt.Println(f)

	f = filepath.Join(jwtdir, "nogfsodomd.jwt")
	tok = sysToken(
		"alovelace+nogfsodomd+dev",
		audienceDomd,
		[]string{"DNS:localhost"},
		scopeDomd,
	)
	must(ioutil.WriteFile(f, tok, 0644))
	fmt.Println(f)

	f = filepath.Join(jwtdir, "nogfsoschd.jwt")
	tok = sysToken(
		"alovelace+nogfsoschd+dev",
		audienceSchd,
		nil,
		scopeSchd,
	)
	must(ioutil.WriteFile(f, tok, 0644))
	fmt.Println(f)

	f = filepath.Join(jwtdir, "nogfsotard.jwt")
	tok = sysToken(
		"alovelace+nogfsotard+dev",
		audienceTard,
		nil,
		scopeTard,
	)
	must(ioutil.WriteFile(f, tok, 0644))
	fmt.Println(f)

	f = filepath.Join(jwtdir, "nogfsosdwbakd3.jwt")
	tok = sysToken(
		"alovelace+nogfsosdwbakd3+dev",
		audienceSdwbakd3,
		nil,
		scopeSdwbakd3,
	)
	must(ioutil.WriteFile(f, tok, 0644))
	fmt.Println(f)

	f = filepath.Join(jwtdir, "nogfsosdwgctd.jwt")
	tok = sysToken(
		"alovelace+nogfsosdwgctd+dev",
		audienceSdwgctd,
		nil,
		scopeSdwgctd,
	)
	must(ioutil.WriteFile(f, tok, 0644))
	fmt.Println(f)

	f = filepath.Join(jwtdir, "nogfsotchd3-registry.jwt")
	tok = sysToken(
		"alovelace+nogfsotchd3+dev",
		audienceTchd3Registry,
		nil,
		scopeTchd3Registry,
	)
	must(ioutil.WriteFile(f, tok, 0644))
	fmt.Println(f)

	f = filepath.Join(jwtdir, "nogfsotchd3-nogapp.jwt")
	tok = sysToken(
		// Issue the token for user `sprohaska` so that `nog-app` uses
		// the fso permission rule `AllowInsecureEverything`.
		"sprohaska+nogfsotchd3+dev",
		audienceTchd3Nogapp,
		nil,
		scopeTchd3Nogapp,
	)
	must(ioutil.WriteFile(f, tok, 0644))
	fmt.Println(f)

	f = filepath.Join(jwtdir, "admin.jwt")
	tok = sysToken(
		"sprohaska+admin+dev",
		audienceAdmin,
		nil,
		scopeAdmin,
	)
	must(ioutil.WriteFile(f, tok, 0644))
	fmt.Println(f)
}

func sysToken(
	name string, audience, san []string, sc Scopes,
) []byte {
	key, x5c := readKey()
	tok := jwt.New(jwt.SigningMethodRS256)
	tok.Header["x5c"] = x5c
	claims := tok.Claims.(jwt.MapClaims)
	claims["jti"] = "devjwt"
	claims["iat"] = time.Now().Unix()
	claims["exp"] = time.Now().Add(expiresIn).Unix()
	claims["iss"] = issuer
	claims["aud"] = audience
	claims["sub"] = fmt.Sprintf("sys:%s", name)
	if san != nil {
		claims["san"] = san
	}
	if sc != nil {
		claims["sc"] = sc
	}
	tokStr, err := tok.SignedString(key)
	must(err)
	return []byte(tokStr)
}

func readKey() (*rsa.PrivateKey, string) {
	pemBytes, err := ioutil.ReadFile(keyPath)
	must(err)

	certBlock, pemBytes := pem.Decode(pemBytes)
	if certBlock == nil {
		panic("Invalid PEM.")
	}
	x5c := base64.StdEncoding.EncodeToString(certBlock.Bytes)

	keyBlock, pemBytes := pem.Decode(pemBytes)
	if keyBlock == nil {
		panic("Invalid PEM.")
	}
	key, err := jwt.ParseRSAPrivateKeyFromPEM(pem.EncodeToMemory(keyBlock))
	must(err)

	return key, x5c
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

// `qqBackticks()` translates double single quote to backtick.
func qqBackticks(s string) string {
	return strings.Replace(s, "''", "`", -1)
}
