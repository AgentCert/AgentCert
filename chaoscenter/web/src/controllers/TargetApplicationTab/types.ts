export interface TargetApplicationData {
  appns?: string | undefined;
  appkind?: string;
  applabel?: string;
}

export interface NamespaceData {
  namespace: string[];
}

export interface AppLabelOption {
  name: string;
  label: string;
}

export interface AppInfoData {
  appLabels: AppLabelOption[];
}
