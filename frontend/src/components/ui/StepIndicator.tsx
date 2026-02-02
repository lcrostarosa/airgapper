import React from 'react';

interface Step {
  /** Label for the step */
  label: string;
  /** Optional description */
  description?: string;
}

interface StepIndicatorProps {
  /** Array of step definitions */
  steps: Step[];
  /** Current step index (0-based) */
  currentStep: number;
  /** Whether to show step labels */
  showLabels?: boolean;
  /** Additional CSS classes */
  className?: string;
}

/**
 * A step indicator for multi-step flows.
 *
 * @example
 * const steps = [
 *   { label: 'Configure', description: 'Set up basics' },
 *   { label: 'Review' },
 *   { label: 'Complete' },
 * ];
 * <StepIndicator steps={steps} currentStep={1} />
 */
export function StepIndicator({
  steps,
  currentStep,
  showLabels = true,
  className = '',
}: StepIndicatorProps) {
  return (
    <div className={`flex items-center justify-between ${className}`}>
      {steps.map((step, index) => {
        const isCompleted = index < currentStep;
        const isCurrent = index === currentStep;
        const isLast = index === steps.length - 1;

        return (
          <React.Fragment key={index}>
            {/* Step circle */}
            <div className="flex flex-col items-center">
              <div
                className={`w-8 h-8 rounded-full flex items-center justify-center text-sm font-medium transition-colors ${
                  isCompleted
                    ? 'bg-green-600 text-white'
                    : isCurrent
                    ? 'bg-blue-600 text-white'
                    : 'bg-gray-700 text-gray-400'
                }`}
              >
                {isCompleted ? '✓' : index + 1}
              </div>
              {showLabels && (
                <div className="mt-2 text-center">
                  <div
                    className={`text-xs font-medium ${
                      isCurrent ? 'text-white' : 'text-gray-400'
                    }`}
                  >
                    {step.label}
                  </div>
                  {step.description && (
                    <div className="text-xs text-gray-500 mt-0.5">
                      {step.description}
                    </div>
                  )}
                </div>
              )}
            </div>

            {/* Connector line */}
            {!isLast && (
              <div
                className={`flex-1 h-0.5 mx-2 ${
                  index < currentStep ? 'bg-green-600' : 'bg-gray-700'
                }`}
              />
            )}
          </React.Fragment>
        );
      })}
    </div>
  );
}
