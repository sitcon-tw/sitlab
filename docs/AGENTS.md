# Documentation Guide

`docs/` is the runnable Astro reference for developers and operators. It publishes the generated OpenAPI document and the shared UI Storybook alongside architecture and deployment guidance.

## Rules

- Document behavior that exists. Update docs in the same change that alters a command, config key, contract, or boundary.
- Keep the first page useful documentation, not a marketing landing page.
- Reuse `@project-template/ui` tokens. Docs-only layout CSS belongs in `src/styles/global.css`.
- Do not hand-edit `public/openapi.json` or `public/storybook/`.
- Keep examples safe for localhost and explicitly label production secret requirements.

## Validation

- `pnpm --filter @project-template/docs check`
- `pnpm --filter @project-template/docs build:site`
- `pnpm --filter @project-template/docs build` to include Storybook
