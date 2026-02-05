import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import type { Plugin } from 'vite'

// Security headers plugin for development server
function securityHeadersPlugin(): Plugin {
  return {
    name: 'security-headers',
    configureServer(server) {
      server.middlewares.use((_req, res, next) => {
        // Content Security Policy
        res.setHeader(
          'Content-Security-Policy',
          [
            "default-src 'self'",
            "script-src 'self' 'unsafe-inline' 'unsafe-eval'", // Vite HMR needs these in dev
            "style-src 'self' 'unsafe-inline'",
            "img-src 'self' data: blob:",
            "font-src 'self'",
            "connect-src 'self' http://localhost:* ws://localhost:*", // API and HMR
            "frame-ancestors 'none'",
            "form-action 'self'",
            "base-uri 'self'",
          ].join('; ')
        );

        // Prevent clickjacking
        res.setHeader('X-Frame-Options', 'DENY');

        // Prevent MIME type sniffing
        res.setHeader('X-Content-Type-Options', 'nosniff');

        // Enable XSS protection (legacy browsers)
        res.setHeader('X-XSS-Protection', '1; mode=block');

        // Referrer policy
        res.setHeader('Referrer-Policy', 'strict-origin-when-cross-origin');

        // Permissions policy (disable dangerous features)
        res.setHeader(
          'Permissions-Policy',
          'camera=(), microphone=(), geolocation=(), payment=()'
        );

        next();
      });
    },
  };
}

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), securityHeadersPlugin()],
  build: {
    // Generate source maps only for error reporting, not for debugging
    sourcemap: false,
    // Minify output using esbuild (faster and built-in)
    minify: 'esbuild',
  },
  esbuild: {
    // Remove console.log and debugger in production
    drop: ['console', 'debugger'],
  },
  // Preview server (production build preview) also gets security headers
  preview: {
    headers: {
      'Content-Security-Policy': [
        "default-src 'self'",
        "script-src 'self'",
        "style-src 'self' 'unsafe-inline'",
        "img-src 'self' data: blob:",
        "font-src 'self'",
        "connect-src 'self'",
        "frame-ancestors 'none'",
        "form-action 'self'",
        "base-uri 'self'",
      ].join('; '),
      'X-Frame-Options': 'DENY',
      'X-Content-Type-Options': 'nosniff',
      'X-XSS-Protection': '1; mode=block',
      'Referrer-Policy': 'strict-origin-when-cross-origin',
      'Permissions-Policy': 'camera=(), microphone=(), geolocation=(), payment=()',
    },
  },
})
