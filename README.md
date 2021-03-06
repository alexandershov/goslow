[![Build Status](https://travis-ci.org/alexandershov/goslow.svg?branch=master)](https://travis-ci.org/alexandershov/goslow)
## Why?
Imagine Facebook/Twitter/Whatever API going down and responding really slow.  
Imagine your application using this API and slowing down because of that.  
Need an always-slow API to reproduce this situation reliably?  
Goslow is that API.

## Quick start
Let's say you're developing an application against the Facebook graph API and
you want to know how your app behaves when the endpoint *graph.facebook.com/me* responds in 10 seconds.

Just configure your app to make requests to *10.goslow.link* instead of the *graph.facebook.com*
and you're set. *10.goslow.link* responds with 10 seconds delay:

```shell
time curl 10.goslow.link/me
{"goslow": "response"}
10.423 total
```

Well, you're almost set, because you received a canned response **{"goslow": "response"}**.
You probably wanted to get the usual graph API response: **{"name": "zuck", "gender": "male"}**.  
No worries, we'll get to that later.

By the way, different endpoints and POST requests also work:
```shell
time curl -X POST -d 'your payload' '10.goslow.link/me/feed?message="test"'
{"goslow": "response"}
10.223 total
```

Need to fake a 6 seconds delay? Just use *6.goslow.link* instead of *10.goslow.link*


```shell
time curl 6.goslow.link/any-endpoint-works
{"goslow": "response"}
6.178 total
```

Need to simulate some serious delay? Use *199.goslow.link*:
```shell
time curl 199.goslow.link/me
{"goslow": "response"}
199.204 total
```

Need to fake a 500 seconds delay? Use *500.goslow.link*, right?

Nope! Domains *200.goslow.link*, *201.goslow.link*, ..., *599.goslow.link* respond with
HTTP status code 200, 201, ..., 599 without any delay:

```shell
time curl -w "%{http_code}" 500.goslow.link/me
{"goslow": "response"}500
0.152 total
```

*301.goslow.link* and *302.goslow.link* redirect to *0.goslow.link*:

```shell
time curl -w "%{redirect_url}" 302.goslow.link/me
{"goslow": "response"}HTTP://0.goslow.link
0.107 total
```

## Not-so-quick start
> No worries, we'll get to that later.

Remember that bit? It's later time!

Back to the Facebook graph API example.
You're using the endpoint *graph.facebook.com/me* and you want to:

1. Slow it down by 5 seconds.
2. Get **{"name": "zuck", "gender": "male"}** in response.

Just make a POST request to *create.goslow.link/me?delay=5* with the payload **{"name": "zuck", "gender": "male"}** and you're set. cURL is a great way to do it:
```shell
curl -d '{"name": "zuck", "gender": "male"}' 'create.goslow.link/me?delay=5'
Hooray!
Endpoint http://5wx55yijr.goslow.link/me responds to any HTTP method with 5s delay.
Response is: {"name": "zuck", "gender": "male"}

Your personal goslow domain is 5wx55yijr.goslow.link
...
```
Awesome, you're *really* set this time:

```shell
time curl 5wx55yijr.goslow.link/me
{"name": "zuck", "gender": "male"}
5.382 total
```

Now, what's the deal with the "*your personal goslow domain is 5wx55yijr.goslow.link*"? Well, the domain *5wx55yijr.goslow.link* is all yours and you can add your custom endpoints to it.

Quick aside:
when *you* POST to *create.goslow.link* your personal goslow domain will be a little different
from the *5wx55yijr.goslow.link*. Domain names are randomly generated. For the sake of example let's pretend that the randomly
generated domain name was *5wx55yijr.goslow.link*.
End of quick aside.


You can add new endpoints to your personal domain by POSTing to *admin-5wx55yijr.goslow.link*

Simple rule. Want an endpoint 5wx55yijr.goslow.link/*your-path* to respond with **your-response**? Post **your-response** to admin-5wx55yijr.goslow.link/*your-path*.

Let's make the endpoint *5wx55yijr.goslow.link/another/* to respond to POST requests with **{"another": "response"}**
and 3.4 seconds delay:
```shell
curl -d '{"another": "response"}' 'admin-5wx55yijr.goslow.link/another/?delay=3.4&method=POST'
Hooray!
Endpoint http://5wx55yijr.goslow.link/another/ responds to POST with 3.4s delay.
Response is: {"another": "response"}
```

Now you have two endpoints sending different responses with different delays.

```shell
time curl 5wx55yijr.goslow.link/me
{"name": "zuck", "gender": "male"}
5.028 total
```

```shell
time curl -d 'any payload' 5wx55yijr.goslow.link/another/
'{"another": "response"}'
3.482 total
```

The sky's the limit.

Worried whether slow javascript CDN will bring down your app? Goslow've got you covered:
```shell
curl ajax.googleapis.com/ajax/libs/jquery/2.1.1/jquery.min.js | curl -d @- "admin-5wx55yijr.goslow.link/ajax/libs/jquery/2.1.1/jquery.min.js?delay=20"

Hooray!
Endpoint http://5wx55yijr.goslow.link/ajax/libs/jquery/2.1.1/jquery.min.js responds to any HTTP method with 20s delay.
Response is: /*! jQuery v2.1.1 | (c) 2005, 2014 jQuery Foundation, Inc. | jquery.org/licen...
```

## Slow start
If you think that storing your data on unprotected-by-passwords-third-party-domains is not a great idea, then you're absolutely right.

You can install goslow on your own servers.

### Installation
[Download](https://github.com/alexandershov/goslow/releases) a precompiled binary for your operating system.

If you're feeling adventurous, you can [build goslow from source.](https://github.com/alexandershov/goslow/blob/master/Build.md)

### Usage

Start the server:
```shell
./goslow_darwin_amd64
# or "goslow_windows_amd64.exe" if you're on Windows
# or "goslow_linux_amd64" if you're on Linux
# or "goslow" if you compiled it by yourself

# listening on localhost:5103
```

By default goslow runs in a single domain mode
because nobody wants to deal with the dynamically generated subdomain names on a localhost.

You can configure goslow with POST requests to the endpoint /goslow.

Simple rule for a local instance of goslow. Want an endpoint localhost:5103/*your-path* to respond with **your-response**? Post **your-response** to localhost:5103/goslow/*your-path*.

Let's add the endpoint */feed*:
```shell
curl -d '{"local": "response"}' 'localhost:5103/goslow/feed?delay=4.3'
Hooray!
Endpoint http://localhost:5103/feed responds to any HTTP method with 4.3s delay.
Response is: {"local": "response"}
```


By default goslow stores endpoint data in memory. This means that any endpoint you add will be lost after restart.
If you want to use a persistent storage, then you need to specify *--db* and *--data-source* options.

Goslow supports sqlite3:
```shell
./goslow --db sqlite3 --data-source /path/to/sqlite3/db/file
```

Actually, sqlite3 is the default db, so this'll do:
```shell
./goslow --data-source /path/to/sqlite3/db/file
```

You can also use postgres:
```shell
./goslow --db postgres --data-source postgres://user@host/dbname
# prefix 'postgres://' is required
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
[MIT](https://github.com/alexandershov/goslow/blob/master/LICENSE)
