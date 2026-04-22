/**
 * Configuration for bucketing-extraction task polling.
 * Adjust these values as needed — they are not hardcoded in the polling logic.
 */

/** Interval between successive poll calls (in milliseconds). */
export const BUCKETING_POLL_INTERVAL_MS = 5 * 60 * 1000; // 5 minutes

/** Maximum time to keep polling before giving up (in milliseconds). */
export const BUCKETING_POLL_TIMEOUT_MS = 15 * 60 * 1000; // 15 minutes

/**
 * TODO: Remove this once the actual /api/v1/tasks/{task_id} API is implemented.
 * When true, the poller returns a mock COMPLETED response for stub task IDs
 * instead of making a real network call.
 */
export const BUCKETING_USE_STUB_POLL_RESPONSE = true;

/**
 * TODO: Remove this once the actual API is implemented.
 * Delay (in ms) before the stub poll returns COMPLETED.
 * Set to 0 for instant, or e.g. 10000 (10s) to see the blinking "..." in action.
 */
export const BUCKETING_STUB_DELAY_MS = 10_000; // 10 seconds
