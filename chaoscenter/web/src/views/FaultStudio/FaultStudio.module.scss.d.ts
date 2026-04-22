declare namespace FaultStudioModuleScssNamespace {
  export interface IFaultStudioModuleScss {
    activeStatus: string;
    enabledStatus: string;
    faultCard: string;
    faultToggle: string;
    faultsContainer: string;
    infoCard: string;
  }
}

declare const FaultStudioModuleScssModule: FaultStudioModuleScssNamespace.IFaultStudioModuleScss;

export = FaultStudioModuleScssModule;
