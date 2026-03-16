# Goqu Notebooks

[Goqu](https://github.com/splch/goqu)
[gonb](https://github.com/janpfeifer/gonb)

## Setup

```bash
pip install jupyter
go install github.com/janpfeifer/gonb@latest && gonb --install
go install golang.org/x/tools/cmd/goimports@latest
go install golang.org/x/tools/gopls@latest
```

## Running

```bash
cd notebooks
jupyter notebook
```

## Notes

- Variables shared across cells are declared at the package level using `var` blocks
- Imports are placed in their own declaration cells (no `%%` prefix)
- Executable cells start with `%%`
