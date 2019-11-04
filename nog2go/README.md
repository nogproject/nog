# Nog to go

*Nog to go* is supposed to make nog usable in offline mode.
The current state allows you to cache and browse nog repos locally with very 
basic functionality:

```bash
mkdir nogRepos
cd nogRepos
./../nog2Go cache ulrikehomberg/example_photo-gallery_2015-07
./../nog2Go cache ulrikehomberg/example_photo-programs_2015-07
./../NogHTTPServer
```

Then open your favorite browser: `http://localhost:8000/`

See `NogHTTPServer --help` or `nog2go --help` to see how to set port, address 
and range of cached history.
