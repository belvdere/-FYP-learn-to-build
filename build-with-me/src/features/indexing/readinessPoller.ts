import { languageLSPPlugins, getRequiredPluginsForWorkspace } from './languageLSPPlugins';

export interface LSStatus {
  name: string;
  ready: boolean;
  reason?: string;
}

export function startReadinessPoller(
  onUpdate: (statuses: LSStatus[]) => void,
  intervalMs = 3000
): () => void {
  let stopped = false;
  let timer: NodeJS.Timeout | undefined;

  const tick = async () => {
    if (stopped) {
      return;
    }

    const required = await getRequiredPluginsForWorkspace();
    const plugins = required.length > 0 ? required : languageLSPPlugins;

    const statuses: LSStatus[] = [];
    for (const plugin of plugins) {
      try {
        const readiness = await plugin.checkReady();
        statuses.push({ name: plugin.name, ready: readiness.ready, reason: readiness.reason });
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        statuses.push({ name: plugin.name, ready: false, reason: msg });
      }
    }

    onUpdate(statuses);

    const allReady = statuses.every((s) => s.ready);
    if (!allReady) {
      timer = setTimeout(() => void tick(), intervalMs);
    }
  };

  void tick();

  return () => {
    stopped = true;
    if (timer) {
      clearTimeout(timer);
      timer = undefined;
    }
  };
}
