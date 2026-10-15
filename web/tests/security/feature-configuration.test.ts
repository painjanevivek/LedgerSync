import assert from "node:assert/strict";
import test from "node:test";
import { readOptionalFeatures } from "../../src/lib/feature-configuration";

test("the server feature matrix keeps core default and rejects unsupported settings",()=>{
 assert.deepEqual(readOptionalFeatures({NODE_ENV:"production"}),{systems_lab:false,live_investigation:false});
 assert.deepEqual(readOptionalFeatures({LEDGERSYNC_DEPLOYMENT_ENV:"development",LEDGERSYNC_STUDY_ENABLED:"true"}),{systems_lab:true,live_investigation:false});
 assert.deepEqual(readOptionalFeatures({LEDGERSYNC_DEPLOYMENT_ENV:"development",LEDGERSYNC_LIVE_INVESTIGATION_ENABLED:"true"}),{systems_lab:false,live_investigation:true});
 for(const environment of [{LEDGERSYNC_STUDY_ENABLED:"1"},{LEDGERSYNC_LIVE_INVESTIGATION_ENABLED:"yes"},{LEDGERSYNC_STUDY_ENABLED:"true",NODE_ENV:"production"},{LEDGERSYNC_LIVE_INVESTIGATION_ENABLED:"true",NODE_ENV:"production"},{LEDGERSYNC_STUDY_ENABLED:"true",LEDGERSYNC_LIVE_INVESTIGATION_ENABLED:"true"}])assert.throws(()=>readOptionalFeatures(environment));
});
