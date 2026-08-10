# Frontend (Vue 3 + shadcn-vue)

Vite + Vue 3 + TypeScript frontend for the LRSS Wails app.

## Scripts

```bash
npm install
npm run dev      # Vite dev server (usually started via `wails3 task dev`)
npm run build    # Type-check + production bundle → dist/
npm run preview  # Preview production build
```

## UI components

shadcn-vue is configured via `components.json`. Components are under `src/components/ui`.

```bash
npx shadcn-vue@latest add dialog select
```

Import with the `@/` alias:

```vue
<script setup lang="ts">
import { Button } from "@/components/ui/button";
</script>
```

## Bindings

Go service bindings are generated into `bindings/` by Wails. Do not edit them by hand.
