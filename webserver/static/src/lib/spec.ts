export interface ObjectMeta {
    name: string;
    generateName: string;
    namespace: string;
    creationTimestamp: string;
}

export interface WaybillSpec {
    autoApply?: boolean;
    delegateServiceAccountSecretRef: string;
    dryRun: boolean;
    gitSSHSecretRef?: {
      name: string;
      namespace: string;
    };
    prune?: boolean;
    pruneClusterResources?: boolean;
    pruneBlacklist: string[];
    repositoryPath: string;
    runInterval: string;
    runTimeout: number;
    serverSideApply: boolean;
    strongboxKeyringSecretRef?: {
      name: string;
      namespace: string;
    };
}

export interface WaybillStatus {
    lastRun?: {
        command: string;
        commit: string;
        errorMessage?: string;
        finished: string;
        output: string;
        started: string;
        success: boolean;
        type: string;
    }
}

export interface Waybill {
    kind: string;
    apiVersion: string;
    metadata: ObjectMeta;
    spec: WaybillSpec;
    status: WaybillStatus;
}
