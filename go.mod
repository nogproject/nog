module github.com/nogproject/nog

// To create Git-commit pseudo-versions:
//
// ```
// TZ=UTC git show -s --abbrev=12 --date=format-local:%Y%m%d%H%M%S --pretty=%ad-%h
// ```

require (
	cloud.google.com/go v0.41.0 // indirect
	github.com/DataDog/zstd v1.4.0
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/docopt/docopt-go v0.0.0-20180111231733-ee0de3bc6815
	github.com/ftrvxmtrx/fd v0.0.0-20150925145434-c6d800382fff
	github.com/golang/protobuf v1.3.1
	github.com/google/uuid v1.1.1
	github.com/hashicorp/hcl v1.0.0
	github.com/juju/ratelimit v1.0.1
	github.com/libgit2/git2go v0.0.0-20180326105853-1381380f3450
	github.com/oklog/ulid v1.3.1
	github.com/paulbellamy/ratecounter v0.2.0
	github.com/pkg/errors v0.8.1 // indirect
	github.com/stretchr/objx v0.2.0 // indirect
	github.com/stretchr/testify v1.3.0
	github.com/xanzy/go-gitlab v0.18.0
	go.uber.org/atomic v1.4.0 // indirect
	go.uber.org/multierr v1.1.0 // indirect
	go.uber.org/zap v1.10.0
	golang.org/x/crypto v0.0.0-20190701094942-4def268fd1a4 // indirect
	golang.org/x/exp v0.0.0-20190627132806-fd42eb6b336f // indirect
	golang.org/x/image v0.0.0-20190703141733-d6a02ce849c9 // indirect
	golang.org/x/mobile v0.0.0-20190607214518-6fa95d984e88 // indirect
	golang.org/x/net v0.0.0-20190628185345-da137c7871d7
	golang.org/x/sync v0.0.0-20190423024810-112230192c58
	golang.org/x/sys v0.0.0-20190626221950-04f50cda93cb // indirect
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4
	golang.org/x/tools v0.0.0-20190703212419-2214986f1668 // indirect
	google.golang.org/genproto v0.0.0-20190701230453-710ae3a149df // indirect
	google.golang.org/grpc v1.22.0
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 // indirect
	gopkg.in/mgo.v2 v2.0.0-20180705113604-9856a29383ce
	gopkg.in/yaml.v2 v2.2.2
)

replace github.com/libgit2/git2go => /go/src/github.com/libgit2/git2go
