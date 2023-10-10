njump
=====

a nostr static gateway.

it displays notes and other stuff from inside nostr as HTML with previews.


Docker
=====
To build and run in a docker container:

1. `docker build -t njump .`
2. `docker run -p 2999:2999`

The Dockerfile has two environment variables that can be overridden to set a port and set the canonical name used in the application. The defaults are set to port 2999 and canonical name `njump.me`. This change must be done _before_ building the docker image.