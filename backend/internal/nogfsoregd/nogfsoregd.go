/*

Package `nogfsoregd` implements modules that are combined in `cmd/nogfsoregd`.

`NewMainServer()`, `maind.Server`: GRPC service `nogfso.Main` to access the FSO
main root entity,

`NewRegistryServer()`, `registryd.Server`: GRPC service `nogfso.Registry` to
access the FSO registry,

`NewReposServer()`, `reposd.Server`: GRPC service `nogfso.Repos` to access the
FSO repos.

`NewStatdsServer()`, `statdsd.Server`: GRPC service `nogfso.Statds`;
`nogfsostad` host servers connect in permanent sessions and wait to execute
commands.  Also GRPC service `nogfso.Stat`; calls are forwarded to host server
sessions.

`ProcessRegistryInit()`, `registryinit.Processor`: Watches main journal and
initializes fsoregistry entities.

`ProcessRepoInit()`, `repoinit.Processor`: Watches registry journals and
initializes repo entities.

*/
package nogfsoregd
