# commit-your-code banner

A single-file Node app (no dependencies) for the container-isolation talk.

It shows a big **"hello, commit your code!"** banner in the CYC theme, and reads
the log written by `../scripts/evilnode.sh` (the fake `node` escape probe):

- **Vulnerabilities found** → `UH OH — YOU HAVE VULNERABILITIES` + a list of every
  boundary that would let an attacker escape or leak host info.
- **All boundaries CONTAINED** → `YOU'RE SECURE`.

It re-reads the log on every request and the page auto-polls every 2.5s, so during
the talk you just re-run the probe and the browser updates itself — no restart.

## Run

```sh
node server.js
# open http://localhost:3000
```

## Config

| Env           | Default                                                           | Meaning                          |
|---------------|------------------------------------------------------------------|----------------------------------|
| `PORT`        | `3000`                                                            | HTTP port                        |
| `EVILNODE_LOG`| first of `/var/log/evilnode.log`, `/tmp/evilnode.log`, self-test | which probe log to read          |

Point `EVILNODE_LOG` at the container's log (e.g. a bind-mounted path) if you run
the app on the host but the probe inside the container.

## Presentation flow

1. Run the container with **no** namespaces → run `node` (the probe) → refresh: lots of vulns.
2. Add each `CLONE_NEW*` flag, re-run `node`, refresh: rows disappear one by one.
3. Fully hardened → `YOU'RE SECURE`.

> The log is append-only; the app parses only the **most recent** run.
