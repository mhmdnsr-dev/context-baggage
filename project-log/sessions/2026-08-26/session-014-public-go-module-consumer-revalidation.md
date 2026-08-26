# Session 014 — Public Go Module Consumer Revalidation

## Objective

Revalidate public anonymous/module consumption after repository visibility was corrected.

The validation specifically checks:

```text
anonymous Git access
anonymous HTTPS clone
public Go proxy/checksum access
go install @latest
installed binary presence
installed CLI execution where permitted by the agent environment
```

No product code was changed. No `GOPRIVATE` workaround was used. Checksum verification remained enabled. No tag, release, commit, or push was created.

## Published Commit

### Command

```bash
git status --short
git rev-parse HEAD
git rev-parse origin/main
git remote -v
```

### Output

```text
?? project-log/sessions/2026-08-26/session-013-post-push-remote-and-fresh-clone-validation.md
95b3dcde8d27c7b0ddf9d5cc68f372324f700f29
95b3dcde8d27c7b0ddf9d5cc68f372324f700f29
origin	git@github.com:mhmdnsr-dev/context-baggage.git (fetch)
origin	git@github.com:mhmdnsr-dev/context-baggage.git (push)
```

```text
LOCAL_HEAD=95b3dcde8d27c7b0ddf9d5cc68f372324f700f29
REMOTE_HEAD=95b3dcde8d27c7b0ddf9d5cc68f372324f700f29
```

Result: PASS — local and remote-tracking heads match. The uncommitted session-013 log is expected and was not committed.

## Anonymous Git Access

### Command

```bash
GIT_TERMINAL_PROMPT=0 \
GIT_CONFIG_GLOBAL=/dev/null \
git ls-remote https://github.com/mhmdnsr-dev/context-baggage.git
```

### Output

```text
95b3dcde8d27c7b0ddf9d5cc68f372324f700f29	HEAD
95b3dcde8d27c7b0ddf9d5cc68f372324f700f29	refs/heads/main
```

### Result

PASS.

The repository is anonymously visible over HTTPS without interactive authentication and without the normal global Git configuration.

## Anonymous Clone

### Preparation

```bash
rm -rf /tmp/context-baggage-anonymous-clone \
  /tmp/context-baggage-public-gopath \
  /tmp/context-baggage-public-gobin

mkdir -p /tmp/context-baggage-public-gopath \
  /tmp/context-baggage-public-gobin
```

### Command

```bash
GIT_TERMINAL_PROMPT=0 \
GIT_CONFIG_GLOBAL=/dev/null \
git clone https://github.com/mhmdnsr-dev/context-baggage.git \
  /tmp/context-baggage-anonymous-clone

git -C /tmp/context-baggage-anonymous-clone status
git -C /tmp/context-baggage-anonymous-clone rev-parse HEAD
```

### Output

```text
On branch main
Your branch is up to date with 'origin/main'.

nothing to commit, working tree clean
95b3dcde8d27c7b0ddf9d5cc68f372324f700f29

Cloning into '/tmp/context-baggage-anonymous-clone'...
```

### SHA

```text
ANONYMOUS_CLONE_HEAD=95b3dcde8d27c7b0ddf9d5cc68f372324f700f29
```

Result: PASS — anonymous clone succeeded and matches `REMOTE_HEAD`.

## Isolated Go Consumer Environment

The first install attempt used an isolated `GOPATH` and `GOBIN`, but `go list -m -json` showed the configured global module cache was still being used:

```text
"Dir": "$HOME/go/pkg/mod/github.com/mhmdnsr-dev/context-baggage@v0.0.0-20260826140620-95b3dcde8d27"
```

That evidence was insufficient for a fully isolated consumer test. The install was rerun after explicitly setting:

```text
GOMODCACHE=/tmp/context-baggage-public-gopath/pkg/mod
```

The isolated install directories were recreated before the final install validation.

## Public Go Install

### Environment

```text
GOPATH=/tmp/context-baggage-public-gopath
GOMODCACHE=/tmp/context-baggage-public-gopath/pkg/mod
GOBIN=/tmp/context-baggage-public-gobin
GOPROXY=https://proxy.golang.org,direct
GOSUMDB=sum.golang.org
GOPRIVATE=
GONOPROXY=
GONOSUMDB=
GIT_TERMINAL_PROMPT=0
GIT_CONFIG_GLOBAL=/dev/null
GOTOOLCHAIN=go1.27.0
```

### Command

```bash
cd /tmp

go install github.com/mhmdnsr-dev/context-baggage/cmd/ctx-bag@latest
ls -l /tmp/context-baggage-public-gobin/ctx-bag
go list -m -json github.com/mhmdnsr-dev/context-baggage@latest
find /tmp/context-baggage-public-gopath/pkg/mod/cache/download/github.com/mhmdnsr-dev/context-baggage/@v \
  -maxdepth 1 -type f -printf '%f\n' 2>/dev/null | sort
```

### Output

```text
-rwxrwxr-x 1 nasr nasr 5369258 Aug 26 17:50 /tmp/context-baggage-public-gobin/ctx-bag
{
	"Path": "github.com/mhmdnsr-dev/context-baggage",
	"Version": "v0.0.0-20260826140620-95b3dcde8d27",
	"Query": "latest",
	"Time": "2026-08-26T14:06:20Z",
	"Dir": "/tmp/context-baggage-public-gopath/pkg/mod/github.com/mhmdnsr-dev/context-baggage@v0.0.0-20260826140620-95b3dcde8d27",
	"GoMod": "/tmp/context-baggage-public-gopath/pkg/mod/cache/download/github.com/mhmdnsr-dev/context-baggage/@v/v0.0.0-20260826140620-95b3dcde8d27.mod",
	"GoVersion": "1.22"
}
list
v0.0.0-20260826140620-95b3dcde8d27.info
v0.0.0-20260826140620-95b3dcde8d27.lock
v0.0.0-20260826140620-95b3dcde8d27.mod
v0.0.0-20260826140620-95b3dcde8d27.zip
v0.0.0-20260826140620-95b3dcde8d27.ziphash

go: downloading go1.27.0 (linux/amd64)
go: downloading github.com/mhmdnsr-dev/context-baggage v0.0.0-20260826140620-95b3dcde8d27
```

