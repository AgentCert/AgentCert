import { ConfigField, ConfigFieldType } from '@api/entities';
import type { Parameter } from '@models';

// The chart settings chosen in the builder travel to install-agent /
// install-app as one install-step arg: `-values-json=<JSON values document>`.
// The installer hands it to Helm as a values file ahead of the platform's own
// --set args, and the server validates it against the chart's declared
// settings on every save and run (graphql/server/pkg/chartconfig).
const VALUES_JSON_FLAG = 'values-json';

export type ConfigValue = string | number | boolean;

// Settings keyed by their dotted Helm values path, e.g. agent.config.SCAN_INTERVAL.
export type ConfigValues = Record<string, ConfigValue>;

// Where an install step's current settings are read from.
export interface InstallStepSettingsSource {
  args: string[];
  parameters: Parameter[];
}

// An install step's chart, as read back from the manifest.
export interface InstallStepSelection {
  folder: string;
  namespace: string;
  source?: InstallStepSettingsSource;
}

// What the install-step drawer hands back: the chosen chart and its settings.
export interface InstallStepChoice {
  folder: string;
  namespace: string;
  settings: {
    values: ConfigValues;
    // Every key the chart declares, so a template's --set for one is replaced.
    declaredKeys: string[];
  };
}

const WORKFLOW_PARAMETER_REF = /^\{\{\s*workflow\.parameters\.([A-Za-z0-9_-]+)\s*\}\}$/;

// Replaces a whole-value {{workflow.parameters.X}} reference with the
// parameter's value. ChaosHub templates write e.g.
// -folder={{workflow.parameters.agentFolder}}.
export function resolveWorkflowParameter(value: string, parameters: Parameter[] = []): string {
  const match = WORKFLOW_PARAMETER_REF.exec(value.trim());
  if (!match) return value;
  return parameters.find(parameter => parameter.name === match[1])?.value ?? value;
}

export function readInstallStepArg(args: string[], flag: string): string | undefined {
  let value: string | undefined;
  for (let i = 0; i < args.length; i++) {
    for (const prefix of [`-${flag}=`, `--${flag}=`]) {
      if (args[i].startsWith(prefix)) value = args[i].slice(prefix.length);
    }
    if ((args[i] === `-${flag}` || args[i] === `--${flag}`) && i + 1 < args.length) value = args[++i];
  }
  return value;
}

export function writeInstallStepArg(args: string[], flag: string, value: string): string[] {
  const kept: string[] = [];
  for (let i = 0; i < args.length; i++) {
    if (args[i].startsWith(`-${flag}=`) || args[i].startsWith(`--${flag}=`)) continue;
    if (args[i] === `-${flag}` || args[i] === `--${flag}`) {
      if (i + 1 < args.length) i++;
      continue;
    }
    kept.push(args[i]);
  }
  return [...kept, `-${flag}=${value}`];
}

// The settings document an install step carries, decoded, or {} without one.
export function readValuesJSON(args: string[] = []): ConfigValues {
  const document = readInstallStepArg(args, VALUES_JSON_FLAG);
  return document ? decodeValuesJSON(document) : {};
}

function isValuesJSONArg(arg: string): boolean {
  return arg.startsWith(`-${VALUES_JSON_FLAG}=`) || arg.startsWith(`--${VALUES_JSON_FLAG}=`);
}

function flatten(node: Record<string, unknown>, prefix: string, out: ConfigValues): void {
  Object.entries(node).forEach(([name, value]) => {
    const key = prefix ? `${prefix}.${name}` : name;
    if (value !== null && typeof value === 'object' && !Array.isArray(value)) {
      flatten(value as Record<string, unknown>, key, out);
    } else if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') {
      out[key] = value;
    }
  });
}

// Decodes a -values-json document into settings keyed by path. A document that
// is not a JSON object sets nothing; the server rejects it on save.
export function decodeValuesJSON(document: string): ConfigValues {
  try {
    const parsed: unknown = JSON.parse(document);
    if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) return {};
    const values: ConfigValues = {};
    flatten(parsed as Record<string, unknown>, '', values);
    return values;
  } catch {
    return {};
  }
}

