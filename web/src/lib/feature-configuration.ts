export type OptionalFeatures = Readonly<{ systems_lab: boolean; live_investigation: boolean }>;

/** Server configuration only. This function never accepts browser preferences. */
export function readOptionalFeatures(environment: Readonly<Record<string, string | undefined>>): OptionalFeatures {
 const flag=(name:string):boolean=>{const value=environment[name]?.trim()??"";if(value===""||value==="false")return false;if(value==="true")return true;throw new Error("Optional feature settings must be true or false.");};
 const systemsLab=flag("LEDGERSYNC_STUDY_ENABLED"),live=flag("LEDGERSYNC_LIVE_INVESTIGATION_ENABLED");
 const deployment=(environment.LEDGERSYNC_DEPLOYMENT_ENV??environment.NODE_ENV??"development").trim().toLowerCase();
 if(systemsLab&&live)throw new Error("Enable one qualified optional feature set at a time.");
 if((systemsLab||live)&&deployment!=="development")throw new Error("Optional study features require the explicit local development environment.");
 return {systems_lab:systemsLab,live_investigation:live};
}
