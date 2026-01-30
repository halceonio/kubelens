# GitHub Pages Publishing

## Enable GitHub Pages
1) In the repository settings, open **Pages**.
2) Set **Source** to **GitHub Actions**.
3) Save. GitHub will publish the site at:
```
https://<org>.github.io/<repo>/
```

## Local preview (optional)
Docs are built with MkDocs Material. Preview locally with:
```bash
make docs-preview
```

## Notes
- The docs entry point is `docs/index.md`.
- The Pages build is handled by `.github/workflows/docs.yml`.
