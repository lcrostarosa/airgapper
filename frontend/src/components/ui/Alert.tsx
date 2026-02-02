import React from 'react';

type AlertVariant = 'error' | 'warning' | 'success' | 'info';

interface AlertProps {
  /** Alert variant determining color scheme */
  variant: AlertVariant;
  /** Alert message content */
  children: React.ReactNode;
  /** Optional title */
  title?: string;
  /** Whether the alert can be dismissed */
  dismissible?: boolean;
  /** Callback when dismissed */
  onDismiss?: () => void;
  /** Additional CSS classes */
  className?: string;
}

const variantStyles: Record<AlertVariant, { bg: string; border: string; text: string; icon: string }> = {
  error: {
    bg: 'bg-red-900/30',
    border: 'border-red-500/50',
    text: 'text-red-400',
    icon: '❌',
  },
  warning: {
    bg: 'bg-yellow-900/30',
    border: 'border-yellow-500/50',
    text: 'text-yellow-400',
    icon: '⚠️',
  },
  success: {
    bg: 'bg-green-900/30',
    border: 'border-green-500/50',
    text: 'text-green-400',
    icon: '✓',
  },
  info: {
    bg: 'bg-blue-900/30',
    border: 'border-blue-500/50',
    text: 'text-blue-400',
    icon: 'ℹ️',
  },
};

/**
 * An alert component for displaying messages with different severity levels.
 *
 * @example
 * <Alert variant="error">Something went wrong</Alert>
 *
 * @example
 * <Alert variant="success" title="Success!">
 *   Your changes have been saved.
 * </Alert>
 *
 * @example
 * <Alert variant="warning" dismissible onDismiss={() => setShow(false)}>
 *   This action cannot be undone.
 * </Alert>
 */
export function Alert({
  variant,
  children,
  title,
  dismissible = false,
  onDismiss,
  className = '',
}: AlertProps) {
  const styles = variantStyles[variant];

  return (
    <div
      className={`p-3 ${styles.bg} border ${styles.border} rounded-lg ${styles.text} text-sm ${className}`}
      role="alert"
    >
      <div className="flex items-start gap-2">
        <span className="flex-shrink-0">{styles.icon}</span>
        <div className="flex-1">
          {title && <div className="font-medium mb-1">{title}</div>}
          <div>{children}</div>
        </div>
        {dismissible && (
          <button
            onClick={onDismiss}
            className={`flex-shrink-0 ${styles.text} hover:opacity-75 transition-opacity`}
            aria-label="Dismiss"
          >
            ✕
          </button>
        )}
      </div>
    </div>
  );
}
