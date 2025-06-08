# njump

njump is a HTTP Nostr static gateway that allows you to browse profiles, notes and relays; it is an easy way to preview a resource and then open it with your preferred client. The typical use of njump is to share a resource outside the Nostr world, where the Nostr: schema is not (yet) working.

njump has some special features to effectively share notes on platforms that offer links preview, like Twitter and Telegram.

njump currently lives under [njump.me](https://njump.me), you can reach it appending a Nostr NIP-19 entity (npub, nevent, nprofile, naddr, etc.) or a NIP-05 address after the domain, e.g. `njump.me/nevent1xxxxxx...xxx` or `njump.me/xxxx@zzzzz.com`

For more information about njump's philosophy and its use, read the presentation [on the homepage](https://njump.me).

## Supported Kinds

| kind    | description                | NIP         |
| ------- | -------------------------- | ----------- |
| `0`     | Metadata                   | [1](https://github.com/nostr-protocol/nips/blob/master/01.md)  |
| `1`     | Short Text Note            | [1](https://github.com/nostr-protocol/nips/blob/master/01.md)  |
| `6`     | Repost                     | [18](https://github.com/nostr-protocol/nips/blob/master/18.md) |
| `11`    | Thread                     | [7D](https://github.com/nostr-protocol/nips/blob/master/7D.md) |
| `1111`  | Comment                    | [22](https://github.com/nostr-protocol/nips/blob/master/22.md) |
| `1063`  | File Metadata              | [94](https://github.com/nostr-protocol/nips/blob/master/94.md) |
| `1311`  | Live Chat Message          | [53](https://github.com/nostr-protocol/nips/blob/master/53.md) |
| `30023` | Long-form Content          | [23](https://github.com/nostr-protocol/nips/blob/master/23.md) |
| `30024` | Draft Long-form Content    | [23](https://github.com/nostr-protocol/nips/blob/master/23.md) |
| `30311` | Live Event                 | [53](https://github.com/nostr-protocol/nips/blob/master/53.md) |
| `30818` | Wiki article               | [54](https://github.com/nostr-protocol/nips/blob/master/54.md) |
| `31922` | Date-Based Calendar Event  | [52](https://github.com/nostr-protocol/nips/blob/master/52.md) |
| `31923` | Time-Based Calendar Event  | [52](https://github.com/nostr-protocol/nips/blob/master/52.md) |

## Running

### Running locally

The easiest way to start is to run the development server with `just` (if you have [it](https://just.systems/) installed) or with `TAILWIND_DEBUG=true go run .`. You can also check the contents of `justfile` to see other useful scripts.

For live-reload you can use [`air`](https://github.com/cosmtrek/air) and start it with `air -c .air.toml` -- this will run it without the local cache, which can be annoying if you're not specifically debugging the part of the code that loads content, so you may want to run it with `air -c .air.toml --build.cmd 'go build -o ./tmp/main .'`. These run modes will recompile the Tailwind bundle on every restart and they assume you have [the `tailwind` CLI](https://tailwindcss.com/docs/installation) installed globally.

### Running from a precompiled binary

You can grab one from the [releases](../../releases), unpack and run it.

### Docker

To build and run in a Docker container:

```bash
docker build -t njump .
docker run -e DOMAIN=njump.mydomain.com -p 2999:2999 njump
```

### Environment variables

These are the defaults that you can change by setting environment variables in your system before running:

```
PORT="2999"
DOMAIN="njump.me"
DISK_CACHE_PATH="/tmp/njump-internal"
EVENT_STORE_PATH="/tmp/njump-db"
TAILWIND_DEBUG=
RELAY_CONFIG_PATH=
TRUSTED_PUBKEYS=npub1...,npub1...
```

`RELAY_CONFIG_PATH` is path to json file to update relay configuration. You can set relay list like below:

```json
{
  "everything": [
    "wss://relay.nostr.band",
    "wss://nostr.lol"
  ]
}
```

See `relay-config.json.sample` for example.

For example, when running from a precompiled binary you can do something like `PORT=5000 ./njump`.

### Localization

Translation files are stored in the `locales` directory. To add support for another language, copy `en.json` to `<lang>.json` (or `.toml`) and replace each English string with its translation. The middleware automatically selects the language from the `lang` query parameter or the `Accept-Language` header.

When a translation is missing for the selected language, njump falls back to the English text.
