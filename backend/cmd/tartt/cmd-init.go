// vim: sw=8

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

type configTemplateData struct {
	OriginDir     string
	StoreName     string
	Driver        string
	DriverOptions map[string]string
}

var configYml = template.Must(template.New("configYml").Parse(
	strings.TrimSpace(`
originDir: {{ .OriginDir }}
storesDir: "./stores"
stores:
  - name: {{ .StoreName }}
    driver: {{ .Driver }}
{{- if .DriverOptions }}
    {{ .Driver }}:
    {{- range $k, $v := .DriverOptions }}
      {{ $k }}: {{ $v }}
    {{- end -}}
{{- end }}
    levels:
      - { interval: "2 months", lifetime: "6 months" }
      - { interval: "5 days", lifetime: "40 days" }
      - { interval: "1 day", lifetime: "8 days" }
      - { interval: "1 hour", lifetime: "50 hours" }
      - { interval: "0", lifetime: "120 minutes" }
`) + "\n",
))

// Ignore:
//
//  - failed archives dirs `*.error/`.
//  - incomplete archives `*.inprogress/`.
//  - logs: They are also packed as metadata.
//  - tar incremental `.snar` state: It is tied to the local filesystem state.
//  - tar data: tar files are big.  Only the manifest is stored in Git.
//  - secrets: They should be stored separately for security.  It may also make
//    sense to regularly reencrypt them.  Both requirements conflict with
//    storing them permanently in the Git history.
//
var gitignore = strings.TrimSpace(`
*.error/
*.inprogress/
*.log
*.snar
*.tar
*.tar.*
secret.asc
secret
`) + "\n"

func cmdInit(args map[string]interface{}) {
	wd, err := os.Getwd()
	if err != nil {
		lg.Fatalw("Failed to determine working directory.")
	}
	if !isEmptyDir(".") {
		lg.Fatalw("Directory not empty.", "dir", wd)
	}

	origin := args["--origin"].(string)
	if !filepath.IsAbs(origin) {
		lg.Fatalw("--origin must be an absolute path.")
	}
	// Warn but do not fail if origin cannot be checked.  Accessing origin
	// may require privileges.  But being able to initialize the repo
	// without having the privileges to access origin may be useful.
	if inf, err := os.Stat(origin); err != nil {
		lg.Warnw("Could not confirm that --origin is a directory.")
	} else if !inf.IsDir() {
		lg.Fatalw("--origin is not a directory.")
	}

	storeName, ok := args["--store"].(string)
	if !ok {
		hn, err := os.Hostname()
		if err != nil {
			lg.Fatalw("Failed to determine hostname.", "err", err)
		}
		storeName = hn
	}

	for _, d := range []string{
		"stores",
		filepath.Join("stores", storeName),
	} {
		if err := os.Mkdir(d, 0777); err != nil {
			lg.Fatalw("Failed to create subdir.", "dir", d)
		}
	}

	driver := "local"
	var opts map[string]string
	if _, ok := args["--driver-localtape-tardir"].(string); ok {
		driver = "localtape"
		opts = driverOptionsLocaltape(args)
	}

	cfgFile := "tarttconfig.yml"
	fp, err := os.Create(cfgFile)
	if err != nil {
		lg.Fatalw("Failed to create config file.", "file", cfgFile)
	}
	if err := configYml.Execute(fp, configTemplateData{
		OriginDir:     jsonString(origin),
		StoreName:     jsonString(storeName),
		Driver:        driver,
		DriverOptions: opts,
	}); err != nil {
		lg.Fatalw("Failed to write config.", "err", err)
	}
	if err := fp.Close(); err != nil {
		lg.Fatalw("Failed to write config.", "err", err)
	}

	// Prepare repo for tracking it in Git.  See details at `gitignore`.
	err = ioutil.WriteFile(".gitignore", []byte(gitignore), 0666)
	if err != nil {
		lg.Fatalw("Failed to write `.gitignore`.", "err", err)
	}

	fmt.Printf("Initialized Tartt repo: %s\n", wd)
}

func driverOptionsLocaltape(args map[string]interface{}) map[string]string {
	tardir, ok := args["--driver-localtape-tardir"].(string)
	if !ok {
		lg.Fatalw("Missing --driver-localtape-tardir.")
	}
	if !filepath.IsAbs(tardir) {
		lg.Fatalw(
			"--driver-localtape-tardir must be an absolute path.",
		)
	}
	if !isDir(tardir) {
		lg.Fatalw("--driver-localtape-tardir is not a directory.")
	}

	return map[string]string{
		"tardir": jsonString(tardir),
	}
}

func jsonString(s string) string {
	buf, err := json.Marshal(s)
	if err != nil {
		panic("failed to JSON encode string")
	}
	return string(buf)
}

func isEmptyDir(p string) bool {
	fp, err := os.Open(p)
	if err != nil {
		return false
	}
	ls, err := fp.Readdirnames(1)
	_ = fp.Close()
	// EOF alone would be sufficient.  Be paranoid and double check.
	return len(ls) == 0 && err == io.EOF
}
