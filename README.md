## Why?
Sometimes you need to test how does your application handle slow/buggy
external API responses. Goslow can help you with that (as long as you can easily
  configure API server domain name).

## Quick start
Let's say you're developing an application against the [Facebook graph API](https://developers.facebook.com/docs/graph-api/quickstart/v2.2) and
you want to see what happens when endpoint *graph.facebook.com/me* starts to respond in 10 seconds.

Just configure your app to make requests to *10.goslow.link* instead of *graph.facebook.com*
and you're set:

```shell
time curl 10.goslow.link/me
{"goslow": "link"}
10.023 total
```

Now, obviously, you got a canned JSON response that has no relation to graph API whatsoever.
We'll get to that later.

By the way different endpoints and POST requests also work:
```shell
echo "your payload"| time curl -d @- 10.goslow.link/me/feed?message="test"
{"goslow": "link"}
10.123 total
```

Need to simulate 6 seconds delay? Just replace calls to *10.goslow.link* with
*6.goslow.link*

```shell
time curl 6.goslow.link/me
{"goslow": "link"}
6.128 total
```

Need to simulate a long delay? Use *99.goslow.link*:
```shell
time curl 99.goslow.link/me
{"goslow": "link"}
99.104 total
```

Need to simulate a 500 seconds delay? We should use *500.goslow.link*, right?

No. Domains *100.goslow.link*, *101.goslow.link*, ..., *599.goslow.link* respond with
HTTP status codes 100, 101, ..., 599 without any delay:

```shell
time curl 500.goslow.link/me
Internal Server Error
0.052 total
```


## Not-so-quick start
If you want to get real (not canned) JSON response things'll be a little ugly.

First you need to, ahem, register. Registration is just a POST request
to **new.goslow.link**

```shell
echo '{"my": "response"}' | curl -d @- new.goslow.link/users?delay=5
Your goslow domain is: dk8kjs.goslow.link
...
```

When you do the POST request for real, you'll get a domain different
from dk8kjs.goslow.link. For the sake of example let's assume that your
personal domain is dk8kjs.goslow.link.

Now you can send requests to your domain:
```shell
time curl dk8kjs.goslow.link/users
'{"my": "response"}'
5.382 total
```

And configure it with POST requests to **admin-dk8kjs.goslow.link**:
```shell
echo '{"another": "response"}' | curl -d @- admin-dk8kjs.goslow.link/another/?delay=3
dk8kjs.goslow.link/another/ will now respond with 3 seconds delay.
Response is '{"another": "response"}'
```

Now you have two urls responding with different JSON and delay.
```shell
time curl -d @- dk8kjs.goslow.link/another/
'{"another": "response"}'
3.182 total
```

```shell
time curl -d @- dk8kjs.goslow.link/users
'{"my": "response"}'
5.028 total
```

## Slow start
If you think that relying on unprotected by passwords third-party domains is a
bad idea, then you're probably right.

You can install goslow on your own server:

```shell
go get github.com/a-ershov/goslow
go build
bin/goslow
listening on :5103
```

And configure it with POST requests
```shell
echo '{"local": "response"}' | curl -d @- localhost:5103/goslow/local?delay=4
localhost:5103/goslow/local will now respond with 4 seconds delay.
Response is echo '{"local": "response"}'
```

You can also proxy goslow requests directly to your API with extra delay:
```shell
echo 'http://your.api' | curl -d @- localhost:5103/goslow?proxy&delay=10
```

It works as expected:
```shell
time curl localhost:5103/some/url
# proxies to http://your.api/some/url
10.123 total
```




## Contributing
Contributing to goslow is easy.
First, we need you to sign a contributor's agreement.
Second, we need your boss to sign a waiver that she's okay with you
contributing to goslow.

Just kidding. Open pull requests, send emails with patches/tarballs/links to pastebin
to [codumentary.com@gmail.com](mailto:codumentary.com@gmail.com) Whatever makes you happy.

## License
MIT
