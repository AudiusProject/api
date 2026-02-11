# Audius API Plans

React application for displaying Audius API plans at `api.audius.co/plans`.

## Setup

1. Install dependencies:
```bash
npm install
```

2. Set environment variable:
```bash
export VITE_AUDIUS_API_KEY=your_api_key_here
```

Or create a `.env` file:
```
VITE_AUDIUS_API_KEY=your_api_key_here
```

## Development

Run the development server:
```bash
npm run dev
```

## Building

Build for production:
```bash
npm run build
```

This will create a `dist/` directory with the built files that will be served by the Go API server.

## Create Key (local dev)

To create developer apps locally, the **API server** (not this app) must have `PLANS_APP_API_SECRET` set. Add to the API root `.env`:

```
PLANS_APP_API_SECRET=<ethereum_private_key_hex>
```

This is the private key for the OAuth app. It must match the address of the app identified by `VITE_AUDIUS_API_KEY`. For local dev you can generate a key with `openssl rand -hex 32` and register a new OAuth app, or use the key from your staging/production plans app config.

## Deployment

After building, the Go server will serve the built files from `./static/plans/dist/` at the `/plans` route.

The app is configured to:
- Serve static assets from `/plans/assets/`
- Serve the SPA from `/plans/*` routes
