# updater

Updater ...

Installation
------------

```bash
go get -u -d src.sourcegraph.com/apps/updater/...
```

Development
-----------

This project relies on `go generate` directives to process and statically embed assets. For development only, you'll need extra dependencies:

```bash
go get -u -d -tags=generate src.sourcegraph.com/apps/updater/...
go get -u -d -tags=js src.sourcegraph.com/apps/updater/...
```

Afterwards, you can build and run in development mode, where all assets are always read and processed from disk:

```bash
go build -tags=dev something/that/uses/updater
```

When you're done with development, you should run `go generate` and commit that:

```bash
go generate src.sourcegraph.com/apps/updater/...
```

License
-------

-	TODO.