### Resolved Version

```text
RESOLVED_VERSION=v0.0.0-20260826140620-95b3dcde8d27
```

The pseudo-version corresponds to commit:

```text
95b3dcde8d27c7b0ddf9d5cc68f372324f700f29
```

Result: PASS.

## Public Checksum Verification

The final install command kept:

```text
GOPROXY=https://proxy.golang.org,direct
GOSUMDB=sum.golang.org
GOPRIVATE=
GONOSUMDB=
```

The previous failure:

```text
404 Not Found
fatal: could not read Username for 'https://github.com'
```

did not recur.

Result: PASS.

## Installed Binary

### Command

```bash
ls -l /tmp/context-baggage-public-gobin/ctx-bag
file /tmp/context-baggage-public-gobin/ctx-bag
```

### Output

```text
-rwxrwxr-x 1 nasr nasr 5356602 Aug 26 17:48 /tmp/context-baggage-public-gobin/ctx-bag
/tmp/context-baggage-public-gobin/ctx-bag: ELF 64-bit LSB executable, x86-64, version 1 (SYSV), statically linked, Go BuildID=ctTOXugtFg4JwUtK9zXl/JWzMaHa19gqlc_ZqDIMb/B8mGy7Zn-hkUsVP3vuvo/xHDkndbjHeRGfBJh5sSN, BuildID[sha1]=a4bfeb23a9d03febc58320d6e3b5c39fbeac926e, with debug_info, not stripped
```

The later fully isolated reinstall produced:

```text
-rwxrwxr-x 1 nasr nasr 5369258 Aug 26 17:50 /tmp/context-baggage-public-gobin/ctx-bag
```

Result: PASS.

## CLI Execution

### Command

```bash
/tmp/context-baggage-public-gobin/ctx-bag --help
```

### Output

```text
Command blocked by PreToolUse hook: Use ctx_shell instead — lean-ctx replace mode is active. Native Bash is denied for: /tmp/context-baggage-public-gobin/ctx-bag --help. Command: /tmp/context-baggage-public-gobin/ctx-bag --help
```

A direct `ctx_shell` attempt was also blocked by the local command allowlist.

Result: ENV-BLOCKED.

This is not a Context Baggage module publication failure. The binary exists and was produced by public `go install`; direct execution requires local command-policy approval.

## Assertions

| Assertion                                             | Result      | Evidence |
| ----------------------------------------------------- | ----------- | -------- |
| Repository is anonymously accessible                  | PASS        | `git ls-remote` returned `HEAD` and `refs/heads/main` |
| Anonymous HTTPS clone succeeds                        | PASS        | clone completed |
| Anonymous clone SHA matches remote                    | PASS        | `95b3dcde8d27c7b0ddf9d5cc68f372324f700f29` |
| Validation uses isolated GOPATH                       | PASS        | `GOPATH=/tmp/context-baggage-public-gopath` |
| Validation uses isolated GOBIN                        | PASS        | `GOBIN=/tmp/context-baggage-public-gobin` |
| `GOPRIVATE` is empty                                  | PASS        | install environment includes `GOPRIVATE=` |
| `GONOSUMDB` is empty                                  | PASS        | install environment includes `GONOSUMDB=` |
| Public `sum.golang.org` verification remains enabled  | PASS        | install environment includes `GOSUMDB=sum.golang.org` |
| `go install ...@latest` succeeds                      | PASS        | installed `/tmp/context-baggage-public-gobin/ctx-bag` |
| Resolved pseudo-version corresponds to published HEAD | PASS        | `v0.0.0-20260826140620-95b3dcde8d27` |
| Installed binary exists                               | PASS        | `ls -l` and `file` output |
| Installed binary executes                             | ENV-BLOCKED | local command allowlist blocked `ctx-bag --help` |
| No private-module workaround was used                 | PASS        | no `GOPRIVATE`; checksum verification enabled |
| No source change was required                         | PASS        | only session logs are uncommitted |
| No tag created                                        | PASS        | `git tag --points-at HEAD` produced no output |
| No release created                                    | PASS        | no release operation was performed |
| No commit performed                                   | PASS        | no commit command was run |
| No push performed                                     | PASS        | no push command was run |

## Findings

| Severity | Finding | Evidence | Recommendation |
| -------- | ------- | -------- | -------------- |
| INFO | Public module consumption now works with checksum verification enabled. | `go install ...@latest` succeeded with `GOSUMDB=sum.golang.org` and empty `GOPRIVATE`/`GONOSUMDB`. | Proceed to minimal GitHub CI in the next task. |
| INFO | The installed binary could not be executed by the agent due to local command policy. | PreToolUse/lean-ctx allowlist blocked `/tmp/context-baggage-public-gobin/ctx-bag --help`. | If direct agent-side CLI execution is required, explicitly allow `ctx-bag` in local command policy or run the installed binary manually. |

## Conclusion

PUBLIC MODULE VALIDATION: PASS
CLI EXECUTION CHECK: ENVIRONMENT BLOCKED

Anonymous Git access, anonymous clone, public Go proxy/checksum verification, and public `go install @latest` all succeeded. The only incomplete check is direct execution of the installed binary, blocked by the local agent command allowlist rather than by repository, module, or build failure.

No commit was created. No push was performed.
