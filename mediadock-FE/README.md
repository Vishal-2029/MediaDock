# MediaDock Frontend (`mediadock-FE`)

Modern, responsive React + TypeScript web application for **MediaDock** — **Your Media. One Simple Destination.**

---

## Tech Stack

- **Framework**: React 19 + TypeScript + Vite
- **Styling & Design System**: Tailwind CSS (dark mode default, persisted light/dark theme toggle) + reusable accessible UI primitives (`Button`, `Card`, `Input`, `Dialog`, `Progress`, `Badge`, `ToastViewport`) + Lucide icons + Framer Motion
- **Routing & State**: React Router v7, TanStack Query v5, Zustand
- **Forms & Validation**: React Hook Form + Zod
- **HTTP Client**: Axios (`withCredentials: true` for `HttpOnly` cookie session handling + automatic refresh on `401`)
- **Testing**: Vitest + React Testing Library + `jsdom`

---

## Folder Structure

```text
mediadock-FE/
├── public/                 # Favicon, SVG logo, robots.txt, sitemap.xml, placeholders
├── src/
│   ├── components/
│   │   ├── common/         # Design-system UI primitives & PageMeta SEO helper
│   │   ├── forms/          # UrlInspectForm (Zod URL validation, paste/clear, auth options)
│   │   ├── layout/         # AppLayout, Footer
│   │   ├── media/          # PlatformBadge, MediaPreviewCard, ProcessingStatusCard
│   │   └── navigation/     # Sticky responsive Navbar, ThemeToggle
│   ├── constants/          # Supported vs planned platform registry
│   ├── features/           # Domain hooks & feature components (auth, download, preview, history, conversion)
│   ├── hooks/              # Shared hooks (useDebounce, useTheme)
│   ├── pages/              # Home, Downloader, History, MyMedia, Converters, Settings, Login, Register, NotFound
│   ├── routes/             # AppRouter & ProtectedRoute
│   ├── services/           # Typed Axios API clients
│   ├── store/              # Zustand stores (auth, theme, toast)
│   ├── styles/             # Global CSS tokens & Tailwind utilities
│   ├── types/              # Shared API TypeScript types
│   └── utils/              # URL detection, formatting, className helpers
├── Dockerfile              # Multi-stage Node build + Nginx static server
├── nginx.conf              # SPA fallback + reverse proxy rules
├── package.json
└── vite.config.ts
```

---

## Local Development

```bash
cd mediadock-FE
cp .env.example .env
npm install
npm run dev
```

Vite dev server starts at `http://localhost:5173` and proxies `/api` requests to `http://localhost:8080`.

### Scripts

| Command | Description |
|---|---|
| `npm run dev` | Start Vite development server |
| `npm run lint` | Run TypeScript type checking (`tsc --noEmit`) |
| `npm test` | Run Vitest unit & component tests |
| `npm run build` | Type-check and build optimized production bundle in `dist/` |
| `npm run preview` | Preview built production bundle locally |

