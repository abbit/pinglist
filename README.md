# pinglist

`pinglist` is a simple CLI tool like `ping`, but can ping multiple hosts defined via txt file.

It will show you table of stats for each ping target. Stats includes

- Packet Loss in %
- RTT Avg (round-trip time average)
- RTT Std Dev (round-trip time standard deviation)

## Install

You can install it to your `$GOPATH/bin` with `go install`

```bash
$ go install github.com/abbit/pinglist@latest
```

## Usage

```bash
$ pinglist sample-list.txt
```

Txt file defines list of ping targets with their name and IP or hostname as `name|host`.

Example:

```txt
display name for host|123.123.123.123
another host|example.com
```

Also you can see `sample-list.txt` for example.
