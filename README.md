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

That writes a `.litcode.jsonc` with `docs`, `source`, `lenient`, and `exclude`
arrays using file patterns relative to the repository root. `lenient` files are
still validated when referenced, but they do not require full line-by-line
documentation coverage.

Or run it without installing:

```sh
go run github.com/semistrict/litcode@latest check
```

## Demo

Short demo of `litcode init` and `litcode check`:

![litcode demo](assets/litcode-demo.gif)

The repository also includes a Codex skill for using `litcode` against other repositories:

- [skill/SKILL.md](/Users/ramon/src/lit/skill/SKILL.md)

## GitHub Pages

This repo includes a GitHub Actions workflow at
`.github/workflows/pages.yml` that builds the HTML docs with:

```sh
make html-docs
```

and deploys the generated site to GitHub Pages on pushes to `main`.

Live docs (for this repo):

- https://semistrict.github.io/litcode/
