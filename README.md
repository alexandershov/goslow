## Why?
There comes a time in life when you need to test how your application handles slow/buggy
external API responses. As long as you can easily configure API server domain name, goslow'll help you.

## Quick start
Let's say you're developing an application against the Facebook graph API and
you want to see what happens when endpoint *graph.facebook.com/me* starts to respond in 10 seconds.

Just configure your app to make requests to *10.goslow.link* instead of *graph.facebook.com*
and you're set:

```shell
time curl 10.goslow.link/me
{"goslow": "response"}
10.023 total
```

Well, almost set, because you got a JSON response that has no relation to graph API whatsoever.
We'll get to that later.

By the way, different endpoints and POST requests also work:
```shell
time curl -X POST -d 'your payload' '10.goslow.link/me/feed?message="test"'
{"goslow": "response"}
10.123 total
```

Need to simulate a 6 seconds delay? Just use *6.goslow.link* instead of *10.goslow.link*


```shell
time curl 6.goslow.link/me
{"goslow": "response"}
6.128 total
```

Need to simulate some serious delay? Use *99.goslow.link*:
```shell
time curl 99.goslow.link/me
{"goslow": "link"}
99.104 total
```

Need to simulate a 500 seconds delay? Use *500.goslow.link*, right?

No. Domains *100.goslow.link*, *101.goslow.link*, ..., *599.goslow.link* respond with
HTTP status code 100, 101, ..., 599 without any delay:

```shell
time curl 500.goslow.link/me
Internal Server Error
0.052 total
```

*301.goslow.link* and *302.goslow.link* redirect to *0.goslow.link*


## Not-so-quick start
Let's return to the Facebook graph API example.
Let's say you're using the endpoint *graph.facebook.com/me* and you want to:
1. Slow it down by 5 seconds
2. Get {"id": 4, "username": "Zuck"} in response.

Just make a POST request to *create.goslow.link/me?delay=5* and you're set.
```shell
curl -d '{"id": 4, "username": "Zuck"}' 'create.goslow.link/me?delay=5'
Your goslow domain is: dk8kjs.goslow.link
...
```

Now, what's the deal with the "*your goslow domain dk8kjs.goslow.link*"?

In the real world your goslow domain will be different
from the *dk8kjs.goslow.link*. (names are randomly generated) For the sake of example let's assume that randomly
generated domain name is *dk8kjs.goslow.link*

Now you can send requests to your domain:
```shell
time curl dk8kjs.goslow.link/users
'{"my": "response"}'
5.382 total
```

And configure it with POST requests to *admin-dk8kjs.goslow.link*
Let's make endpoint *dk8kjs.goslow.link/another/* to respond with **{"another": "response"}**
and 3 seconds delay:
```shell
curl -d '{"another": "response"}' 'admin-dk8kjs.goslow.link/another/?delay=3'
dk8kjs.goslow.link/another/ will now respond with 3 seconds delay.
Response is '{"another": "response"}'
```

Now you have two urls responding with different JSON and delay.
```shell
time curl dk8kjs.goslow.link/another/
'{"another": "response"}'
3.182 total
```

```shell
time curl dk8kjs.goslow.link/users
'{"my": "response"}'
5.028 total
```

## Slow start
If you think that relying on unprotected-by-passwords third-party-domains is a
bad idea, then you're absolutely right.

You can install goslow locally. You'll need the [golang](https://golang.org/) compiler to build it.

```shell
# install dependencies
go get github.com/alexandershov/goslow \
github.com/lib/pq                      \
github.com/mattn/go-sqlite3            \
github.com/speps/go-hashids

# build
go install github.com/alexandershov/goslow

# run
bin/goslow
listening on :5103
```

Local install of goslow runs in a single domain mode by default
since nobody wants to deal with dynamically generated subdomain names on a local machine.
You can configure goslow with the POST requests to /goslow/.
```shell
curl -d '{"local": "response"}' 'localhost:5103/goslow/feed?delay=4.3'
/feed is now responding with 4.3 seconds delay.
Response is {"local": "response"}
```

You can also proxy goslow requests directly to your API with extra delay:
```shell
curl -d 'http://your.api' 'localhost:5103/goslow/?proxy&delay=10'
```


```shell
time curl localhost:5103/some/url
# proxies to http://your.api/some/url
10.123 total
```

By default goslow stores data in memory. This means that any
configuration change you make will be lost after restart.
You need to specify *--driver* and *--data-source* options to use a persistend storage.

Goslow supports sqlite3:
```shell
sqlite3 /path/to/sqlite3/db/file < goslow/sql/schema.sql
bin/goslow --driver sqlite3 --data-source /path/to/sqlite3/db/file
```

and postgres:
```shell
psql -U user dbname < goslow/sql/schema.sql
bin/goslow --driver postgres --data-source postgres://user@host/dbname
# data source prefix 'postgres://' is required
```

## Contributing
Contributing to goslow is easy.
First, we need you to sign a contributor's agreement.
Second, we need your boss to sign a waiver that she's okay with you
contributing to goslow.

Just kidding. Open pull requests, send emails with patches/tarballs/links-to-pastebin
to [codumentary.com@gmail.com](mailto:codumentary.com@gmail.com) Whatever makes you happy.

## License
MIT