// Encodes settings as the nested JSON values document Helm expects. Keys are
// sorted so the same settings always produce the same arg.
export function encodeValuesJSON(values: ConfigValues): string {
  const root: Record<string, unknown> = Object.create(null);
  Object.keys(values)
    .sort()
    .forEach(key => {
      const segments = key.split('.');
      let node = root;
      segments.slice(0, -1).forEach(segment => {
        if (typeof node[segment] !== 'object' || node[segment] === null) node[segment] = Object.create(null);
        node = node[segment] as Record<string, unknown>;
      });
      node[segments[segments.length - 1]] = values[key];
    });
  return JSON.stringify(root);
}

// The --set values a ChaosHub template writes on the step, by key, with
// workflow parameter references resolved. Values that still hold an Argo
// expression after resolution (e.g. {{workflow.uid}}) are run-time values,
// not settings, and are left out.
function templateSetValues(source: InstallStepSettingsSource): Record<string, string> {
  const values: Record<string, string> = {};
  const record = (assignment: string): void => {
    const separator = assignment.indexOf('=');
    if (separator <= 0) return;
    const value = resolveWorkflowParameter(assignment.slice(separator + 1), source.parameters);
    if (!value.includes('{{')) values[assignment.slice(0, separator)] = value;
  };
  source.args.forEach((arg, i) => {
    const combined = /^--?set(?:-string)?=(.*)$/.exec(arg);
    if (combined) record(combined[1]);
    else if ((arg === '-set' || arg === '--set' || arg === '--set-string') && i + 1 < source.args.length) {
      record(source.args[i + 1]);
    }
  });
  return values;
}

// The settings an install step currently sets for the chart's declared fields:
// its -values-json first, then a ChaosHub template's own --set args. A field
// the step does not set is left out, so the form shows the chart default.
//
// A model is only ever read from -values-json: the server has always replaced
// a template's --set MODEL_ALIAS with its own, so adopting that value would
// change which model a template runs with.
export function readInstallStepSettings(fields: ConfigField[], source?: InstallStepSettingsSource): ConfigValues {
  if (!source) return {};
  const configured = readValuesJSON(source.args);
  const fromTemplate = templateSetValues(source);

  const values: ConfigValues = {};
  fields.forEach(field => {
    if (Object.prototype.hasOwnProperty.call(configured, field.key)) {
      values[field.key] = configured[field.key];
    } else if (field.type !== ConfigFieldType.MODEL && Object.prototype.hasOwnProperty.call(fromTemplate, field.key)) {
      values[field.key] = fromTemplate[field.key];
    }
  });
  return values;
}

function setArgKey(assignment: string): string {
  const separator = assignment.indexOf('=');
  return separator < 0 ? assignment : assignment.slice(0, separator);
}

// Returns the step's args with its settings replaced by `values`: the old
// -values-json and any template --set for a declared key are dropped (a --set
// would otherwise override the user's setting), and the new document is
// appended. An empty document records an explicit reset to defaults, so saving
// also clears a model override left by an earlier run.
export function writeInstallStepSettings(args: string[], values: ConfigValues, declaredKeys: string[]): string[] {
  const declared = new Set(declaredKeys);
  const kept: string[] = [];
  for (let i = 0; i < args.length; i++) {
    const arg = args[i];
    if (isValuesJSONArg(arg)) continue;
    if ((arg === `-${VALUES_JSON_FLAG}` || arg === `--${VALUES_JSON_FLAG}`) && i + 1 < args.length) {
      i++;
      continue;
    }
    const combined = /^--?set(?:-string|-json)?=(.*)$/.exec(arg);
    if (combined && declared.has(setArgKey(combined[1]))) continue;
    if (
      (arg === '-set' || arg === '--set' || arg === '--set-string' || arg === '--set-json') &&
      i + 1 < args.length &&
      declared.has(setArgKey(args[i + 1]))
    ) {
      i++;
      continue;
    }
    kept.push(arg);
  }
  kept.push(`-${VALUES_JSON_FLAG}=${encodeValuesJSON(values)}`);
  return kept;
}
