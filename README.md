njump
=====

a nostr static gateway.

it displays notes and other stuff from inside nostr as HTML with previews.


Docker
=====
To build and run in a docker container
```bash
docker build -t njump .
docker run -p 2999:2999 njump
```

You can also override these two environment variables:
- CANONICAL_HOST
    - Defaults to `njump.me`
- PORT
    - Defaults to `2999`

Example:
```bash
docker run -e CANONICAL_HOST=njump.mydomain.com -e PORT=2999 -p 2999:2999 njump
```

