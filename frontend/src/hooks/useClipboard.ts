import { useState, useCallback } from 'react';

interface UseClipboardOptions {
  /** Duration in ms to show "copied" state. Default: 2000 */
  resetDelay?: number;
}

interface UseClipboardReturn {
  /** Whether the most recent copy was successful */
  copied: boolean;
  /** The identifier of the most recently copied item (for tracking multiple items) */
  copiedId: string | null;
  /** Copy text to clipboard */
  copy: (text: string, id?: string) => Promise<boolean>;
  /** Reset the copied state */
  reset: () => void;
}

/**
 * Hook for copying text to clipboard with feedback state.
 *
 * @example
 * // Simple usage
 * const { copied, copy } = useClipboard();
 * <button onClick={() => copy(text)}>{copied ? 'Copied!' : 'Copy'}</button>
 *
 * @example
 * // Multiple items with IDs
 * const { copiedId, copy } = useClipboard();
 * <button onClick={() => copy(item.text, item.id)}>
 *   {copiedId === item.id ? 'Copied!' : 'Copy'}
 * </button>
 */
export function useClipboard(options: UseClipboardOptions = {}): UseClipboardReturn {
  const { resetDelay = 2000 } = options;
  const [copied, setCopied] = useState(false);
  const [copiedId, setCopiedId] = useState<string | null>(null);

  const reset = useCallback(() => {
    setCopied(false);
    setCopiedId(null);
  }, []);

  const copy = useCallback(async (text: string, id?: string): Promise<boolean> => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setCopiedId(id ?? null);

      setTimeout(() => {
        setCopied(false);
        setCopiedId(null);
      }, resetDelay);

      return true;
    } catch (err) {
      console.error('Failed to copy to clipboard:', err);
      return false;
    }
  }, [resetDelay]);

  return { copied, copiedId, copy, reset };
}
