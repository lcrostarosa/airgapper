
interface CopyButtonProps {
  /** Whether the text was recently copied */
  copied: boolean;
  /** Click handler */
  onClick: () => void;
  /** Additional CSS classes */
  className?: string;
  /** Size variant */
  size?: 'sm' | 'md';
}

/**
 * A button that shows copy/copied state with icons.
 *
 * @example
 * const { copied, copy } = useClipboard();
 * <CopyButton copied={copied} onClick={() => copy(text)} />
 */
export function CopyButton({
  copied,
  onClick,
  className = '',
  size = 'md',
}: CopyButtonProps) {
  const sizeClasses = size === 'sm'
    ? 'px-2 py-1 text-sm'
    : 'px-3 py-2';

  return (
    <button
      onClick={onClick}
      className={`${sizeClasses} bg-gray-700 hover:bg-gray-600 rounded transition-colors ${className}`}
      title={copied ? 'Copied!' : 'Copy to clipboard'}
    >
      {copied ? '✓' : '📋'}
    </button>
  );
}
