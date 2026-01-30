# GitHub Pages Publishing

## Enable GitHub Pages
1) In the repository settings, open **Pages**.
2) Set **Source** to the `main` branch.
3) Set **Folder** to `/docs`.
4) Save. GitHub will publish the site at:
```
https://<org>.github.io/<repo>/
```

## Local preview (optional)
GitHub Pages uses Jekyll. You can preview locally with:
```bash
bundle exec jekyll serve --source docs
```

## Notes
- The docs entry point is `docs/index.md`.
- The Pages build will render markdown automatically.

