veepn-proxy
===========

This project inspired by Snawoot.

Standalone VeePN VPN client.

Just run it and it'll start a plain HTTP proxy server forwarding traffic through "VeePN VPN" proxies of your choice.
By default the application listens on 127.0.0.1:18090.

## Features

* Cross-platform (Windows/Mac OS/Linux/Android (via shell)/\*BSD)
* Uses TLS for secure communication with upstream proxies
* Zero configuration
* Simple and straightforward

## Installation

#### Binaries

Pre-built binaries are available [here](https://github.com/Mbrmail/veepn-proxy/releases/latest).

#### Build from source

Alternatively, you may install veepn-proxy from source. Run the following within the source directory:

```
make install
```

## Usage

List available countries:

```
$ ./veepn-proxy -list-countries
country code,country name
fr,Paris
nl,Amsterdam
ru-spb,Saint Petersburg
sg,Singapore
gb-lnd,London
us-va,Virginia
us-or,Oregon
```

Run proxy via country of your choice:

```
$ ./veepn-proxy -country fr
```

## Check your IP address

```
$ curl --proxy 127.0.0.1:18090 https://api.ipify.org
```

## List of arguments

| Argument | Type | Description |
| -------- | ---- | ----------- |
| api-address | string | override IP address of antpeak.com |
| bind-address | string | HTTP proxy listen address (default "127.0.0.1:18090") |
| bootstrap-dns | string | DNS/DoH/DoT/DoQ resolver for initial discovering of SurfEasy API address. See https://github.com/ameshkov/dnslookup/ for upstream DNS URL format. Examples: https://1.1.1.1/dns-query, quic://dns.adguard.com |
| cafile | string | use custom CA certificate bundle file |
| certchain-workaround | Bool | add bundled cross-signed intermediate cert to certchain to make it check out on old systems (default true) |
| country | string | desired proxy location (default "nl") |
| list-countries | - | list available countries and exit |
| proxy | string | sets base proxy to use for all dial-outs. Format: <http|https|socks5|socks5h>://[login:password@]host[:port] Examples: http://user:password@192.168.1.1:3128, socks5://10.0.0.1:1080 |
| refresh | duration | login refresh interval (default 4h0m0s) |
| refresh-retry | duration | login refresh retry interval (default 5s) |
| timeout | duration | timeout for network operations (default 10s) |
| verbosity | int | logging verbosity (10 - debug, 20 - info, 30 - warning, 40 - error, 50 - critical) (default 20) |
| version | - | show program version and exit |
