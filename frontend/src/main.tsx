import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'

// Disable React DevTools in production for security
// This prevents attackers from inspecting component state
if (import.meta.env.PROD) {
  // Disable React DevTools
  const disableDevTools = () => {
    const noop = () => undefined;
    const devToolsHook = (window as unknown as { __REACT_DEVTOOLS_GLOBAL_HOOK__?: unknown }).__REACT_DEVTOOLS_GLOBAL_HOOK__;

    if (typeof devToolsHook === 'object' && devToolsHook !== null) {
      Object.keys(devToolsHook).forEach((key) => {
        if (typeof (devToolsHook as Record<string, unknown>)[key] === 'function') {
          (devToolsHook as Record<string, (...args: unknown[]) => unknown>)[key] = noop;
        }
      });
    }

    // Also set a dummy hook to prevent future installation
    (window as unknown as { __REACT_DEVTOOLS_GLOBAL_HOOK__?: object }).__REACT_DEVTOOLS_GLOBAL_HOOK__ = {
      inject: noop,
      onCommitFiberRoot: noop,
      onCommitFiberUnmount: noop,
      supportsFiber: true,
    };
  };

  disableDevTools();
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
