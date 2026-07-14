# Project Template docs

The Astro documentation site includes developer guides, architecture references, the generated OpenAPI explorer, and published Storybook output.

```bash
pnpm --filter @project-template/docs dev
pnpm --filter @project-template/docs check
pnpm --filter @project-template/docs build
```

`pnpm generate` owns `public/openapi.json`. The full docs build owns `public/storybook/`; neither directory should be edited by hand.
