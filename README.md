## Why?
Sometimes you need to test how your application handles slow/buggy
external APIs. If you can easily configure API server domain name, then goslow will help you.

## Quick start
Let's say you're developing an application against the Facebook graph API and
you want to see what happens when the endpoint *graph.facebook.com/me* starts to respond in 10 seconds.

Just configure your app to make requests to *10.goslow.link* instead of *graph.facebook.com*
and you're set:

```shell
time curl 10.goslow.link/me
{"goslow": "response"}
10.023 total
```

Well, almost set, because you've got a canned response **{"goslow": "response"}**.
You probably wanted to get the usual graph API response: **{"name": "zuck", "gender": "male"}**.  
No worries, we'll get to that later.

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
{"goslow": "response"}
99.104 total
```

Need to simulate a 500 seconds delay? Use *500.goslow.link*, right?

Nope. Domains *100.goslow.link*, *101.goslow.link*, ..., *599.goslow.link* respond with
HTTP status code 100, 101, ..., 599 without any delay:

```shell
time curl 500.goslow.link/me
{"goslow": "response"}%
0.052 total
```

*301.goslow.link* and *302.goslow.link* redirect to *0.goslow.link*


## Not-so-quick start
> No worries, we'll get to that later.

Remember that bit? Well, it's later time!

Let's return to the Facebook graph API example.
Let's say you're using the endpoint *graph.facebook.com/me* and you want to:
1. Slow it down by 5 seconds
2. Get **{"name": "zuck", "gender": "male"}** in response.

Just make a POST request to *create.goslow.link/me?delay=5* and you're set.
```shell
curl -d '{"name": "zuck", "gender": "male"}' 'create.goslow.link/me?delay=5'
Hooray!
Endpoint http://5wx55yijr.goslow.link/me responds to any HTTP method with 5s delay.
Response is: {"name": "zuck", "gender": "male"}

Your personal goslow domain is 5wx55yijr.goslow.link
...
```

Now, what's the deal with the "*your personal goslow domain is 5wx55yijr.goslow.link*"? Well, now you the domain *5wx55yijr.goslow.link* is all yours and you can add different endpoints to it.

Quick aside:
when you do a POST request to *create.goslow.link* your personal goslow domain will be a little different
from the *5wx55yijr.goslow.link*. Domain names are randomly generated. For the sake of example let's pretent that the randomly
generated domain name was *5wx55yijr.goslow.link*.
End of quick aside.

Now you can send requests to your domain:
```shell
time curl 5wx55yijr.goslow.link/me
{"name": "zuck", "gender": "male"}
5.382 total
```

You can add new endpoints by POSTing to *admin-5wx55yijr.goslow.link*  
Let's make the endpoint *5wx55yijr.goslow.link/another/* to respond to POST requests with **{"another": "response"}**
and 3.4 seconds delay:
```shell
curl -d '{"another": "response"}' 'admin-5wx55yijr.goslow.link/another/?delay=3.4'
Hooray!
Endpoint http://5wx55yijr.goslow.link/another/ responds to POST with 3.4s delay.
Response is: {"another": "response"}
```

Now you have two urls sending different responses with different delays.
```shell
time curl -d 'any payload' 5wx55yijr.goslow.link/another/
'{"another": "response"}'
3.482 total
```

```shell
time curl 5wx55yijr.goslow.link/me
{"name": "zuck", "gender": "male"}
5.028 total
```

Sky is the limit.

## Slow start
If you think that having unprotected-by-passwords third-party-domains storing-your-data is not a million dollar idea, then you're absolutely right.

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
since nobody wants to deal with the dynamically generated subdomain names on a localhost.
You can configure goslow with the POST requests to /goslow/ (this special endpoint can be changed via -admin-url-path-prefix option)
```shell
curl -d '{"local": "response"}' 'localhost:5103/goslow/feed?delay=4.3'
Hooray!
Endpoint http://localhost:5103/feed responds to any HTTP method with 4.3s delay.
Response is: {"local": "response"}
```


By default goslow server stores all data in memory. This means that any
configuration change you make will be lost after restart.
If you want to use a persistent storage, then you need to specify *--driver* and *--data-source* options.

Goslow supports sqlite3:
```shell
bin/goslow --driver sqlite3 --data-source /path/to/sqlite3/db/file
```

and postgres:
```shell
bin/goslow --driver postgres --data-source postgres://user@host/dbname
# data source prefix 'postgres://' is required
```

## Get in touch
Got a question or a suggestion?
I'd love to hear from you: [codumentary.com@gmail.com](mailto:codumentary.com@gmail.com)


## Contributing
Contributing to goslow is easy.  
First, you need to sign a contributor agreement.  
Second, your boss needs to sign a waiver that she's okay with you
contributing to goslow.

Just kidding.

Create pull requests, open issues, send emails with patches/tarballs/links-to-pastebin
to [codumentary.com@gmail.com](mailto:codumentary.com@gmail.com). Whatever makes you happy.
Any form of contribution is welcome.


## License
MIT
