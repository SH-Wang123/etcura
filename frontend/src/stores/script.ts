import { get, writable } from 'svelte/store';
import { api, localizeError } from '../lib/api';
import type { ScriptConfig, ScriptInfo, ScriptOutput } from '../lib/types';
import { toast } from '../lib/toast';
import { tr } from '../lib/i18n';

/** Empty editor state (both variable names optional). */
export function emptyScriptConfig(): ScriptConfig {
  return { language: 'python', inputVar: 'payload', outputVar: 'result', body: '' };
}

/** The configured snippet (null when none). */
export const scriptInfo = writable<ScriptInfo | null>(null);

/** Output of the last run for the currently selected key. */
export const scriptOutput = writable<ScriptOutput | null>(null);
export const scriptRunning = writable(false);
export const scriptError = writable<string | null>(null);

let runGen = 0;

function configured(): boolean {
  return get(scriptInfo)?.configured === true;
}

export async function loadScript(): Promise<void> {
  try {
    const info = await api.getScript();
    scriptInfo.set(info.configured ? info : null);
  } catch {
    /* best effort (no runtime in a plain browser) */
  }
}

export async function saveScript(cfg: ScriptConfig): Promise<boolean> {
  try {
    const info = await api.setScript(cfg);
    scriptInfo.set(info);
    scriptOutput.set(null);
    scriptError.set(null);
    if (info.warning) {
      toast(info.warning, 'error');
    } else {
      toast(tr('script.setToast', { lang: info.language }), 'success');
    }
    return true;
  } catch (e) {
    const err = localizeError(e);
    toast(err.message, 'error', err.detail);
    return false;
  }
}

export async function clearScript(): Promise<void> {
  try {
    await api.clearScript();
    scriptInfo.set(null);
    scriptOutput.set(null);
    scriptError.set(null);
    toast(tr('script.clearedToast'), 'success');
  } catch (e) {
    toast(localizeError(e).message, 'error');
  }
}

/**
 * Runs the snippet over the given value. Superseding runs are tracked with a
 * generation counter so a slow run cannot overwrite a newer result.
 */
export async function runScriptOver(valueBase64: string): Promise<void> {
  if (!configured()) return;
  const gen = ++runGen;
  scriptRunning.set(true);
  scriptError.set(null);
  try {
    const out = await api.runScript(valueBase64);
    if (gen !== runGen) return;
    scriptOutput.set(out);
  } catch (e) {
    if (gen !== runGen) return;
    const err = localizeError(e);
    scriptOutput.set(null);
    scriptError.set(err.message);
  } finally {
    if (gen === runGen) scriptRunning.set(false);
  }
}

/** Re-runs the snippet for the currently selected value. */
export async function rerunScript(valueBase64: string): Promise<void> {
  await runScriptOver(valueBase64);
}

/** Drops any result (e.g. when the selection changes). */
export function resetScriptOutput(): void {
  ++runGen;
  scriptOutput.set(null);
  scriptError.set(null);
  scriptRunning.set(false);
}
