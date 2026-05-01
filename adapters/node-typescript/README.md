# Node / TypeScript Adapter

This adapter profile documents the first Node / TypeScript conventions that the
Nomos detector treats as auditable product signals.

## Backend Conventions

- API routes: Next.js `app/api/**/route.ts`, Next.js `pages/api/**`, Express or
  Fastify route handler calls, and route/controller modules under `routes/` or
  `controllers/`.
- Services: modules under `services/`, files ending in `service.ts` or
  `.service.ts`, and exported service symbols.
- Mocks: `__mocks__/`, `mocks/`, `.mock.ts`, MSW handlers, `jest.mock`, and
  `vi.mock`.

## Frontend Conventions

- UI routes: Next.js `app/**/page.tsx` and `pages/**` components, excluding
  framework bootstrap files such as `_app` and `_document`.
- Fixtures: `fixtures/`, `__fixtures__/`, `testdata/`, and Cypress fixtures.
- Hardcoded catalogues: TypeScript constants or path conventions that expose
  catalogues, plans, statuses, tiers, products, options, or similar business
  enumerations without an upstream canonical source.

## Official Fixtures

`fixtures/nextjs-api-ui` is the compatibility fixture for NOM-402. It contains
API routes, UI routes, services, mocks, fixtures, and a hardcoded catalogue so
the detector can prove each adapter signal.
