import {
  StartCompare, CancelCompare, ComparePage, CompareDetail,
  PreviewSync, CancelSync, ApplySync, StartWatch, PollWatch, StopWatch,
} from '../../wailsjs/go/main/App';

export const workbench = {
  startCompare: StartCompare,
  cancelCompare: CancelCompare,
  comparePage: ComparePage,
  compareDetail: CompareDetail,
  previewSync: PreviewSync,
  cancelSync: CancelSync,
  applySync: ApplySync,
  startWatch: StartWatch,
  pollWatch: PollWatch,
  stopWatch: StopWatch,
};

export function sessionLabel(session: { name: string; environment?: string }): string {
  return session.environment ? `${session.name} · ${session.environment}` : session.name;
}
