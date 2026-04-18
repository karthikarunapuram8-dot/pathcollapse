# v0.2.1 Release Prep

This is the first patch release that should carry the corrected module path:

`github.com/karthikarunapuram8-dot/pathcollapse`

## Why v0.2.1 exists

- `v0.2.0` shipped valid binaries and release assets.
- The module-path rename landed after `v0.2.0` was tagged.
- As a result:
  - release downloads from `v0.2.0` are fine
  - `go install github.com/karthikarunapuram8-dot/pathcollapse/cmd/pathcollapse@main` works
  - `go install github.com/karthikarunapuram8-dot/pathcollapse/cmd/pathcollapse@v0.2.0` does not

`v0.2.1` is the first semver tag that fixes tagged `go install`.

## Suggested scope

Keep `v0.2.1` intentionally small:

1. One user-visible fix or polish item
2. The already-landed module-path correction
3. No large feature additions

Good candidates:

- issue `#8` — suppress the cold-start confidence note in non-TTY or `--quiet` runs
- issue `#9` — only if kept very small; otherwise save for `v0.3.0`

## Release checklist

1. Ensure `main` is green:
   - `go test ./...`
   - `go vet ./...`
2. Confirm `go.mod` still declares:
   - `module github.com/karthikarunapuram8-dot/pathcollapse`
3. Update `CHANGELOG.md` with the final `v0.2.1` notes.
4. Tag from `main`:

   ```bash
   git tag -a v0.2.1 -m "PathCollapse v0.2.1"
   git push origin v0.2.1
   ```

5. Publish the GitHub release.
6. Verify after release:
   - GitHub Actions / GoReleaser run passed
   - release binaries uploaded
   - this command succeeds:

   ```bash
   go install github.com/karthikarunapuram8-dot/pathcollapse/cmd/pathcollapse@v0.2.1
   ```

## Release-note sentence to keep

Use some version of this in the release notes:

> `v0.2.1` is the first tagged release whose Go module path matches the public
> repository URL, so tagged `go install` now works as expected.
