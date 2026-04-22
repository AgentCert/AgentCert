/**
 * Configuration for bucketing-extraction task polling.
 * Adjust these values as needed — they are not hardcoded in the polling logic.
 */

/** Interval between successive poll calls (in milliseconds). */
export const BUCKETING_POLL_INTERVAL_MS = 5 * 60 * 1000; // 5 minutes

/** Maximum time to keep polling before giving up (in milliseconds). */
export const BUCKETING_POLL_TIMEOUT_MS = 15 * 60 * 1000; // 15 minutes
