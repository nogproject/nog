/*

Package `nogfsostad` implements modules that are combined in `cmd/nogfsostad`.

Package `nogfsostad/observer4`: Watches the FSO registry and initializes repos:
FSO shadow repos and GitLab projects.

`Processor`: Coordinates repo initialization, stat, and sha.

`RepoInitializer`: Initializes repos: FSO shadow repos and GitLab projects.

`Session`: Permanent connection to `nogfsoregd` to receive commands.

Package `gits`: Init GitLab projects.

Package `shadows`: FSO shadow repos.

`NewStatServer()`, Package `statd`: GRPC service `nogfso.Stat`.

*/
package nogfsostad

type Logger interface {
	Infow(msg string, kv ...interface{})
	Warnw(msg string, kv ...interface{})
	Errorw(msg string, kv ...interface{})
}
