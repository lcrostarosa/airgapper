import { CopyButton } from './CopyButton';
import { useClipboard } from '../../hooks/useClipboard';

interface CopyableFieldProps {
  /** The text value to display and copy */
  value: string;
  /** Optional label above the field */
  label?: string;
  /** Whether to use monospace font */
  mono?: boolean;
  /** Whether to show the value in a code block style */
  code?: boolean;
  /** Whether to allow text to wrap or truncate */
  wrap?: boolean;
  /** Additional CSS classes for the container */
  className?: string;
  /** Unique identifier for tracking multiple fields */
  id?: string;
  /** External copied state (for when using shared clipboard state) */
  externalCopied?: boolean;
  /** External copy handler (for when using shared clipboard state) */
  onCopy?: () => void;
}

/**
 * A field that displays a value with a copy button.
 *
 * @example
 * // Self-managed clipboard state
 * <CopyableField value={apiKey} label="API Key" mono />
 *
 * @example
 * // Shared clipboard state
 * const { copiedId, copy } = useClipboard();
 * <CopyableField
 *   value={key}
 *   id="key-1"
 *   externalCopied={copiedId === 'key-1'}
 *   onCopy={() => copy(key, 'key-1')}
 * />
 */
export function CopyableField({
  value,
  label,
  mono = false,
  code = true,
  wrap = false,
  className = '',
  id,
  externalCopied,
  onCopy,
}: CopyableFieldProps) {
  const internalClipboard = useClipboard();

  // Use external state if provided, otherwise use internal
  const copied = externalCopied !== undefined ? externalCopied : internalClipboard.copied;
  const handleCopy = onCopy ?? (() => internalClipboard.copy(value, id));

  const textClasses = [
    'flex-1 rounded px-3 py-2 text-sm',
    code ? 'bg-gray-900' : 'bg-gray-800',
    mono || code ? 'font-mono' : '',
    wrap ? '' : 'break-all',
  ].filter(Boolean).join(' ');

  return (
    <div className={className}>
      {label && (
        <label className="block text-sm font-medium mb-2 text-gray-300">
          {label}
        </label>
      )}
      <div className="flex gap-2">
        <code className={textClasses}>{value}</code>
        <CopyButton copied={copied} onClick={handleCopy} />
      </div>
    </div>
  );
}
