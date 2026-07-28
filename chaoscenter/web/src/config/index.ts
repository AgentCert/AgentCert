import type { APIConfig } from '@api/LitmusAPIProvider';

let auth: string;
let chaosManager: string;

if (__DEV__) {
  auth = `/auth`;
  chaosManager = `/api`;
} else {
  auth = `${window.location.origin}/auth`;
  chaosManager = `${window.location.origin}/api`;
}

// Langfuse port: 4002 (Docker Compose, LANGFUSE_PORT=4002 in .env) or 32400 (k8s NodePort).
// Port 2001 identifies the Docker Compose setup (web served on host port 2001).
const langfusePort = window.location.port === '2001' ? 4002 : 32400;
export function getLangfuseTraceURL(notifyID: string): string {
  return `${window.location.protocol}//${window.location.hostname}:${langfusePort}/project/agentcert/traces/${notifyID}`;
}

const APIEndpoints: APIConfig = {
  gqlEndpoints: {
    chaosManagerUri: `${chaosManager}/query`
  },
  restEndpoints: {
    authUri: auth,
    chaosManagerUri: chaosManager
  }
};
export default APIEndpoints;
