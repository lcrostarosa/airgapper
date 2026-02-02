import React from 'react';

interface FormInputProps extends Omit<React.InputHTMLAttributes<HTMLInputElement>, 'size'> {
  /** Input label */
  label?: string;
  /** Helper text below input */
  helperText?: string;
  /** Error message (shows in red) */
  error?: string;
  /** Size variant */
  size?: 'sm' | 'md' | 'lg';
}

interface FormTextareaProps extends React.TextareaHTMLAttributes<HTMLTextAreaElement> {
  /** Textarea label */
  label?: string;
  /** Helper text below textarea */
  helperText?: string;
  /** Error message (shows in red) */
  error?: string;
}

interface FormSelectProps extends React.SelectHTMLAttributes<HTMLSelectElement> {
  /** Select label */
  label?: string;
  /** Helper text below select */
  helperText?: string;
  /** Error message (shows in red) */
  error?: string;
  /** Options for the select */
  options: Array<{ value: string; label: string }>;
}

const baseInputClasses = 'w-full bg-gray-900 border border-gray-700 rounded-lg focus:outline-none focus:border-blue-500 transition-colors';

const sizeClasses = {
  sm: 'px-3 py-2 text-sm',
  md: 'px-4 py-3',
  lg: 'px-4 py-4 text-lg',
};

/**
 * A styled form input component.
 *
 * @example
 * <FormInput
 *   label="Email"
 *   type="email"
 *   value={email}
 *   onChange={(e) => setEmail(e.target.value)}
 *   placeholder="you@example.com"
 * />
 *
 * @example
 * <FormInput
 *   label="Password"
 *   type="password"
 *   error={errors.password}
 *   helperText="Must be at least 8 characters"
 * />
 */
export function FormInput({
  label,
  helperText,
  error,
  size = 'md',
  className = '',
  ...props
}: FormInputProps) {
  const hasError = Boolean(error);

  return (
    <div className={className}>
      {label && (
        <label className="block text-sm font-medium mb-2 text-gray-300">
          {label}
        </label>
      )}
      <input
        className={`${baseInputClasses} ${sizeClasses[size]} ${
          hasError ? 'border-red-500 focus:border-red-500' : ''
        }`}
        {...props}
      />
      {error && <p className="mt-1 text-sm text-red-400">{error}</p>}
      {helperText && !error && (
        <p className="mt-1 text-sm text-gray-500">{helperText}</p>
      )}
    </div>
  );
}

/**
 * A styled textarea component.
 *
 * @example
 * <FormTextarea
 *   label="Description"
 *   value={description}
 *   onChange={(e) => setDescription(e.target.value)}
 *   rows={4}
 * />
 */
export function FormTextarea({
  label,
  helperText,
  error,
  className = '',
  ...props
}: FormTextareaProps) {
  const hasError = Boolean(error);

  return (
    <div className={className}>
      {label && (
        <label className="block text-sm font-medium mb-2 text-gray-300">
          {label}
        </label>
      )}
      <textarea
        className={`${baseInputClasses} px-4 py-3 ${
          hasError ? 'border-red-500 focus:border-red-500' : ''
        }`}
        {...props}
      />
      {error && <p className="mt-1 text-sm text-red-400">{error}</p>}
      {helperText && !error && (
        <p className="mt-1 text-sm text-gray-500">{helperText}</p>
      )}
    </div>
  );
}

/**
 * A styled select component.
 *
 * @example
 * <FormSelect
 *   label="Country"
 *   value={country}
 *   onChange={(e) => setCountry(e.target.value)}
 *   options={[
 *     { value: 'us', label: 'United States' },
 *     { value: 'ca', label: 'Canada' },
 *   ]}
 * />
 */
export function FormSelect({
  label,
  helperText,
  error,
  options,
  className = '',
  ...props
}: FormSelectProps) {
  const hasError = Boolean(error);

  return (
    <div className={className}>
      {label && (
        <label className="block text-sm font-medium mb-2 text-gray-300">
          {label}
        </label>
      )}
      <select
        className={`${baseInputClasses} px-4 py-3 ${
          hasError ? 'border-red-500 focus:border-red-500' : ''
        }`}
        {...props}
      >
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
      {error && <p className="mt-1 text-sm text-red-400">{error}</p>}
      {helperText && !error && (
        <p className="mt-1 text-sm text-gray-500">{helperText}</p>
      )}
    </div>
  );
}
