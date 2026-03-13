# litcode

`litcode` checks that markdown documentation stays aligned with source code.

Install it:

```sh
go install github.com/semistrict/litcode@latest
```

Initialize a repository:

```sh
litcode init
```

That writes a `.litcode.json` with `docs`, `source`, `lenient`, and `exclude`
arrays using file patterns relative to the repository root. `lenient` files are
still validated when referenced, but they do not require full line-by-line
documentation coverage.

Or run it without installing:

```sh
go run github.com/semistrict/litcode@latest check
```

The repository also includes a Codex skill for using `litcode` against other repositories:

- [skill/SKILL.md](/Users/ramon/src/lit/skill/SKILL.md)
