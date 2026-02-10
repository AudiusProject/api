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

## Deployment

After building, the Go server will serve the built files from `./static/plans/dist/` at the `/plans` route.

The app is configured to:
- Serve static assets from `/plans/assets/`
- Serve the SPA from `/plans/*` routes
