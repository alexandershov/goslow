## Install from source
Go 1.3+ is required.

Install goslow and its dependencies:
```shell
go get github.com/alexandershov/goslow     \
       github.com/lib/pq                   \
       github.com/mattn/go-sqlite3         \
       github.com/alexandershov/go-hashids
```

Build:
```shell
go install github.com/alexandershov/goslow
```

Run:
```shell
bin/goslow
# listening on :5103
```
