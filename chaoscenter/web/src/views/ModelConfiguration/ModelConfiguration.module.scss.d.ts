declare namespace ModelConfigurationModuleScssNamespace {
  export interface IModelConfigurationModuleScss {
    active: string;
    defaultTag: string;
    description: string;
    dialog: string;
    formInput: string;
    needsKey: string;
    platform: string;
    smallDialog: string;
    status: string;
    testFailed: string;
    testPassed: string;
    testResult: string;
    working: string;
  }
}

declare const ModelConfigurationModuleScssModule: ModelConfigurationModuleScssNamespace.IModelConfigurationModuleScss;

export = ModelConfigurationModuleScssModule;
