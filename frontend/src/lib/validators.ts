/**
 * Input validation utilities for security
 */

export interface ValidationResult {
  valid: boolean;
  error?: string;
  warning?: string;
}

/**
 * Validates a hexadecimal string
 * @param str The string to validate
 * @param expectedBytes Expected byte length (each byte = 2 hex chars)
 */
export function validateHex(str: string, expectedBytes?: number): ValidationResult {
  if (!str || str.trim() === "") {
    return { valid: false, error: "Value is required" };
  }

  const trimmed = str.trim().toLowerCase();

  // Remove optional 0x prefix
  const hex = trimmed.startsWith("0x") ? trimmed.slice(2) : trimmed;

  // Check for valid hex characters
  if (!/^[0-9a-f]+$/.test(hex)) {
    return { valid: false, error: "Invalid hexadecimal format" };
  }

  // Check length if specified
  if (expectedBytes !== undefined) {
    const expectedChars = expectedBytes * 2;
    if (hex.length !== expectedChars) {
      return {
        valid: false,
        error: `Expected ${expectedBytes} bytes (${expectedChars} hex characters), got ${hex.length / 2} bytes`,
      };
    }
  }

  return { valid: true };
}

/**
 * Validates a repository URL
 * @param url The URL to validate
 */
export function validateRepoUrl(url: string): ValidationResult {
  if (!url || url.trim() === "") {
    return { valid: false, error: "Repository URL is required" };
  }

  const trimmed = url.trim();

  // Check for valid URL format
  try {
    // Handle rest: prefix (used by restic)
    const urlToCheck = trimmed.startsWith("rest:")
      ? trimmed.slice(5)
      : trimmed;

    const parsed = new URL(urlToCheck);

    // Warn about HTTP (not HTTPS)
    if (parsed.protocol === "http:") {
      // Allow localhost and private networks without warning
      const host = parsed.hostname;
      const isPrivate =
        host === "localhost" ||
        host === "127.0.0.1" ||
        host.startsWith("192.168.") ||
        host.startsWith("10.") ||
        /^172\.(1[6-9]|2[0-9]|3[0-1])\./.test(host);

      if (!isPrivate) {
        return {
          valid: true,
          warning:
            "Using HTTP is insecure. Consider using HTTPS for production.",
        };
      }
    }

    return { valid: true };
  } catch {
    return { valid: false, error: "Invalid URL format" };
  }
}

/**
 * Validates a file/directory path
 * @param path The path to validate
 */
export function validatePath(path: string): ValidationResult {
  if (!path || path.trim() === "") {
    return { valid: false, error: "Path is required" };
  }

  const trimmed = path.trim();

  // Reject path traversal attempts
  if (trimmed.includes("..")) {
    return { valid: false, error: "Path traversal (..) not allowed" };
  }

  // Reject home directory expansion (could be security risk in some contexts)
  if (trimmed.startsWith("~") && trimmed.length > 1 && trimmed[1] !== "/") {
    return {
      valid: false,
      error: "Home directory expansion for other users not allowed",
    };
  }

  // Reject shell metacharacters that could be dangerous
  const shellMetachars = /[;&|`$(){}[\]<>!*?]/;
  if (shellMetachars.test(trimmed)) {
    return { valid: false, error: "Path contains invalid characters" };
  }

  // Reject null bytes
  if (trimmed.includes("\0")) {
    return { valid: false, error: "Path contains invalid characters" };
  }

  return { valid: true };
}

/**
 * Validates a name (e.g., vault name, key holder name)
 * @param name The name to validate
 * @param maxLength Maximum allowed length (default 64)
 */
export function validateName(name: string, maxLength = 64): ValidationResult {
  if (!name || name.trim() === "") {
    return { valid: false, error: "Name is required" };
  }

  const trimmed = name.trim();

  // Check length
  if (trimmed.length > maxLength) {
    return {
      valid: false,
      error: `Name must be ${maxLength} characters or less`,
    };
  }

  // Allow alphanumeric, spaces, hyphens, underscores, and dots
  if (!/^[a-zA-Z0-9\s\-_.]+$/.test(trimmed)) {
    return {
      valid: false,
      error:
        "Name can only contain letters, numbers, spaces, hyphens, underscores, and dots",
    };
  }

  // Must start with alphanumeric
  if (!/^[a-zA-Z0-9]/.test(trimmed)) {
    return { valid: false, error: "Name must start with a letter or number" };
  }

  return { valid: true };
}

/**
 * Validates a cron schedule expression
 * @param schedule The schedule expression to validate
 */
export function validateSchedule(schedule: string): ValidationResult {
  if (!schedule || schedule.trim() === "") {
    return { valid: false, error: "Schedule is required" };
  }

  const trimmed = schedule.trim().toLowerCase();

  // Allow common aliases
  const aliases = [
    "hourly",
    "daily",
    "weekly",
    "monthly",
    "@hourly",
    "@daily",
    "@weekly",
    "@monthly",
  ];
  if (aliases.includes(trimmed)) {
    return { valid: true };
  }

  // Basic cron format validation (5 or 6 fields)
  const parts = trimmed.split(/\s+/);
  if (parts.length < 5 || parts.length > 6) {
    return {
      valid: false,
      error: "Invalid schedule format. Use cron syntax or aliases like 'daily', 'weekly'",
    };
  }

  // Validate each cron field (basic check)
  const cronFieldPattern = /^(\*|[0-9,\-*/]+)$/;
  for (const part of parts) {
    if (!cronFieldPattern.test(part)) {
      return {
        valid: false,
        error: `Invalid cron field: ${part}`,
      };
    }
  }

  return { valid: true };
}

/**
 * Validates an API key format
 * @param key The API key to validate
 */
export function validateApiKey(key: string): ValidationResult {
  if (!key || key.trim() === "") {
    return { valid: false, error: "API key is required" };
  }

  const trimmed = key.trim();

  // Minimum length for security
  if (trimmed.length < 16) {
    return {
      valid: false,
      error: "API key must be at least 16 characters",
    };
  }

  // Check for reasonable characters
  if (!/^[a-zA-Z0-9\-_]+$/.test(trimmed)) {
    return {
      valid: false,
      error: "API key contains invalid characters",
    };
  }

  return { valid: true };
}

/**
 * Validates a threshold value (for m-of-n schemes)
 */
export function validateThreshold(
  threshold: number,
  total: number
): ValidationResult {
  if (!Number.isInteger(threshold) || threshold < 1) {
    return { valid: false, error: "Threshold must be a positive integer" };
  }

  if (!Number.isInteger(total) || total < 1) {
    return { valid: false, error: "Total must be a positive integer" };
  }

  if (threshold > total) {
    return {
      valid: false,
      error: "Threshold cannot be greater than total",
    };
  }

  return { valid: true };
}

/**
 * Sanitizes a string by removing potentially dangerous characters
 * @param str The string to sanitize
 */
export function sanitizeString(str: string): string {
  if (!str) return "";

  // Remove null bytes, control characters, and common injection patterns
  return str
    .replace(/\0/g, "")
    .replace(/[\x00-\x08\x0B\x0C\x0E-\x1F]/g, "")
    .trim();
}
